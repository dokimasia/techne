// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engines_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/engines"
)

// TestDoc covers the contract the package comment states: which engines
// follow from a workspace, decided here rather than ten times over.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("which engines a workspace supports", func(t *testing.T) {
		t.Parallel()

		t.Run("is settled the same way for every language", func(t *testing.T) {
			t.Parallel()
			// Ten modules asking the question themselves would be nine
			// chances for one to answer differently, and a language
			// served differently from its siblings by accident is the
			// kind of gap nothing reports.
			held := onDisk(t)
			for _, of := range []lang.Declaration{declared(), other()} {
				_, has, err := engines.Serving(held, of, served())
				assert.NoError(t, err, "each language builds over the same workspace")
				assert.True(t, has, "and reaches the same answer about it")
			}
		})

		t.Run("turns on the workspace rather than on the language", func(t *testing.T) {
			t.Parallel()
			// The only thing that decides is whether a process can open
			// files by name. Nothing about the language enters into it.
			_, onDisc, err := engines.Serving(onDisk(t), declared(), served())
			assert.NoError(t, err, "a directory supports a server")

			_, elsewhere, err := engines.Serving(nowhere(), declared(), served())
			assert.NoError(t, err, "and a tree that is nowhere is not a fault")

			assert.True(t, onDisc, "the same language and the same server")
			assert.False(t, elsewhere, "answered differently only by where the tree is")
		})
	})

	t.Run("a server that is not installed", func(t *testing.T) {
		t.Parallel()

		t.Run("is registered rather than dropped", func(t *testing.T) {
			t.Parallel()
			// A language that vanished with its server would read as one
			// techne cannot serve at all, which is a conclusion a caller
			// would act on and be wrong about.
			_, has, err := engines.Serving(onDisk(t), declared(), served())
			assert.NoError(t, err, "the declaration is sound whether or not the program is here")
			assert.True(t, has, "so the language keeps its server")
		})
	})
}

// other is a second language, to show the decision does not turn on
// which one is asking.
func other() lang.Declaration {
	held := declared()
	held.Language = "other"
	held.Extensions = []string{".other"}
	return held
}
