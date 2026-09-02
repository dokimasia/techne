// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java_test

import (
	"os"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/java"
	"go.dokimi.dev/techne/lang/lsp"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("claims the wire form this module owns", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, string(java.Declaration().Language), "java",
				"the wire form reaches an index that outlives the process, so it is pinned here")
		})

		t.Run("states every convention the registry demands", func(t *testing.T) {
			t.Parallel()
			d := java.Declaration()
			assert.NotEmpty(t, d.Extensions, "without an extension nothing routes to this module")
			assert.NotNil(t, d.IsTest, "a nil convention panics on the first call")
			assert.NotNil(t, d.Namespace, "a nil convention panics on the first call")
			assert.NotNil(t, d.Visibility, "a nil convention panics on the first call")
			assert.NotEmpty(t, d.Comment.Line, "the document operations need a comment prefix no grammar states")
		})

		t.Run("claims .java", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, java.Declaration().Extensions, ".java",
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
			assert.NoError(t, java.Server().Valid(),
				"the declaration carries everything a server needs")
		})

		t.Run("names itself as it names the program to run", func(t *testing.T) {
			t.Parallel()
			// The name reaches a caller in a capability report and a
			// provenance. One that named a different program from the one
			// it starts would tell a caller to install the wrong thing.
			assert.Equal(t, java.Server().Name, java.Server().Command[0],
				"what answered and what was run are the same program")
		})

		t.Run("opens files under the identity the protocol names", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, java.Server().LanguageID, lsp.IdentityJava,
				"the specification's own spelling, which is what a server matches on")
		})

		t.Run("claims only the roles binding answers", func(t *testing.T) {
			t.Parallel()
			// A server outlines one file no better than a parser does at
			// a thousandth of the speed, so claiming outline would make
			// every outline start a process to do worse.
			assert.Equal(t, java.Server().Reaches(engine.RoleResolve), trust.Resolved,
				"a type checker binds names, which is what resolve asks about")
			assert.Equal(t, java.Server().Reaches(engine.RoleOutline), trust.None,
				"and holds no evidence a parser does not already have for outline")
		})
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("puts the language in the registry and its engine in the catalogue", func(t *testing.T) {
			t.Parallel()
			r, c := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, java.Register(lang.Workspace{FS: fstest.MapFS{}}, r, c),
				"a composition root registers this module with one call")
			assert.Length(t, r.Languages(), 1, "one call registers one language")
			assert.Length(t, c.For(t.Context(), java.Declaration().Language, engine.RoleOutline), 1,
				"the parser is selectable for the role it serves")
		})

		t.Run("adds the server for a workspace on disk", func(t *testing.T) {
			t.Parallel()
			// The parser answers over any tree; the server needs one a
			// process can open files in. Both are registered here, and
			// the roles they claim do not overlap.
			registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
			held := lang.Workspace{FS: os.DirFS(t.TempDir()), Root: t.TempDir()}
			assert.NoError(t, java.Register(held, registry, catalogue),
				"a workspace on disk registers both")

			assert.NotContains(t,
				serving(t, catalogue, java.Declaration().Language, engine.RoleOutline),
				java.Server().Name,
				"the parser keeps outline, which a server does no better and far slower")
		})

		t.Run("leaves the server out where a tree is nowhere", func(t *testing.T) {
			t.Parallel()
			// A server is a process that opens files by name. Registered
			// over a tree that was never written, it would fail on the
			// first call rather than never be offered.
			registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, java.Register(lang.Workspace{FS: fstest.MapFS{}}, registry, catalogue),
				"a tree that is nowhere still registers a parser")
			assert.NotContains(t,
				serving(t, catalogue, java.Declaration().Language, engine.RolePlan),
				java.Server().Name,
				"and its server is not among what can answer")
		})

		t.Run("refuses a second registration of one language", func(t *testing.T) {
			t.Parallel()
			r, c := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, java.Register(lang.Workspace{FS: fstest.MapFS{}}, r, c), "the first call registers")
			assert.HasError(t, java.Register(lang.Workspace{FS: fstest.MapFS{}}, r, c),
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
