// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

func TestVendored(t *testing.T) {
	t.Parallel()

	t.Run("Vendored", func(t *testing.T) {
		t.Parallel()

		t.Run("names the trees a workspace did not write", func(t *testing.T) {
			t.Parallel()
			// Each of these holds source in a language techne serves,
			// which is what makes skipping it worth doing rather than
			// harmless.
			for _, held := range []string{
				".git", "node_modules", "vendor", "target", ".venv",
				"__pycache__", "site-packages", ".bloop", ".gradle",
			} {
				assert.True(t, lang.Vendored(held),
					"a walk that descends here answers about code nobody asked about: "+held)
			}
		})

		t.Run("leaves alone a name that is output for some and source for others", func(t *testing.T) {
			t.Parallel()
			// Nothing here matches a compiled artefact's extension, so
			// walking these costs a caller nothing. A project keeping
			// real source under one of them would lose it.
			for _, held := range []string{"build", "dist", "out", "bin", "obj", "lib", "src"} {
				assert.False(t, lang.Vendored(held),
					"guessing here trades noise for silence, which is the worse trade: "+held)
			}
		})

		t.Run("answers on the name alone", func(t *testing.T) {
			t.Parallel()
			// No filesystem and no context, so a walk asks it per
			// directory before deciding whether to descend.
			assert.False(t, lang.Vendored("node_modules/react"),
				"it is a name rather than a path")
			assert.False(t, lang.Vendored(""), "and an empty one is nobody's")
		})
	})
}

func TestFilesInVendored(t *testing.T) {
	t.Parallel()

	tree := func() fstest.MapFS {
		return fstest.MapFS{
			"main.fx":                       {Data: []byte("a")},
			"pkg/held.fx":                   {Data: []byte("b")},
			"node_modules/dep/index.fx":     {Data: []byte("c")},
			"node_modules/dep/deep/one.fx":  {Data: []byte("d")},
			"pkg/node_modules/other/two.fx": {Data: []byte("e")},
			".git/objects/three.fx":         {Data: []byte("f")},
			"target/generated/four.fx":      {Data: []byte("g")},
		}
	}
	claimed := []string{".fx"}

	t.Run("a walk", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps what the workspace wrote and nothing else", func(t *testing.T) {
			t.Parallel()
			// The defect this exists for: a two-declaration project
			// searched for a name answered with a hundred and
			// twenty-five matches, every one in a dependency, reported
			// as total coverage of the workspace.
			got, err := lang.FilesIn(tree(), ".", claimed)
			assert.NoError(t, err, "the workspace is walked")
			assert.Equal(t, got, []source.Path{"main.fx", "pkg/held.fx"},
				"a dependency tree is not this workspace's code")
		})

		t.Run("skips one at any depth", func(t *testing.T) {
			t.Parallel()
			// A monorepo has one per package, and none of them is
			// anchored to the root.
			got, err := lang.FilesIn(tree(), "pkg", claimed)
			assert.NoError(t, err, "a subdirectory is walked")
			assert.Equal(t, got, []source.Path{"pkg/held.fx"},
				"the one nested inside it is skipped too")
		})

		t.Run("answers about one a caller named on purpose", func(t *testing.T) {
			t.Parallel()
			// Naming it is a choice. Answering nothing would report a
			// directory full of code as empty.
			got, err := lang.FilesIn(tree(), "node_modules", claimed)
			assert.NoError(t, err, "a scope that exists is walked")
			assert.Equal(t, got,
				[]source.Path{"node_modules/dep/deep/one.fx", "node_modules/dep/index.fx"},
				"the scope itself is never skipped, at any depth beneath it")
		})
	})
}
