// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang_test

import (
	"os"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/lsp"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("claims the wire form this module owns", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, string(golang.Declaration().Language), "go",
				"the wire form reaches an index that outlives the process, so it is pinned here")
		})

		t.Run("states every convention the registry demands", func(t *testing.T) {
			t.Parallel()
			d := golang.Declaration()
			assert.NotEmpty(t, d.Extensions, "without an extension nothing routes to this module")
			assert.NotNil(t, d.IsTest, "a nil convention panics on the first call")
			assert.NotNil(t, d.Namespace, "a nil convention panics on the first call")
			assert.NotNil(t, d.Visibility, "a nil convention panics on the first call")
			assert.NotEmpty(t, d.Comment.Line, "the document operations need a comment prefix no grammar states")
		})

		t.Run("claims .go", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, golang.Declaration().Extensions, ".go",
				"a file with this suffix is this language's to answer about")
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("can be run as it is declared", func(t *testing.T) {
			t.Parallel()
			// Checked here rather than when a call arrives. A server
			// declared without a language identity opens every file under
			// an empty name and is answered about nothing, which in a
			// tool whose job includes reporting that it found nothing is
			// the hardest failure to notice.
			assert.NoError(t, golang.Server().Valid(),
				"the declaration carries everything a server needs")
		})

		t.Run("names itself as it names the program to run", func(t *testing.T) {
			t.Parallel()
			// The name reaches a caller in a capability report and a
			// provenance. One that named a different program from the one
			// it starts would tell a caller to install the wrong thing.
			assert.Equal(t, golang.Server().Name, golang.Server().Command[0],
				"what answered and what was run are the same program")
		})

		t.Run("opens files under the identity the protocol names", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Server().LanguageID, lsp.IdentityGo,
				"the specification's own spelling, which is what a server matches on")
		})

		t.Run("claims only the roles binding answers", func(t *testing.T) {
			t.Parallel()
			// A server outlines one file no better than a parser does at
			// a thousandth of the speed, so claiming outline would make
			// every outline start a process to do worse.
			assert.Equal(t, golang.Server().Reaches(engine.RoleResolve), trust.Resolved,
				"a type checker binds names, which is what resolve asks about")
			assert.Equal(t, golang.Server().Reaches(engine.RoleOutline), trust.None,
				"and holds no evidence a parser does not already have for outline")
		})
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("puts the language in the registry and its engine in the catalogue", func(t *testing.T) {
			t.Parallel()
			r, c := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, golang.Register(lang.Workspace{FS: fstest.MapFS{}}, r, c),
				"a composition root registers this module with one call")
			assert.Length(t, r.Languages(), 1, "one call registers one language")
			assert.Length(t, c.For(t.Context(), golang.Declaration().Language, engine.RoleOutline), 1,
				"the parser is selectable for the role it serves")
		})

		t.Run("adds the server for a workspace on disk", func(t *testing.T) {
			t.Parallel()
			// The parser answers over any tree; the server needs one a
			// process can open files in. Both are registered here, and
			// the roles they claim do not overlap.
			registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
			held := lang.Workspace{FS: os.DirFS(t.TempDir()), Root: t.TempDir()}
			assert.NoError(t, golang.Register(held, registry, catalogue),
				"a workspace on disk registers both")

			assert.NotContains(t,
				serving(t, catalogue, golang.Declaration().Language, engine.RoleOutline),
				golang.Server().Name,
				"the parser keeps outline, which a server does no better and far slower")
		})

		t.Run("leaves the server out where a tree is nowhere", func(t *testing.T) {
			t.Parallel()
			// A server is a process that opens files by name. Registered
			// over a tree that was never written, it would fail on the
			// first call rather than never be offered.
			registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, golang.Register(lang.Workspace{FS: fstest.MapFS{}}, registry, catalogue),
				"a tree that is nowhere still registers a parser")
			assert.NotContains(t,
				serving(t, catalogue, golang.Declaration().Language, engine.RolePlan),
				golang.Server().Name,
				"and its server is not among what can answer")
		})

		t.Run("refuses a second registration of one language", func(t *testing.T) {
			t.Parallel()
			r, c := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, golang.Register(lang.Workspace{FS: fstest.MapFS{}}, r, c), "the first call registers")
			assert.HasError(t, golang.Register(lang.Workspace{FS: fstest.MapFS{}}, r, c),
				"two claims on one language would make routing depend on call order")
		})
	})
}

// serving is the engines a catalogue offers for a role, by name.
func serving(t *testing.T, c *engine.Catalog, l source.Language, role engine.Role) []string {
	t.Helper()
	held := c.For(t.Context(), l, role)
	out := make([]string, 0, len(held))
	for _, one := range held {
		out = append(out, one.Name())
	}
	return out
}

