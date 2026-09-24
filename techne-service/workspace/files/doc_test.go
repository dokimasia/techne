// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package files_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/service/workspace/files"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("FS", func(t *testing.T) {
		t.Parallel()

		t.Run("serves the directory that Write changes", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			written(t, dir, "a.go", "old\n", 0o644)
			before, err := fs.ReadFile(root.FS(), "a.go")
			assert.NoError(t, err, "ReadFile of a.go")
			assert.Equal(t, string(before), "old\n", "the content of a.go before Write")
			assert.NoError(t, root.Write("a.go", []byte("new\n")), "Write of a.go")
			after, err := fs.ReadFile(root.FS(), "a.go")
			assert.NoError(t, err, "ReadFile of a.go")
			assert.Equal(t, string(after), "new\n", "the content of a.go after Write")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a path through .. out of the directory", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			outside := filepath.Join(filepath.Dir(dir), "escaped.go")
			assert.NoError(t, os.WriteFile(outside, []byte("secret\n"), 0o600), "WriteFile of escaped.go")
			_, err := root.Read("../escaped.go")
			assert.HasError(t, err, "the error of Read")
			assert.HasError(t, root.Write("../escaped.go", []byte("x")), "the error of Write")
			assert.Equal(t, read(t, filepath.Dir(dir), "escaped.go"), "secret\n", "the content of escaped.go")
		})

		t.Run("leaves the original file when the staged file cannot be written", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			written(t, dir, "a.go", "original\n", 0o644)
			assert.NoError(t, os.Mkdir(filepath.Join(dir, "a.go"+files.Partial), 0o755), "Mkdir of the staged name")
			assert.HasError(t, root.Write("a.go", []byte("replacement\n")), "the error of Write")
			assert.Equal(t, read(t, dir, "a.go"), "original\n", "the content of a.go")
		})
	})
}
