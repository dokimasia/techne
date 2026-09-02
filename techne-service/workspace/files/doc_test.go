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

// TestDoc covers the claims the package comment makes.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("one root", func(t *testing.T) {
		t.Parallel()

		t.Run("serves reading and writing from the same directory", func(t *testing.T) {
			t.Parallel()
			// An engine reads through the FS half and the write path
			// writes through the other. Two roots would let a plan be
			// computed against one tree and applied to another.
			root, dir := opened(t)
			assert.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("old\n"), 0o644),
				"the test can prepare the directory")

			read, err := fs.ReadFile(root.FS(), "a.go")
			assert.NoError(t, err, "the reading half sees the directory")
			assert.Equal(t, string(read), "old\n", "and reads what is in it")

			assert.NoError(t, root.Write("a.go", []byte("new\n")), "the writing half writes to it")
			after, err := fs.ReadFile(root.FS(), "a.go")
			assert.NoError(t, err, "the reading half sees the write")
			assert.Equal(t, string(after), "new\n", "so both halves are one directory")
		})
	})

	t.Run("a path that would leave the root", func(t *testing.T) {
		t.Parallel()

		t.Run("is refused by the operating system, not by a check here", func(t *testing.T) {
			t.Parallel()
			// A check written here would have to be right every time.
			// This one is right because it is not this package's to get
			// wrong.
			root, dir := opened(t)
			outside := filepath.Join(filepath.Dir(dir), "escaped.go")
			assert.NoError(t, os.WriteFile(outside, []byte("secret\n"), 0o644),
				"the test can put a file outside the root")

			_, err := root.Read("../escaped.go")
			assert.HasError(t, err, "a path climbing out is refused")
			assert.HasError(t, root.Write("../escaped.go", []byte("x")),
				"and so is a write to one")
			held, err := os.ReadFile(outside)
			assert.NoError(t, err, "the file outside is still there")
			assert.Equal(t, string(held), "secret\n", "and untouched")
		})
	})

	t.Run("a write", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves the original when it cannot finish", func(t *testing.T) {
			t.Parallel()
			// The bytes are staged beside the target and renamed over
			// it, so a process that stops partway leaves the original
			// rather than half of each.
			root, dir := opened(t)
			assert.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("original\n"), 0o644),
				"the test can prepare the directory")

			// A directory in the staged file's place is a rename that
			// cannot succeed, which is the shape of a write that fails
			// after staging.
			assert.NoError(t, os.Mkdir(filepath.Join(dir, "a.go"+files.Partial), 0o755),
				"the test can occupy the staged name")
			assert.HasError(t, root.Write("a.go", []byte("replacement\n")),
				"a write that cannot stage is reported rather than half done")

			held, err := os.ReadFile(filepath.Join(dir, "a.go"))
			assert.NoError(t, err, "the target is still there")
			assert.Equal(t, string(held), "original\n", "holding what it held")
		})
	})
}
