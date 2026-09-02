// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/treesitter"
)

// What Index returns for real source is what Outline returns for that
// file, which the conformance suite asserts inside a language module
// where a grammar exists. What this checks is the part an index acts on:
// how far a change to one file reaches.
func TestIndex(t *testing.T) {
	t.Parallel()

	t.Run("Granularity", func(t *testing.T) {
		t.Parallel()

		t.Run("is one file, which is what makes this engine affordable", func(t *testing.T) {
			t.Parallel()
			// A parser reads one file and resolves nothing across files,
			// so its facts about one cannot go stale because another
			// changed. An engine reporting the workspace would have an
			// index rebuild everything per keystroke to serve answers a
			// live query produces for the same price.
			var e engine.Indexer = (*treesitter.Engine)(nil)
			assert.Equal(t, e.Granularity(), engine.InvalidateFile,
				"a change to one file stales that file and nothing else")
		})
	})

	t.Run("Affected", func(t *testing.T) {
		t.Parallel()

		t.Run("is the changed file and nothing else", func(t *testing.T) {
			t.Parallel()
			var e engine.Indexer = (*treesitter.Engine)(nil)
			assert.Equal(t, e.Affected("pkg/a.fx"), []source.Path{"pkg/a.fx"},
				"nothing else this engine said depends on it")
		})

		t.Run("names the file whatever it is called", func(t *testing.T) {
			t.Parallel()
			// Asked about a path this language does not claim, an index
			// still gets a straight answer: reindexing it costs one
			// skipped read, and guessing at more would cost a walk.
			var e engine.Indexer = (*treesitter.Engine)(nil)
			assert.Equal(t, e.Affected("notes.md"), []source.Path{"notes.md"},
				"the answer turns on the engine rather than on the path")
		})
	})
}
