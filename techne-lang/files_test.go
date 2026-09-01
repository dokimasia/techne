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

func TestFilesIn(t *testing.T) {
	t.Parallel()

	tree := func() fstest.MapFS {
		return fstest.MapFS{
			"a.fx":         {Data: []byte("a")},
			"b.txt":        {Data: []byte("b")},
			"pkg/c.fx":     {Data: []byte("c")},
			"pkg/d.other":  {Data: []byte("d")},
			"pkg/sub/e.fx": {Data: []byte("e")},
			"Makefile":     {Data: []byte("m")},
		}
	}
	claimed := []string{".fx"}

	t.Run("a directory", func(t *testing.T) {
		t.Parallel()

		t.Run("yields every claimed file beneath it", func(t *testing.T) {
			t.Parallel()
			got, err := lang.FilesIn(tree(), ".", claimed)
			assert.NoError(t, err, "a directory that exists is walked")
			assert.Equal(t, got, []source.Path{"a.fx", "pkg/c.fx", "pkg/sub/e.fx"},
				"every file the language claims, at any depth, and nothing else")
		})

		t.Run("yields them in a stable order", func(t *testing.T) {
			t.Parallel()
			// Two identical requests must answer identically, or a
			// caller diffing two runs sees changes nobody made.
			first, err := lang.FilesIn(tree(), ".", claimed)
			assert.NoError(t, err, "a directory that exists is walked")
			second, err := lang.FilesIn(tree(), ".", claimed)
			assert.NoError(t, err, "a directory that exists is walked")
			assert.Equal(t, first, second,
				"two identical requests answer identically, so a caller diffing runs sees only real changes")
		})

		t.Run("skips a file the language does not claim", func(t *testing.T) {
			t.Parallel()
			got, err := lang.FilesIn(tree(), ".", claimed)
			assert.NoError(t, err, "a directory that exists is walked")
			for _, unclaimed := range []source.Path{"b.txt", "pkg/d.other", "Makefile"} {
				assert.NotContains(t, got, unclaimed, "a file another language owns is not this language's to outline")
			}
		})

		t.Run("narrows to the subtree asked for", func(t *testing.T) {
			t.Parallel()
			got, err := lang.FilesIn(tree(), "pkg/sub", claimed)
			assert.NoError(t, err, "a directory that exists is walked")
			assert.Equal(t, got, []source.Path{"pkg/sub/e.fx"}, "a scope narrows the walk to its own subtree")
		})
	})

	t.Run("a file", func(t *testing.T) {
		t.Parallel()

		t.Run("yields itself when the language claims it", func(t *testing.T) {
			t.Parallel()
			got, err := lang.FilesIn(tree(), "pkg/c.fx", claimed)
			assert.NoError(t, err, "a file that exists is in scope")
			assert.Equal(t, got, []source.Path{"pkg/c.fx"}, "a file scope yields that file")
		})

		t.Run("yields nothing when the language does not claim it", func(t *testing.T) {
			t.Parallel()
			// A directory holding several languages is the normal case,
			// so a file another language owns is skipped rather than
			// refused.
			got, err := lang.FilesIn(tree(), "b.txt", claimed)
			assert.NoError(t, err, "a mixed directory is the normal case, so another language's file is not an error")
			assert.Empty(t, got, "a file this language does not claim yields nothing")
		})
	})

	t.Run("a scope that is not there", func(t *testing.T) {
		t.Parallel()

		t.Run("is an error, not an empty answer", func(t *testing.T) {
			t.Parallel()
			// An empty answer would read as "this directory declares
			// nothing", which is a different fact from "there is no
			// such directory".
			_, err := lang.FilesIn(tree(), "nowhere", claimed)
			assert.HasError(t, err,
				"answering nothing would read as this directory declaring nothing, which is a different fact")
		})
	})
}
