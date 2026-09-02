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

func TestRoot(t *testing.T) {
	t.Parallel()

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a directory that is not there", func(t *testing.T) {
			t.Parallel()
			_, err := files.Open(filepath.Join(t.TempDir(), "nowhere"))
			assert.HasError(t, err, "a workspace that does not exist is a mistake in the invocation")
		})
	})

	t.Run("Read", func(t *testing.T) {
		t.Parallel()

		t.Run("returns what is there", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			written(t, dir, "a.go", "package a\n", 0o644)

			got, err := root.Read("a.go")
			assert.NoError(t, err, "reading a file that exists succeeds")
			assert.Equal(t, string(got), "package a\n", "the bytes are the file's own")
		})

		t.Run("refuses a path that climbs out of the root", func(t *testing.T) {
			t.Parallel()
			// The check is the operating system's rather than one
			// written here, which would have to be right every time.
			root, _ := opened(t)
			_, err := root.Read("../escaped.go")
			assert.HasError(t, err, "a path leaving the workspace is refused before it is opened")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("replaces the content", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			written(t, dir, "a.go", "old\n", 0o644)

			assert.NoError(t, root.Write("a.go", []byte("new\n")), "writing a file that exists succeeds")
			assert.Equal(t, read(t, dir, "a.go"), "new\n", "the file holds what was written")
		})

		t.Run("keeps the mode the file had", func(t *testing.T) {
			t.Parallel()
			// A file that was executable stays executable. Rewriting it
			// at the default would break whatever ran it.
			root, dir := opened(t)
			written(t, dir, "run.sh", "#!/bin/sh\n", 0o755)

			assert.NoError(t, root.Write("run.sh", []byte("#!/bin/sh\ntrue\n")), "writing succeeds")
			assert.Equal(t, mode(t, dir, "run.sh"), fs.FileMode(0o755),
				"a script rewritten at 0644 is a script nothing can run")
		})

		t.Run("makes the directories a new file needs", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			assert.NoError(t, root.Write("deep/nested/a.go", []byte("package a\n")),
				"a change that makes a file in a new package is a change, not a failure")
			assert.Equal(t, read(t, dir, "deep/nested/a.go"), "package a\n", "and the file is there")
		})

		t.Run("leaves nothing staged behind", func(t *testing.T) {
			t.Parallel()
			// A staged file left in the tree would make the next write
			// fail on a name it does not own, and would sit in the
			// workspace looking like source.
			root, dir := opened(t)
			assert.NoError(t, root.Write("a.go", []byte("package a\n")), "writing succeeds")

			entries, err := os.ReadDir(dir)
			assert.NoError(t, err, "the workspace can be listed")
			for _, one := range entries {
				assert.NotContains(t, one.Name(), files.Partial,
					"a finished write leaves the file it wrote and nothing else")
			}
		})

		t.Run("refuses a path that climbs out of the root", func(t *testing.T) {
			t.Parallel()
			root, _ := opened(t)
			assert.HasError(t, root.Write("../escaped.go", []byte("x")),
				"a write leaving the workspace is refused rather than performed")
		})
	})

	t.Run("Remove", func(t *testing.T) {
		t.Parallel()

		t.Run("takes a file away", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			written(t, dir, "a.go", "package a\n", 0o644)

			assert.NoError(t, root.Remove("a.go"), "removing a file that exists succeeds")
			_, err := os.Stat(filepath.Join(dir, "a.go"))
			assert.True(t, os.IsNotExist(err), "the file is gone")
		})

		t.Run("is content with a file that is already gone", func(t *testing.T) {
			t.Parallel()
			// The workspace is in the state the caller asked for, and a
			// change retried after a partial one should not fail on the
			// half that already happened.
			root, _ := opened(t)
			assert.NoError(t, root.Remove("never-existed.go"),
				"asking for a state the workspace is already in is not a failure")
		})
	})

	t.Run("FS", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the same directory that is written", func(t *testing.T) {
			t.Parallel()
			// An engine reads through this and the write path writes
			// through the other half. Two roots would let a plan be
			// computed against one tree and applied to another.
			root, _ := opened(t)
			assert.NoError(t, root.Write("a.go", []byte("package a\n")), "writing succeeds")

			got, err := fs.ReadFile(root.FS(), "a.go")
			assert.NoError(t, err, "what was written is readable")
			assert.Equal(t, string(got), "package a\n", "and is the same bytes")
		})
	})
}

// opened returns a root over a fresh directory, and the directory.
func opened(t *testing.T) (*files.Root, string) {
	t.Helper()
	dir := t.TempDir()
	root, err := files.Open(dir)
	assert.NoError(t, err, "opening a directory that exists succeeds")
	t.Cleanup(func() { _ = root.Close() })
	return root, dir
}

// written puts a file in the directory behind the root's back, so a
// test can set up what the root then reads.
func written(t *testing.T, dir, name, content string, perm fs.FileMode) {
	t.Helper()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), perm),
		"the test can prepare the workspace it is about to read")
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dir, name))
	assert.NoError(t, err, "the file the root wrote is on disk")
	return string(got)
}

func mode(t *testing.T, dir, name string) fs.FileMode {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, name))
	assert.NoError(t, err, "the file the root wrote is on disk")
	return info.Mode().Perm()
}
