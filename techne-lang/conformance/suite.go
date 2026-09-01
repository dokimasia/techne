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

	// Declares is the whole outline of Files, and is compared as a set.
	// A symbol found and not listed fails the suite exactly as one
	// listed and not found does.
	//
	// It is exact because the alternative proved nothing. Checking only
	// that listed symbols appear passes a query that finds a quarter of
	// the language, which is what every query here did: Go reported no
	// constant and no interface, Python no method, and eight of the
	// fourteen kinds were produced by nothing at all. A fixture must
	// therefore name every declaration form its language has.
	Declares []Declared

	// Unclaimed is a path whose extension the language does not claim.
	// Outlining it must find nothing rather than refuse, because a
	// directory holding several languages is the normal case.
	Unclaimed string
}

// Declared is one symbol a module says its source declares.
//
// Name, Kind and Visibility are compared as a whole set. The rest are
// checked only where the module states them, because a fixture cannot
// exercise every declaration's metadata without becoming unreadable.
type Declared struct {
	Name string
	Kind sema.Kind
	// Visibility is what the module expects the parser to report. A
	// language spelling visibility as a modifier expects
	// [sema.VisibilityUnknown], because a name carries nothing of it.
	Visibility sema.Visibility
	// Signature is the declaration without its body, checked where a
	// module states one. Stating it pins the shapes that are hard: a
	// declaration whose grammar names no body field, a member that must
	// not take its container's text, and one written under an
	// annotation that must not take it.
	Signature string
	// Doc is the documentation the fixture writes on this declaration,
	// in whichever form the language's own documentation tool reads.
	Doc string
	// Annotations are the annotations the declaration carries.
	Annotations []Annotated
	// Modifiers are the keywords the declaration carries.
	Modifiers []string
}

