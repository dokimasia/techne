// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engines_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/engines"
)

// TestDoc covers the rule of the package comment: the workspace decides whether a language has
// a server engine, and the language does not.
func TestDoc(t *testing.T) {
	t.Parallel()

	languages := []lang.Declaration{declared(), other()}

	t.Run("Serving", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a server engine for every language over a workspace on disk", func(t *testing.T) {
			t.Parallel()
			w := onDisk(t)
			for _, d := range languages {
				_, has, err := engines.Serving(w, d, served(), nil)
				assert.NoError(t, err, "the error of Serving for "+string(d.Language))
				assert.True(t, has, "the report of a server engine for "+string(d.Language))
			}
		})

		t.Run("returns no server engine for any language over a workspace in memory", func(t *testing.T) {
			t.Parallel()
			w := inMemory()
			for _, d := range languages {
				_, has, err := engines.Serving(w, d, served(), nil)
				assert.NoError(t, err, "the error of Serving for "+string(d.Language))
				assert.False(t, has, "the report of a server engine for "+string(d.Language))
			}
		})
	})
}

// other returns the declaration of a second language, other, with the extension .other.
func other() lang.Declaration {
	out := declared()
	out.Language = "other"
	out.Extensions = []string{".other"}
	return out
}
