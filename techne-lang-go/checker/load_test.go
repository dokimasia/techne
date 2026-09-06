// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/go/checker"
)

// A cached reading answers the second question for nothing, and stops
// describing the workspace the moment a file in it moves.
func TestEngineCache(t *testing.T) {
	t.Parallel()

	t.Run("what was type-checked", func(t *testing.T) {
		t.Parallel()

		t.Run("is read again when a file changes", func(t *testing.T) {
			t.Parallel()
			root := workspace(t, whole())
			e, err := checker.New(root, golang.Declaration())
			assert.NoError(t, err, "an engine builds over the module")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "the first reading succeeds")
			assert.Empty(t, got.Items, "and the module compiles")

			assert.NoError(t,
				os.WriteFile(filepath.Join(root, "use.go"), []byte(broken), 0o644),
				"something rewrites a file, as the write path does")

			got, err = e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "asking again succeeds")
			assert.NotEmpty(t, got.Items, "and the answer is about what is there now")
		})

		t.Run("is read again when a file is added", func(t *testing.T) {
			t.Parallel()
			// The loaded file list would not notice: a package that
			// gained a file since the last reading is one the cache
			// would answer about as though the file were not there.
			root := workspace(t, whole())
			e, err := checker.New(root, golang.Declaration())
			assert.NoError(t, err, "an engine builds over the module")

			_, err = e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "the first reading succeeds")

			assert.NoError(t,
				os.WriteFile(filepath.Join(root, "extra.go"), []byte(broken), 0o644),
				"and a file appears")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "asking again succeeds")
			assert.NotEmpty(t, got.Items, "and the new file was read")
		})
	})
}
