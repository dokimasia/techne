// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package conformance

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Suite is what a language module hands to [Run].
type Suite struct {
	// Declaration is the module's own, exactly as it registers it.
	Declaration lang.Declaration

	// Grammar is the compiled grammar and the tags query that goes with
	// it.
	Grammar treesitter.Grammar

	// Files is the source the checks outline, keyed by path relative to
	// the workspace root. Every path must carry an extension the
	// declaration claims.
	Files map[string]string

	// Declares names what outlining Files must find. The suite fails
	// when one is missing; it does not fail on a symbol found that is
	// not listed, because a grammar may capture more than a module
	// chooses to enumerate.
	Declares []Declared

	// Unclaimed is a path whose extension the language does not claim.
	// Outlining it must find nothing rather than refuse, because a
	// directory holding several languages is the normal case.
	Unclaimed string
}

// Declared is one symbol a module says its source declares.
type Declared struct {
	Name string
	Kind sema.Kind
	// Visibility is what the module expects the parser to report. A
	// language spelling visibility as a modifier expects
	// [sema.VisibilityUnknown], because a name carries nothing of it.
	Visibility sema.Visibility
}

// Run applies every check to one language module.
func Run(t *testing.T, s Suite) {
	t.Helper()

	fsys := fstest.MapFS{}
	for path, content := range s.Files {
		fsys[path] = &fstest.MapFile{Data: []byte(content)}
	}
	if s.Unclaimed != "" {
		fsys[s.Unclaimed] = &fstest.MapFile{Data: []byte("not this language\n")}
	}

	t.Run("registration", func(t *testing.T) {
		t.Parallel()
		registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
		e := build(t, fsys, s)
		assert.NoError(t, registry.Register(catalogue, s.Declaration, e),
			"a module's own declaration must satisfy the rules it will be registered under")

		for _, suffix := range s.Declaration.Extensions {
			got, routed := registry.LanguageOf(source.Path("a/b" + suffix))
			assert.True(t, routed, "every extension a module claims must route back to it")
			assert.Equal(t, got, s.Declaration.Language,
				"every extension a module claims must route back to it")
		}

		assert.Length(t, catalogue.For(t.Context(), s.Declaration.Language, engine.RoleOutline), 1,
			"a registered module's engine is selectable for the role it serves")
	})

	t.Run("outline", func(t *testing.T) {
		t.Parallel()
		e := build(t, fsys, s)
		got := outline(t, e, ".")

		t.Run("finds what the module declares", func(t *testing.T) {
			t.Parallel()
			for _, want := range s.Declares {
				assert.True(t, held(got.Items, want), fmt.Sprintf(
					"the vendored tags query captures %s %s (visibility %s), "+
						"which the module says it declares; found %s",
					want.Kind, want.Name, want.Visibility, summarise(got.Items)))
			}
		})

		t.Run("returns only well-formed symbols", func(t *testing.T) {
			t.Parallel()
			for _, sym := range got.Items {
				assert.NotEmpty(t, string(sym.ID), "a symbol an index may store carries an identity")
				assert.NotEmpty(t, sym.Name, "a symbol a caller may act on carries a name")
				assert.NotEqual(t, sym.Kind, sema.KindUnknown,
					"a capture that declares nothing is skipped rather than emitted as unknown")
				assert.Equal(t, sym.Language, s.Declaration.Language,
					"an engine answers about the one language it was built for")
				assert.NotEmpty(t, string(sym.Span.Path), "a symbol says which file declares it")
			}
		})

		t.Run("reports it covered the whole scope", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, got.Completeness, trust.ScopeTotal,
				"the walk read every file in scope, which only the engine can say")
		})

		t.Run("carries the caveat that a cross-file name is coincidence", func(t *testing.T) {
			t.Parallel()
			var carried bool
			for _, c := range got.Caveats {
				carried = carried || c.Code == trust.CaveatDynamic
			}
			assert.True(t, carried,
				"a caller must be told what this tier cannot see before trusting a name it matched")
		})

		t.Run("publishes as syntactic evidence that proves no absence", func(t *testing.T) {
			t.Parallel()
			// What a caller sees is the published answer. The engine
			// supplies coverage and caveats; Publish supplies the tier.
			published := engine.Publish(got, e, engine.RoleOutline, trust.None)
			assert.Equal(t, published.Provenance.Fidelity, trust.Syntactic,
				"a parser matched text, and the published answer says so")
			assert.False(t, published.Provenance.SupportsNegativeClaim(),
				"a parser's empty answer means none were found, never that there are none")
			assert.True(t, published.Status.Answered(), "an engine ran and returned what it found")
		})

		t.Run("answers two identical requests identically", func(t *testing.T) {
			t.Parallel()
			again := outline(t, e, ".")
			assert.Equal(t, again.Items, got.Items,
				"a caller diffing two runs sees only changes somebody made")
		})
	})

	t.Run("a file the language does not claim", func(t *testing.T) {
		t.Parallel()
		if s.Unclaimed == "" {
			t.Skip("the module supplied no unclaimed path")
		}
		e := build(t, fsys, s)
		assert.Empty(t, outline(t, e, source.Path(s.Unclaimed)).Items,
			"a directory holding several languages is normal, so another language's file yields nothing")
	})
}

// build returns the engine under test, failing the suite when a module's
// grammar or declaration is incomplete.
func build(t *testing.T, fsys fstest.MapFS, s Suite) *treesitter.Engine {
	t.Helper()
	e, err := treesitter.New(fsys, s.Declaration, s.Grammar)
	assert.NoError(t, err, "a module's grammar and declaration must produce a working engine")
	t.Cleanup(e.Close)
	return e
}

// outline runs the engine over a scope.
func outline(t *testing.T, e *treesitter.Engine, scope source.Path) engine.Result[sema.Symbol] {
	t.Helper()
	got, err := e.Outline(t.Context(), engine.Request{Scope: scope})
	assert.NoError(t, err, "outlining a scope that exists succeeds")
	return got
}

// summarise renders what the outline found, so a missing declaration
// fails with the alternatives beside it rather than only its own name.
func summarise(found []sema.Symbol) string {
	seen := make([]string, 0, len(found))
	for _, sym := range found {
		seen = append(seen, fmt.Sprintf("%s %s/%s", sym.Kind, sym.Name, sym.Visibility))
	}
	sort.Strings(seen)
	return "[" + strings.Join(seen, ", ") + "]"
}

// held reports whether the outline found one expected declaration.
func held(found []sema.Symbol, want Declared) bool {
	for _, sym := range found {
		if sym.Name == want.Name && sym.Kind == want.Kind && sym.Visibility == want.Visibility {
			return true
		}
	}
	return false
}
