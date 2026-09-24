// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// router assigns the .fx extension to fixture and lists fixture and other.
type router struct{}

func (router) LanguageOf(p source.Path) (source.Language, bool) {
	if path.Ext(string(p)) == ".fx" {
		return fixture, true
	}
	return "", false
}

func (router) Languages() []source.Language { return []source.Language{fixture, other} }

// outline calls the Outliner port of e.
func outline(e engine.Engine) (engine.Result[sema.Symbol], error) {
	return e.(engine.Outliner).Outline(context.Background(), engine.Request{})
}

// declining returns ErrDecline wrapped with a reason.
func declining(reason string) error {
	return fmt.Errorf("%w: %s", engine.ErrDecline, reason)
}

func TestAsk(t *testing.T) {
	t.Parallel()

	t.Run("Languages", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give engine.Request
			want []source.Language
		}{
			{
				name: "returns the request language when one is set",
				give: engine.Request{Scope: "a.fx", Language: other},
				want: []source.Language{other},
			},
			{
				name: "returns the language that claims the extension",
				give: engine.Request{Scope: "src/a.fx"},
				want: []source.Language{fixture},
			},
			{
				name: "returns no language for an unclaimed extension",
				give: engine.Request{Scope: "README.md"},
				want: nil,
			},
			{
				name: "returns every language for a directory",
				give: engine.Request{Scope: "src"},
				want: []source.Language{fixture, other},
			},
			{
				name: "returns every language for the root",
				give: engine.Request{Scope: engine.Root},
				want: []source.Language{fixture, other},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, engine.Languages(router{}, tt.give), tt.want, "languages")
			})
		}
	})

	t.Run("Ask", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the answer of the highest-fidelity engine", func(t *testing.T) {
			t.Parallel()
			c := catalog(t,
				fake{name: "parser", fidelity: trust.Syntactic},
				fake{name: "checker", fidelity: trust.Resolved})
			got, ok, _, err := engine.Ask(t.Context(), c, fixture, engine.RoleOutline, trust.None, outline)
			assert.NoError(t, err, "Ask")
			assert.True(t, ok, "answered")
			assert.Equal(t, got.Provenance.Engine, "checker", "engine")
		})

		t.Run("publishes the declared tier of the engine", func(t *testing.T) {
			t.Parallel()
			c := catalog(t, fake{name: "parser", fidelity: trust.Syntactic})
			got, _, _, err := engine.Ask(t.Context(), c, fixture, engine.RoleOutline, trust.Resolved, outline)
			assert.NoError(t, err, "Ask")
			assert.Equal(t, got.Provenance.Fidelity, trust.Syntactic, "fidelity")
		})

		t.Run("tries the next engine after a decline", func(t *testing.T) {
			t.Parallel()
			c := catalog(t,
				fake{name: "checker", fidelity: trust.Resolved, err: declining("not loaded")},
				fake{name: "parser", fidelity: trust.Syntactic})
			got, ok, _, err := engine.Ask(t.Context(), c, fixture, engine.RoleOutline, trust.None, outline)
			assert.NoError(t, err, "Ask")
			assert.True(t, ok, "answered")
			assert.Equal(t, got.Provenance.Engine, "parser", "engine")
		})

		t.Run("returns the reason of every engine that declined", func(t *testing.T) {
			t.Parallel()
			c := catalog(t,
				fake{name: "checker", fidelity: trust.Resolved, err: declining("not loaded")},
				fake{name: "parser", fidelity: trust.Syntactic, err: declining("no grammar")})
			_, ok, declined, err := engine.Ask(t.Context(), c, fixture, engine.RoleOutline, trust.None, outline)
			assert.NoError(t, err, "Ask")
			assert.False(t, ok, "answered")
			assert.Equal(t, declined, engine.Declined{"checker: not loaded", "parser: no grammar"}, "Declined")
		})

		t.Run("names the engine once when the reason starts with the name", func(t *testing.T) {
			t.Parallel()
			c := catalog(t, fake{name: "checker", fidelity: trust.Resolved, err: declining("checker: not loaded")})
			_, _, declined, err := engine.Ask(t.Context(), c, fixture, engine.RoleOutline, trust.None, outline)
			assert.NoError(t, err, "Ask")
			assert.Equal(t, strings.Count(declined.Reason(), "checker"), 1, "engine names")
		})

		t.Run("names the engine when the reason contains the name inside a word", func(t *testing.T) {
			t.Parallel()
			c := catalog(t, fake{name: "ot", fidelity: trust.Resolved, err: declining("not loaded")})
			_, _, declined, err := engine.Ask(t.Context(), c, fixture, engine.RoleOutline, trust.None, outline)
			assert.NoError(t, err, "Ask")
			assert.Equal(t, declined, engine.Declined{"ot: not loaded"}, "Declined")
		})

		t.Run("returns no reasons when an engine returns an answer", func(t *testing.T) {
			t.Parallel()
			c := catalog(t, fake{name: "parser", fidelity: trust.Syntactic})
			_, _, declined, err := engine.Ask(t.Context(), c, fixture, engine.RoleOutline, trust.None, outline)
			assert.NoError(t, err, "Ask")
			assert.Empty(t, declined, "Declined")
		})

		t.Run("stops at the first engine that fails", func(t *testing.T) {
			t.Parallel()
			broken := errors.New("checker: exited")
			calls := 0
			c := catalog(t,
				fake{name: "checker", fidelity: trust.Resolved, err: broken},
				fake{name: "parser", fidelity: trust.Syntactic, calls: &calls})
			_, _, _, err := engine.Ask(t.Context(), c, fixture, engine.RoleOutline, trust.None, outline)
			assert.ErrorIs(t, err, broken, "Ask")
			assert.Equal(t, calls, 0, "parser calls")
		})

		t.Run("returns false when no engine serves the role", func(t *testing.T) {
			t.Parallel()
			_, ok, declined, err := engine.Ask(t.Context(), catalog(t), fixture, engine.RoleOutline, trust.None,
				outline)
			assert.NoError(t, err, "Ask")
			assert.False(t, ok, "answered")
			assert.Empty(t, declined, "Declined")
		})
	})

	t.Run("AskEach", func(t *testing.T) {
		t.Parallel()

		directory := engine.Request{Scope: "src"}

		t.Run("returns the answer of every language", func(t *testing.T) {
			t.Parallel()
			c := catalog(t,
				fake{name: "fixture", fidelity: trust.Syntactic},
				fake{name: "other", language: other, fidelity: trust.Syntactic})
			got, _, err := engine.AskEach(t.Context(), c, router{}, directory, engine.RoleOutline, outline)
			assert.NoError(t, err, "AskEach")
			assert.Length(t, got, 2, "answers")
		})

		t.Run("returns the other answers when one language fails", func(t *testing.T) {
			t.Parallel()
			c := catalog(t,
				fake{name: "fixture", fidelity: trust.Syntactic, err: errors.New("broken")},
				fake{name: "other", language: other, fidelity: trust.Syntactic})
			got, _, err := engine.AskEach(t.Context(), c, router{}, directory, engine.RoleOutline, outline)
			assert.NoError(t, err, "AskEach")
			assert.Length(t, got, 1, "answers")
			assert.Equal(t, got[0].Provenance.Engine, "other", "engine")
		})

		t.Run("records the failed language in Declined", func(t *testing.T) {
			t.Parallel()
			c := catalog(t,
				fake{name: "fixture", fidelity: trust.Syntactic, err: errors.New("broken")},
				fake{name: "other", language: other, fidelity: trust.Syntactic})
			_, declined, err := engine.AskEach(t.Context(), c, router{}, directory, engine.RoleOutline, outline)
			assert.NoError(t, err, "AskEach")
			assert.Equal(t, declined, engine.Declined{"fixture: broken"}, "Declined")
		})

		t.Run("returns the first error when every language fails", func(t *testing.T) {
			t.Parallel()
			first, second := errors.New("first broken"), errors.New("second broken")
			c := catalog(t,
				fake{name: "fixture", fidelity: trust.Syntactic, err: first},
				fake{name: "other", language: other, fidelity: trust.Syntactic, err: second})
			_, _, err := engine.AskEach(t.Context(), c, router{}, directory, engine.RoleOutline, outline)
			assert.ErrorIs(t, err, first, "AskEach")
		})

		t.Run("returns the error of a failed language when the context is done", func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			calls := 0
			c := catalog(t,
				fake{name: "fixture", fidelity: trust.Syntactic, err: context.Canceled},
				fake{name: "other", language: other, fidelity: trust.Syntactic, calls: &calls})
			_, _, err := engine.AskEach(ctx, c, router{}, directory, engine.RoleOutline, outline)
			assert.ErrorIs(t, err, context.Canceled, "AskEach")
			assert.Equal(t, calls, 0, "other calls")
		})
	})

	t.Run("AskAny", func(t *testing.T) {
		t.Parallel()

		directory := engine.Request{Scope: "src"}

		t.Run("returns the answer of the first language with an answer", func(t *testing.T) {
			t.Parallel()
			calls := 0
			c := catalog(t,
				fake{name: "fixture", fidelity: trust.Syntactic},
				fake{name: "other", language: other, fidelity: trust.Syntactic, calls: &calls})
			got, ok, _, err := engine.AskAny(t.Context(), c, router{}, directory, engine.RoleOutline, outline)
			assert.NoError(t, err, "AskAny")
			assert.True(t, ok, "answered")
			assert.Equal(t, got.Provenance.Engine, "fixture", "engine")
			assert.Equal(t, calls, 0, "other calls")
		})

		t.Run("asks the next language after every engine declines", func(t *testing.T) {
			t.Parallel()
			c := catalog(t,
				fake{name: "fixture", fidelity: trust.Syntactic, err: declining("no match")},
				fake{name: "other", language: other, fidelity: trust.Syntactic})
			got, ok, declined, err := engine.AskAny(t.Context(), c, router{}, directory, engine.RoleOutline, outline)
			assert.NoError(t, err, "AskAny")
			assert.True(t, ok, "answered")
			assert.Equal(t, got.Provenance.Engine, "other", "engine")
			assert.Equal(t, declined, engine.Declined{"fixture: no match"}, "Declined")
		})

		t.Run("asks the next language after a skipped answer", func(t *testing.T) {
			t.Parallel()
			c := catalog(t,
				fake{name: "fixture", fidelity: trust.Resolved, skipped: true},
				fake{name: "other", language: other, fidelity: trust.Syntactic})
			got, ok, _, err := engine.AskAny(t.Context(), c, router{}, directory, engine.RoleOutline, outline)
			assert.NoError(t, err, "AskAny")
			assert.True(t, ok, "answered")
			assert.Equal(t, got.Provenance.Engine, "other", "engine")
		})

		t.Run("returns false when every language skips the scope", func(t *testing.T) {
			t.Parallel()
			c := catalog(t,
				fake{name: "fixture", fidelity: trust.Resolved, skipped: true},
				fake{name: "other", language: other, fidelity: trust.Syntactic, skipped: true})
			_, ok, _, err := engine.AskAny(t.Context(), c, router{}, directory, engine.RoleOutline, outline)
			assert.NoError(t, err, "AskAny")
			assert.False(t, ok, "answered")
		})

		t.Run("stops at the first language that fails", func(t *testing.T) {
			t.Parallel()
			broken := errors.New("broken")
			calls := 0
			c := catalog(t,
				fake{name: "fixture", fidelity: trust.Syntactic, err: broken},
				fake{name: "other", language: other, fidelity: trust.Syntactic, calls: &calls})
			_, _, _, err := engine.AskAny(t.Context(), c, router{}, directory, engine.RoleOutline, outline)
			assert.ErrorIs(t, err, broken, "AskAny")
			assert.Equal(t, calls, 0, "other calls")
		})

		t.Run("returns false when no language returns an answer", func(t *testing.T) {
			t.Parallel()
			_, ok, _, err := engine.AskAny(t.Context(), catalog(t), router{}, directory, engine.RoleOutline, outline)
			assert.NoError(t, err, "AskAny")
			assert.False(t, ok, "answered")
		})
	})

	t.Run("Reason", func(t *testing.T) {
		t.Parallel()

		t.Run("joins the reasons with semicolons", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, engine.Declined{"a: one", "b: two"}.Reason(), "a: one; b: two", "Reason")
		})

		t.Run("returns an empty string for no reasons", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, engine.Declined(nil).Reason(), "", "Reason")
		})
	})

	t.Run("Unsupported", func(t *testing.T) {
		t.Parallel()

		t.Run("returns status Unsupported with the reason as a caveat", func(t *testing.T) {
			t.Parallel()
			got := engine.Unsupported[sema.Symbol]("no engine serves fixture")
			assert.Equal(t, got.Status, trust.Unsupported, "status")
			assert.Equal(t, got.Provenance.Caveats, []trust.Caveat{{
				Code: trust.CaveatUnsupported,
				Note: "no engine serves fixture",
			}}, "caveats")
		})
	})
}
