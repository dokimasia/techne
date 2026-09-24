// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package files_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/service/workspace/files"
)

// The variables that make a process of the test binary run one part of a test.
const (
	// part names the part that the process runs.
	part = "TECHNE_FILES_PART"
	// workspaceVar is the directory of the workspace of the part.
	workspaceVar = "TECHNE_FILES_WORKSPACE"
)

// TestMain runs the part of a test that the environment names, in a process that a test
// started, and the tests otherwise.
func TestMain(m *testing.M) {
	which := os.Getenv(part)
	if which == "" {
		os.Exit(m.Run())
	}
	if err := running(which); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

// running runs the part which over the workspace of the environment.
func running(which string) error {
	root, err := files.Open(os.Getenv(workspaceVar))
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	switch which {
	case "rewrite":
		return rewriting(root)
	case "append":
		return appending(root)
	case "hold":
		return holding(root)
	case "wait":
		return waiting(root)
	}
	return fmt.Errorf("no part is named %s", which)
}

// rewriting rewrites exec.sh and plain.txt and creates new.txt.
func rewriting(root *files.Root) error {
	return errors.Join(
		root.Write("exec.sh", []byte("#!/bin/sh\ntrue\n")),
		root.Write("plain.txt", []byte("two\n")),
		root.Write("new.txt", []byte("new\n")),
	)
}

// child returns the command that runs the part which of a test in a process of the test
// binary, over the workspace dir, with the user cache directory cache and the variables of
// env. A umask other than the empty string runs the process under that umask, through sh.
func child(t *testing.T, cache, which, dir, umask string, env ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$")
	if umask != "" {
		cmd = exec.CommandContext(t.Context(), "sh", "-c", "umask "+umask+" && exec \"$0\" -test.run='^$'", os.Args[0])
	}
	cmd.Env = append(os.Environ(),
		part+"="+which, workspaceVar+"="+dir,
		"XDG_CACHE_HOME="+cache, "HOME="+cache, "LocalAppData="+cache)
	cmd.Env = append(cmd.Env, env...)
	return cmd
}

func TestRoot(t *testing.T) {
	t.Parallel()

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a directory that does not exist", func(t *testing.T) {
			t.Parallel()
			_, err := files.Open(filepath.Join(t.TempDir(), "nowhere"))
			assert.HasError(t, err, "the error of Open")
		})
	})

	t.Run("Read", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the content of a file", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			written(t, dir, "a.go", "package a\n", 0o644)
			got, err := root.Read("a.go")
			assert.NoError(t, err, "Read of a.go")
			assert.Equal(t, string(got), "package a\n", "the content of a.go")
		})

		t.Run("returns an error that wraps ErrNotExist for a path without a file", func(t *testing.T) {
			t.Parallel()
			root, _ := opened(t)
			_, err := root.Read("absent.go")
			assert.ErrorIs(t, err, fs.ErrNotExist, "the error of Read")
		})

		t.Run("refuses a path out of the directory", func(t *testing.T) {
			t.Parallel()
			root, _ := opened(t)
			_, err := root.Read("../escaped.go")
			assert.HasError(t, err, "the error of Read")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Parallel()

		t.Run("replaces the content of a file", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			written(t, dir, "a.go", "old\n", 0o644)
			assert.NoError(t, root.Write("a.go", []byte("new\n")), "Write of a.go")
			assert.Equal(t, read(t, dir, "a.go"), "new\n", "the content of a.go")
		})

		t.Run("keeps the modes of files under the umasks 027 and 077", func(t *testing.T) {
			t.Parallel()
			if runtime.GOOS == "windows" {
				t.Skip("windows has no umask")
			}
			for _, mask := range []fs.FileMode{0o027, 0o077} {
				dir := t.TempDir()
				written(t, dir, "exec.sh", "#!/bin/sh\n", 0o755)
				written(t, dir, "plain.txt", "one\n", 0o644)
				umask := fmt.Sprintf("%03o", mask)
				out, err := child(t, t.TempDir(), "rewrite", dir, umask).CombinedOutput()
				assert.NoError(t, err, "the rewrite under umask "+umask+": "+string(out))
				assert.Equal(t, mode(t, dir, "exec.sh"), fs.FileMode(0o755), "the mode of exec.sh")
				assert.Equal(t, mode(t, dir, "plain.txt"), fs.FileMode(0o644), "the mode of plain.txt")
				assert.Equal(t, mode(t, dir, "new.txt"), 0o644&^mask, "the mode of new.txt")
			}
		})

		t.Run("creates the directories of a new file", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			assert.NoError(t, root.Write("deep/nested/a.go", []byte("package a\n")), "Write of deep/nested/a.go")
			assert.Equal(t, read(t, dir, "deep/nested/a.go"), "package a\n", "the content of deep/nested/a.go")
		})

		t.Run("leaves no staged file", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			assert.NoError(t, root.Write("a.go", []byte("package a\n")), "Write of a.go")
			entries, err := os.ReadDir(dir)
			assert.NoError(t, err, "ReadDir of the workspace")
			for _, one := range entries {
				assert.NotContains(t, one.Name(), files.Partial, "the name of "+one.Name())
			}
		})

		t.Run("writes through a symbolic link inside the directory", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			written(t, dir, "real.go", "old\n", 0o644)
			assert.NoError(t, os.Symlink("real.go", filepath.Join(dir, "link.go")), "Symlink of link.go")
			assert.NoError(t, root.Write("link.go", []byte("new\n")), "Write of link.go")
			assert.Equal(t, read(t, dir, "real.go"), "new\n", "the content of real.go")
			info, err := os.Lstat(filepath.Join(dir, "link.go"))
			assert.NoError(t, err, "Lstat of link.go")
			assert.True(t, info.Mode()&fs.ModeSymlink != 0, "the link at link.go")
		})

		t.Run("refuses a symbolic link out of the directory", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			outside := filepath.Join(t.TempDir(), "outside.go")
			written(t, filepath.Dir(outside), "outside.go", "kept\n", 0o644)
			assert.NoError(t, os.Symlink(outside, filepath.Join(dir, "link.go")), "Symlink of link.go")
			assert.HasError(t, root.Write("link.go", []byte("new\n")), "the error of Write")
			assert.Equal(t, read(t, filepath.Dir(outside), "outside.go"), "kept\n", "the content of outside.go")
		})

		t.Run("refuses a path out of the directory", func(t *testing.T) {
			t.Parallel()
			root, _ := opened(t)
			assert.HasError(t, root.Write("../escaped.go", []byte("x")), "the error of Write")
		})
	})

	t.Run("Move", func(t *testing.T) {
		t.Parallel()

		t.Run("renames a file with its mode", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			written(t, dir, "run.sh", "#!/bin/sh\n", 0o755)
			assert.NoError(t, root.Move("run.sh", "bin/run.sh"), "Move of run.sh")
			assert.Equal(t, read(t, dir, "bin/run.sh"), "#!/bin/sh\n", "the content of bin/run.sh")
			assert.Equal(t, mode(t, dir, "bin/run.sh"), fs.FileMode(0o755), "the mode of bin/run.sh")
			_, err := os.Stat(filepath.Join(dir, "run.sh"))
			assert.ErrorIs(t, err, fs.ErrNotExist, "the file at run.sh")
		})

		t.Run("refuses a destination with a file", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			written(t, dir, "a.go", "a\n", 0o644)
			written(t, dir, "b.go", "b\n", 0o644)
			assert.HasError(t, root.Move("a.go", "b.go"), "the error of Move")
			assert.Equal(t, read(t, dir, "b.go"), "b\n", "the content of b.go")
		})
	})

	t.Run("Remove", func(t *testing.T) {
		t.Parallel()

		t.Run("deletes a file", func(t *testing.T) {
			t.Parallel()
			root, dir := opened(t)
			written(t, dir, "a.go", "package a\n", 0o644)
			assert.NoError(t, root.Remove("a.go"), "Remove of a.go")
			_, err := os.Stat(filepath.Join(dir, "a.go"))
			assert.ErrorIs(t, err, fs.ErrNotExist, "the file at a.go")
		})

		t.Run("returns no error for a path without a file", func(t *testing.T) {
			t.Parallel()
			root, _ := opened(t)
			assert.NoError(t, root.Remove("never-existed.go"), "Remove of never-existed.go")
		})
	})

	t.Run("FS", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the files that Write writes", func(t *testing.T) {
			t.Parallel()
			root, _ := opened(t)
			assert.NoError(t, root.Write("a.go", []byte("package a\n")), "Write of a.go")
			got, err := fs.ReadFile(root.FS(), "a.go")
			assert.NoError(t, err, "ReadFile of a.go")
			assert.Equal(t, string(got), "package a\n", "the content of a.go")
		})
	})
}

// opened returns a root over a new directory, and the directory.
func opened(t *testing.T) (*files.Root, string) {
	t.Helper()
	dir := t.TempDir()
	root, err := files.Open(dir)
	assert.NoError(t, err, "Open of the workspace")
	t.Cleanup(func() { _ = root.Close() })
	return root, dir
}

// written writes a file of the directory with the mode perm, past the root.
func written(t *testing.T, dir, name, content string, perm fs.FileMode) {
	t.Helper()
	at := filepath.Join(dir, name)
	assert.NoError(t, os.WriteFile(at, []byte(content), perm), "WriteFile of "+name)
	assert.NoError(t, os.Chmod(at, perm), "Chmod of "+name)
}

// read returns the content of a file of the directory.
func read(t *testing.T, dir, name string) string {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dir, name))
	assert.NoError(t, err, "ReadFile of "+name)
	return string(got)
}

// mode returns the permission bits of a file of the directory.
func mode(t *testing.T, dir, name string) fs.FileMode {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, name))
	assert.NoError(t, err, "Stat of "+name)
	return info.Mode().Perm()
}
