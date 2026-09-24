// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package query_test

import (
	"path"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/service/query"
)

const other = source.Language("other")

// suffixes routes a file by its extension, .fx to fixture and .ot to other, and asks its
// languages about a directory.
type suffixes []source.Language

func (suffixes) LanguageOf(p source.Path) (source.Language, bool) {
	switch path.Ext(string(p)) {
	case ".fx":
		return fixture, true
	case ".ot":
		return other, true
	}
	return "", false
}

func (s suffixes) Languages() []source.Language { return s }

// merged returns the outline of the directory src over engines of the languages fixture
// and other.
func merged(t *testing.T, engines ...engine.Engine) engine.Answer[sema.Symbol] {
	t.Helper()
	got, err := query.New(catalogue(t, engines...), suffixes{fixture, other}).
		Outline(t.Context(), engine.Request{Scope: "src"})
	assert.NoError(t, err, "Outline of src")
	return got
}

func TestMerge(t *testing.T) {
	t.Parallel()

	t.Run("Outline", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declarations of every language of a directory", func(t *testing.T) {
			t.Parallel()
			got := merged(t,
				answering{name: "fx", fidelity: trust.Resolved, found: symbol("A")},
				answering{name: "ot", language: other, fidelity: trust.Resolved, found: symbol("B")},
			)
			assert.Equal(t, got.Status, trust.OK, "the status of the answer")
			assert.Equal(t, names(got.Items), []string{"A", "B"}, "the declarations of the answer")
		})

		t.Run("names every engine that read a file", func(t *testing.T) {
			t.Parallel()
			got := merged(t,
				answering{name: "fx", fidelity: trust.Resolved, found: symbol("A")},
				answering{name: "ot", language: other, fidelity: trust.Resolved, found: symbol("B")},
			)
			assert.Equal(t, got.Provenance.Engine, "fx, ot", "the engines of the answer")
		})

		t.Run("states a caveat without paths once", func(t *testing.T) {
			t.Parallel()
			shared := trust.Caveat{Code: trust.CaveatDynamic, Note: "a parser matched the text of a name"}
			got := merged(t,
				answering{name: "fx", fidelity: trust.Syntactic, caveats: []trust.Caveat{shared}},
				answering{name: "ot", language: other, fidelity: trust.Syntactic, caveats: []trust.Caveat{shared}},
			)
			assert.Equal(t, got.Provenance.Caveats, []trust.Caveat{shared}, "the caveats of the answer")
		})

		t.Run("keeps two caveats that name different files", func(t *testing.T) {
			t.Parallel()
			first := trust.Caveat{Code: trust.CaveatUnread, Note: "too large", Paths: []source.Path{"a.fx"}}
			second := trust.Caveat{Code: trust.CaveatUnread, Note: "too large", Paths: []source.Path{"b.ot"}}
			got := merged(t,
				answering{name: "fx", fidelity: trust.Syntactic, caveats: []trust.Caveat{first}},
				answering{name: "ot", language: other, fidelity: trust.Syntactic, caveats: []trust.Caveat{second}},
			)
			assert.Equal(t, got.Provenance.Caveats, []trust.Caveat{first, second}, "the caveats of the answer")
		})

		t.Run("claims the weakest tier of the answers", func(t *testing.T) {
			t.Parallel()
			got := merged(t,
				answering{name: "fx", fidelity: trust.Resolved, found: symbol("A")},
				answering{name: "ot", language: other, fidelity: trust.Syntactic, found: symbol("B")},
			)
			assert.Equal(t, got.Provenance.Fidelity, trust.Syntactic, "the tier of the answer")
		})

		t.Run("claims the weakest completeness of the answers", func(t *testing.T) {
			t.Parallel()
			got := merged(t,
				answering{name: "fx", fidelity: trust.Resolved, found: symbol("A")},
				answering{name: "ot", language: other, fidelity: trust.Resolved, coverage: trust.ScopePartial},
			)
			assert.Equal(t, got.Provenance.Completeness, trust.ScopePartial, "the completeness of the answer")
		})

		t.Run("returns a partial answer with the reason when a language declines", func(t *testing.T) {
			t.Parallel()
			loading := declining("the index is loading")
			got := merged(t,
				answering{name: "fx", fidelity: trust.Resolved, found: symbol("A")},
				answering{name: "ot", language: other, fidelity: trust.Resolved, err: loading},
			)
			assert.Equal(t, got.Provenance.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.Equal(t, got.Provenance.Caveats, []trust.Caveat{{
				Code: trust.CaveatUnsupported,
				Note: "nothing answered for part of the scope: ot: the index is loading",
			}}, "the caveats of the answer")
		})

		t.Run("returns the answer of the one language with an engine", func(t *testing.T) {
			t.Parallel()
			got := merged(t, answering{name: "fx", fidelity: trust.Syntactic, found: symbol("A")})
			assert.Equal(t, got.Status, trust.OK, "the status of the answer")
			assert.Equal(t, names(got.Items), []string{"A"}, "the declarations of the answer")
		})

		t.Run("returns unsupported when no language of a directory has an engine", func(t *testing.T) {
			t.Parallel()
			got := merged(t)
			assert.Equal(t, got.Status, trust.Unsupported, "the status of the answer")
		})

		t.Run("returns the same answer for the same request", func(t *testing.T) {
			t.Parallel()
			engines := []engine.Engine{
				answering{name: "fx", fidelity: trust.Resolved, found: symbol("A")},
				answering{name: "ot", language: other, fidelity: trust.Resolved, found: symbol("B")},
			}
			assert.Equal(t, merged(t, engines...), merged(t, engines...), "the second answer")
		})

		t.Run("asks only the language that claims a file", func(t *testing.T) {
			t.Parallel()
			c := catalogue(t,
				answering{name: "fx", fidelity: trust.Syntactic, found: symbol("A")},
				answering{name: "ot", language: other, fidelity: trust.Syntactic, found: symbol("B")},
			)
			got, err := query.New(c, suffixes{fixture, other}).Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.NoError(t, err, "Outline of a.fx")
			assert.Equal(t, got.Provenance.Engine, "fx", "the engine of the answer")
		})

		t.Run("leaves a skipped answer out of the evidence", func(t *testing.T) {
			t.Parallel()
			got := merged(t,
				answering{name: "fx", fidelity: trust.Resolved, found: symbol("A")},
				answering{name: "ot", language: other, fidelity: trust.Syntactic, skipped: true},
			)
			assert.Equal(t, got.Provenance.Fidelity, trust.Resolved, "the tier of the answer")
			assert.Equal(t, got.Provenance.Engine, "fx", "the engines of the answer")
			assert.True(t, got.Provenance.SupportsNegativeClaim(), "the negative claim of the answer")
		})

		t.Run("counts an answer that read files without a match", func(t *testing.T) {
			t.Parallel()
			got := merged(t,
				answering{name: "fx", fidelity: trust.Resolved, found: symbol("A")},
				answering{name: "ot", language: other, fidelity: trust.Syntactic},
			)
			assert.Equal(t, got.Provenance.Fidelity, trust.Syntactic, "the tier of the answer")
			assert.False(t, got.Provenance.SupportsNegativeClaim(), "the negative claim of the answer")
		})

		t.Run("claims the weakest tier when every answer is skipped", func(t *testing.T) {
			t.Parallel()
			got := merged(t,
				answering{name: "fx", fidelity: trust.Resolved, skipped: true},
				answering{name: "ot", language: other, fidelity: trust.Syntactic, skipped: true},
			)
			assert.Equal(t, got.Provenance.Fidelity, trust.Syntactic, "the tier of the answer")
		})
	})
}

// names returns the name of each declaration, in order.
func names(items []sema.Symbol) []string {
	out := make([]string, 0, len(items))
	for _, one := range items {
		out = append(out, one.Name)
	}
	return out
}