// An import is the one edge a parser can be correct about: the tags
// query already captures it, and neither end needs a name bound to
// anything. It is also the one relation a workspace with no language
// server installed can still ask for.
func TestImports(t *testing.T) {
	t.Parallel()

	// relating builds the engines over a small workspace and asks one
	// direction of one edge.
	relating := func(t *testing.T, of sema.ID, kind sema.RelationKind) engine.Result[sema.Relation] {
		t.Helper()
		registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
		held := lang.Workspace{FS: fstest.MapFS{
			"a.go": {Data: []byte("package p\n\nimport (\n\t\"fmt\"\n\t\"encoding/json\"\n)\n\ntype Store struct{}\n")},
			"b.go": {Data: []byte("package p\n\nimport \"fmt\"\n\nfunc Use() {}\n")},
			"c.go": {Data: []byte("package p\n\nfunc Alone() {}\n")},
		}}
		assert.NoError(t, golang.Register(held, registry, catalogue), "the module registers")

		for _, e := range catalogue.For(t.Context(), golang.Declaration().Language, engine.RoleRelate) {
			relator, serves := e.(engine.Relator)
			if !serves {
				continue
			}
			got, err := relator.Relate(t.Context(), engine.Request{Scope: ".", Preferred: trust.Syntactic},
				of, kind)
			if err == nil {
				return got
			}
		}
		t.Fatalf("no engine answered %s", kind)
		return engine.Result[sema.Relation]{}
	}

	t.Run("imported-by", func(t *testing.T) {
		t.Parallel()

		t.Run("names every file that brings a package into scope", func(t *testing.T) {
			t.Parallel()
			got := relating(t, sema.NewID(golang.Declaration().Language, ".", "fmt", sema.KindImport),
				sema.ImportedBy)

			assert.Equal(t, ends(got.Items), []string{"a.go", "b.go"},
				"both files that import it, and not the one that does not")
			assert.Equal(t, got.Completeness, trust.ScopeTotal,
				"the walk read every file in the scope")
		})

		t.Run("carries the line the import was written on", func(t *testing.T) {
			t.Parallel()
			got := relating(t, sema.NewID(golang.Declaration().Language, ".", "fmt", sema.KindImport),
				sema.ImportedBy)

			assert.Contains(t, got.Items[0].Via, "fmt",
				"a caller reading who depends on this wants to read the import")
		})

		t.Run("matches a package by the name it is written under", func(t *testing.T) {
			t.Parallel()
			// A caller asks for what it reads, which is the last segment
			// as often as the whole path.
			got := relating(t, sema.NewID(golang.Declaration().Language, ".", "json", sema.KindImport),
				sema.ImportedBy)

			assert.Equal(t, ends(got.Items), []string{"a.go"},
				"encoding/json is asked for as json")
		})
	})

	t.Run("imports", func(t *testing.T) {
		t.Parallel()

		t.Run("names what the file declaring a symbol brings into scope", func(t *testing.T) {
			t.Parallel()
			got := relating(t, sema.NewID(golang.Declaration().Language, ".", "Store", sema.KindStruct),
				sema.Imports)

			assert.Equal(t, ends(got.Items), []string{"fmt", "encoding/json"},
				"the imports of the file that declares it, in the order it wrote them, "+
					"and no other file's")
		})
	})

	t.Run("a direction that needs a binding", func(t *testing.T) {
		t.Parallel()

		t.Run("is declined rather than answered with none", func(t *testing.T) {
			t.Parallel()
			// A parser matched text. Answering none would be a claim
			// that nothing calls the declaration, over a tier that
			// cannot support one.
			registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t,
				golang.Register(lang.Workspace{FS: fstest.MapFS{
					"a.go": {Data: []byte("package p\n\nfunc Use() {}\n")},
				}}, registry, catalogue), "the module registers")

			for _, e := range catalogue.For(t.Context(), golang.Declaration().Language, engine.RoleRelate) {
				relator, serves := e.(engine.Relator)
				if !serves {
					continue
				}
				_, err := relator.Relate(t.Context(), engine.Request{Scope: "."},
					sema.NewID(golang.Declaration().Language, ".", "Use", sema.KindFunction),
					sema.CalledBy)
				assert.ErrorIs(t, err, engine.ErrDecline,
					"who calls this is a binding, and this engine has none")
			}
		})
	})
}

// ends is what each edge pointed at, in order.
func ends(held []sema.Relation) []string {
	out := make([]string, 0, len(held))
	for _, one := range held {
		out = append(out, one.To.Name)
	}
	return out
}