// Annotated is one annotation a module says a declaration carries.
type Annotated struct {
	Name string
	// Text is the whole annotation as written, punctuation and arguments
	// included. It is checked only where a module states it, and stating
	// it is what pins the bytes a tool reproducing the annotation needs.
	Text string
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

		// One adapter serves every grammar, so an engine's name carries
		// the language it was built for. Were it constant, a catalogue
		// holding two languages would refuse the second and a provenance
		// would not say which answered.
		assert.Contains(t, e.Name(), string(s.Declaration.Language),
			"an engine names the language it serves, so instances of one adapter do not collide")
	})

	t.Run("outline", func(t *testing.T) {
		t.Parallel()
		e := build(t, fsys, s)
		got := outline(t, e, ".")

		t.Run("finds exactly what the module declares", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, summarise(got.Items), expected(s.Declares),
				"the fixture is the whole outline: a declaration the query "+
					"misses and a symbol it invents are the same failure")
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

		t.Run("carries the source text its span names", func(t *testing.T) {
			t.Parallel()
			for _, sym := range got.Items {
				content, known := s.Files[string(sym.Span.Path)]
				if !known {
					continue
				}
				assert.True(t, sym.Span.End.Offset <= len(content),
					"a span names bytes inside the file it points at")
				assert.Equal(t, sym.Snippet, content[sym.Span.Start.Offset:sym.Span.End.Offset],
					"the snippet is the span's own bytes, so the two cannot describe different code")
				assert.Contains(t, sym.Snippet, sym.Name,
					"a declaration's own text contains the name it declares")
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

		t.Run("carries the metadata the module names", func(t *testing.T) {
			t.Parallel()
			for _, want := range s.Declares {
				if len(want.Annotations) == 0 && len(want.Modifiers) == 0 && want.Signature == "" {
					continue
				}
				// A name and kind can repeat: an interface and the class
				// implementing it both declare get, and only one carries
				// the annotation. The check is that some declaration of
				// that name and kind carries it.
				matching := every(got.Items, want)
				if len(matching) == 0 {
					continue // the exact-set check already reports this
				}
				for _, annotation := range want.Annotations {
					assert.True(t, anyAnnotated(matching, annotation),
						"metadata decides what a tool rewriting the declaration must reproduce, "+
							"so it is read whole rather than dropped or trimmed")
				}
				if want.Signature != "" {
					assert.True(t, anySigned(matching, want.Signature),
						"a signature is what a caller needs to call a declaration or "+
							"implement it, and holds nothing of how it works")
				}
				for _, keyword := range want.Modifiers {
					assert.True(t, anyModified(matching, keyword),
						"for most of these languages the keywords are the only place "+
							"visibility is written")
				}
			}
		})

		t.Run("reads the documentation the language writes", func(t *testing.T) {
			t.Parallel()
			if !documents(s.Declares) {
				t.Skip("this module's fixture writes no documentation")
			}
			// Exact, like the outline. A comment read as documentation
			// that the module did not name fails as surely as one it
			// named and the parser did not find, which is the whole
			// reason a language states which of its comment forms
			// document and which do not.
			assert.Equal(t, documented(got.Items), expectedDocs(s.Declares),
				"a language states the forms its own documentation tool reads, "+
					"and a comment written in any other form is not documentation")
		})

		t.Run("answers two identical requests identically", func(t *testing.T) {
			t.Parallel()
			again := outline(t, e, ".")
			assert.Equal(t, again.Items, got.Items,
				"a caller diffing two runs sees only changes somebody made")
		})
	})

	t.Run("search", func(t *testing.T) {
		t.Parallel()
		e := build(t, fsys, s)

		t.Run("finds a declaration by its exact name", func(t *testing.T) {
			t.Parallel()
			if len(s.Declares) == 0 {
				t.Skip("the module names no declaration to search for")
			}
			wanted := s.Declares[0]
			got, err := e.Search(t.Context(), engine.Request{Scope: "."}, engine.Query{
				Text: wanted.Name, Private: true,
			})
			assert.NoError(t, err, "searching a scope that exists succeeds")
			assert.True(t, held(got.Items, wanted),
				"a name the module says it declares is findable by that name")
		})

		t.Run("puts an exact match before a longer one containing it", func(t *testing.T) {
			t.Parallel()
			if len(s.Declares) == 0 {
				t.Skip("the module names no declaration to search for")
			}
			wanted := s.Declares[0]
			got, err := e.Search(t.Context(), engine.Request{Scope: "."}, engine.Query{
				Text: wanted.Name, Private: true,
			})
			assert.NoError(t, err, "searching a scope that exists succeeds")
			assert.NotEmpty(t, got.Items, "the declaration the module named is found")
			assert.Equal(t, got.Items[0].Name, wanted.Name,
				"an engine returns its own best order, and an exact match is the best")
		})

		t.Run("finds nothing for a name nothing declares", func(t *testing.T) {
			t.Parallel()
			got, err := e.Search(t.Context(), engine.Request{Scope: "."}, engine.Query{
				Text: "aNameNoModuleWouldDeclare", Private: true,
			})
			assert.NoError(t, err, "finding nothing is an answer, not a fault")
			assert.Empty(t, got.Items, "a parser reports what it matched and invents nothing")
			assert.False(t, engine.Publish(got, e, engine.RoleSearch, trust.None).
				Provenance.SupportsNegativeClaim(),
				"an empty search at this tier means none were found, never that there are none")
		})

		t.Run("answers two identical searches identically", func(t *testing.T) {
			t.Parallel()
			q := engine.Query{Text: "e", Private: true}
			first, err := e.Search(t.Context(), engine.Request{Scope: "."}, q)
			assert.NoError(t, err, "searching a scope that exists succeeds")
			second, err := e.Search(t.Context(), engine.Request{Scope: "."}, q)
			assert.NoError(t, err, "searching a scope that exists succeeds")
			assert.Equal(t, second.Items, first.Items,
				"identity breaks any remaining tie, so the order does not wander")
		})
	})

	t.Run("refuses a query naming a capture no kind carries", func(t *testing.T) {
		t.Parallel()
		// A capture the vocabulary does not know would match and then be
		// dropped, so the pattern would find nothing and say nothing.
		// That has to fail at startup, because an engine that silently
		// finds nothing is the hardest failure to notice in a system
		// whose job includes reporting that it found nothing.
		spoiled := s.Grammar
		spoiled.Tags = s.Grammar.Tags + "\n((_) @definition.no_such_shape)"
		_, err := treesitter.New(fsys, s.Declaration, spoiled)
		assert.ErrorIs(t, err, treesitter.ErrUnknownCapture,
			"a mistyped capture must stop startup rather than produce an engine that finds nothing")
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

// summarise renders what the outline found, sorted, so a mismatch fails
// with both whole sets beside each other.
func summarise(found []sema.Symbol) string {
	seen := make([]string, 0, len(found))
	for _, sym := range found {
		seen = append(seen, fmt.Sprintf("%s %s/%s", sym.Kind, sym.Name, sym.Visibility))
	}
	return list(seen)
}

// expected renders what the module says its fixture declares, in the
// form summarise produces.
func expected(want []Declared) string {
	named := make([]string, 0, len(want))
	for _, d := range want {
		named = append(named, fmt.Sprintf("%s %s/%s", d.Kind, d.Name, d.Visibility))
	}
	return list(named)
}

func list(of []string) string {
	sort.Strings(of)
	return "[" + strings.Join(of, ", ") + "]"
}

// every returns each symbol matching a declared name and kind.
func every(in []sema.Symbol, want Declared) []sema.Symbol {
	var out []sema.Symbol
	for _, sym := range in {
		if sym.Name == want.Name && sym.Kind == want.Kind {
			out = append(out, sym)
		}
	}
	return out
}

func anyAnnotated(in []sema.Symbol, want Annotated) bool {
	for _, sym := range in {
		for _, a := range sym.Annotations {
			if a.Name != want.Name {
				continue
			}
			if want.Text == "" || a.Text == want.Text {
				return true
			}
		}
	}
	return false
}

func anySigned(in []sema.Symbol, signature string) bool {
	for _, sym := range in {
		if sym.Signature == signature {
			return true
		}
	}
	return false
}

func anyModified(in []sema.Symbol, keyword string) bool {
	for _, sym := range in {
		if sym.Modified(keyword) {
			return true
		}
	}
	return false
}

// documents reports whether a fixture writes any documentation, so a
// module that exercises none is skipped rather than held to an empty
// expectation it never stated.
func documents(want []Declared) bool {
	for _, d := range want {
		if d.Doc != "" {
			return true
		}
	}
	return false
}

// documented renders every declaration the parser attached
// documentation to, sorted, so a mismatch shows both whole sets beside
// each other.
func documented(found []sema.Symbol) string {
	seen := make([]string, 0, len(found))
	for _, sym := range found {
		if sym.Doc == "" {
			continue
		}
		seen = append(seen, fmt.Sprintf("%s %s: %q", sym.Kind, sym.Name, sym.Doc))
	}
	return list(seen)
}

// expectedDocs renders what the module says its fixture documents, in
// the form documented produces.
func expectedDocs(want []Declared) string {
	named := make([]string, 0, len(want))
	for _, d := range want {
		if d.Doc == "" {
			continue
		}
		named = append(named, fmt.Sprintf("%s %s: %q", d.Kind, d.Name, d.Doc))
	}
	return list(named)
}

// held reports whether a search found one expected declaration. A search
// is asked for one name, so unlike an outline it is a subset check.
func held(found []sema.Symbol, want Declared) bool {
	for _, sym := range found {
		if sym.Name == want.Name && sym.Kind == want.Kind && sym.Visibility == want.Visibility {
			return true
		}
	}
	return false
}
