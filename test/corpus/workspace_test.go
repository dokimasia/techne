// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package corpus_test

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/test/corpus"
)

func TestWorkspace(t *testing.T) {
	t.Parallel()
	origin, head := remote(t)
	repository := corpus.Repository{
		Name: "fixture", Language: "go", URL: "file://" + origin, Commit: head,
		Prepare: [][]string{{"sh", "-c", "echo ran >> prepared.log"}},
		Build:   []string{"echo", "built"},
		Warmup:  corpus.Duration(time.Minute),
	}

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		t.Run("clones a repository at its commit", func(t *testing.T) {
			t.Parallel()
			w := opened(t, repository)
			assert.Equal(t, git(t, w.Root, "rev-parse", "HEAD"), head, "the commit of the clone")
			assert.Equal(t, read(t, w.Root, "a.go"), "package a\n", "the content of a.go")
		})

		t.Run("returns ErrForeign for a directory that a run did not clone", func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			assert.NoError(t, os.Mkdir(filepath.Join(dir, "fixture"), 0o755), "Mkdir of the directory")
			_, err := corpus.Open(t.Context(), dir, "", repository, io.Discard)
			assert.ErrorIs(t, err, corpus.ErrForeign, "Open of a foreign directory")
		})

		t.Run("removes a file that an earlier run left", func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			w, err := corpus.Open(t.Context(), dir, "", repository, io.Discard)
			assert.NoError(t, err, "the first Open")
			writeFile(t, w.Root, "left.go", "package a\n")
			_, err = corpus.Open(t.Context(), dir, "", repository, io.Discard)
			assert.NoError(t, err, "the second Open")
			_, err = os.Stat(filepath.Join(w.Root, "left.go"))
			assert.HasError(t, err, "the Stat of left.go")
		})

		t.Run("opens a repository without a URL at its path", func(t *testing.T) {
			t.Parallel()
			base := t.TempDir()
			assert.NoError(t, os.Mkdir(filepath.Join(base, "local"), 0o755), "Mkdir of the local repository")
			local := corpus.Repository{Name: "local", Language: "go", Path: "local"}
			w, err := corpus.Open(t.Context(), t.TempDir(), base, local, io.Discard)
			assert.NoError(t, err, "Open of a local repository")
			assert.Equal(t, w.Root, filepath.Join(base, "local"), "the root of the local repository")
		})
	})

	t.Run("Prepare", func(t *testing.T) {
		t.Parallel()

		t.Run("runs the prepare steps once", func(t *testing.T) {
			t.Parallel()
			w := opened(t, repository)
			assert.NoError(t, w.Prepare(t.Context()), "the first Prepare")
			assert.NoError(t, w.Prepare(t.Context()), "the second Prepare")
			assert.Equal(t, read(t, w.Root, "prepared.log"), "ran\n", "the log of the prepare steps")
		})

		t.Run("keeps the files that the prepare steps leave", func(t *testing.T) {
			t.Parallel()
			w := opened(t, repository)
			assert.NoError(t, w.Prepare(t.Context()), "Prepare")
			assert.NoError(t, w.Reset(t.Context(), nil), "Reset")
			assert.Equal(t, read(t, w.Root, "prepared.log"), "ran\n", "the log of the prepare steps")
		})
	})

	t.Run("Reset", func(t *testing.T) {
		t.Parallel()

		t.Run("restores a changed tracked file", func(t *testing.T) {
			t.Parallel()
			w := opened(t, repository)
			writeFile(t, w.Root, "a.go", "package changed\n")
			assert.NoError(t, w.Reset(t.Context(), nil), "Reset")
			assert.Equal(t, read(t, w.Root, "a.go"), "package a\n", "the content of a.go")
		})

		t.Run("removes an untracked file that keep does not list", func(t *testing.T) {
			t.Parallel()
			w := opened(t, repository)
			writeFile(t, w.Root, "made.go", "package a\n")
			assert.NoError(t, w.Reset(t.Context(), nil), "Reset")
			_, err := os.Stat(filepath.Join(w.Root, "made.go"))
			assert.HasError(t, err, "the Stat of made.go")
		})

		t.Run("keeps an untracked file that keep lists", func(t *testing.T) {
			t.Parallel()
			w := opened(t, repository)
			writeFile(t, w.Root, "kept.go", "package a\n")
			assert.NoError(t, w.Reset(t.Context(), map[string]bool{"kept.go": true}), "Reset")
			assert.Equal(t, read(t, w.Root, "kept.go"), "package a\n", "the content of kept.go")
		})

		t.Run("keeps an ignored file", func(t *testing.T) {
			t.Parallel()
			w := opened(t, repository)
			writeFile(t, w.Root, "ignored/cache.txt", "cached\n")
			assert.NoError(t, w.Reset(t.Context(), nil), "Reset")
			assert.Equal(t, read(t, w.Root, "ignored/cache.txt"), "cached\n", "the content of the ignored file")
		})

		t.Run("returns ErrReadOnly for a repository without a URL", func(t *testing.T) {
			t.Parallel()
			w := &corpus.Workspace{Repository: corpus.Repository{Name: "local", Path: "local"}, Root: t.TempDir()}
			assert.ErrorIs(t, w.Reset(t.Context(), nil), corpus.ErrReadOnly, "Reset of a local repository")
		})
	})

	t.Run("Files", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the tracked files of the extensions", func(t *testing.T) {
			t.Parallel()
			w := opened(t, repository)
			got, err := w.Files(t.Context(), []string{".go"})
			assert.NoError(t, err, "Files")
			assert.Equal(t, got, []string{"a.go", "pkg/b.go"}, "the Go files")
		})

		t.Run("leaves out an excluded prefix", func(t *testing.T) {
			t.Parallel()
			excluding := repository
			excluding.Exclude = []string{"pkg/"}
			w := opened(t, excluding)
			got, err := w.Files(t.Context(), []string{".go"})
			assert.NoError(t, err, "Files")
			assert.Equal(t, got, []string{"a.go"}, "the Go files outside pkg")
		})
	})

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the output of the build command", func(t *testing.T) {
			t.Parallel()
			w := opened(t, repository)
			out, err := w.Build(t.Context())
			assert.NoError(t, err, "Build")
			assert.Equal(t, string(out), "built\n", "the output of the build")
		})

		t.Run("returns an error with the output of a failed command", func(t *testing.T) {
			t.Parallel()
			failing := repository
			failing.Build = []string{"sh", "-c", "echo broken; exit 3"}
			_, err := opened(t, failing).Build(t.Context())
			assert.HasError(t, err, "Build of a failing command")
			assert.Contains(t, err.Error(), "broken", "the error of the build")
		})
	})

	t.Run("Environ", func(t *testing.T) {
		t.Parallel()

		t.Run("replaces the placeholders of a variable", func(t *testing.T) {
			t.Parallel()
			w := &corpus.Workspace{
				Repository: corpus.Repository{Env: map[string]string{"TOOLS": "{tools}/bin:{clone}"}},
				Root:       "/clone", Tools: "/tools",
			}
			assert.Contains(t, w.Environ(), "TOOLS=/tools/bin:/clone", "the environment")
		})
	})
}

// remote makes a repository with two commits under a temporary directory,
// which a clone can fetch by commit, and returns its path and the first
// commit.
func remote(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "--quiet")
	git(t, dir, "config", "uploadpack.allowAnySHA1InWant", "true")
	writeFile(t, dir, "a.go", "package a\n")
	writeFile(t, dir, "pkg/b.go", "package pkg\n")
	writeFile(t, dir, "notes.md", "notes\n")
	writeFile(t, dir, ".gitignore", "ignored/\nprepared.log\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "--quiet", "-m", "first")
	first := git(t, dir, "rev-parse", "HEAD")
	writeFile(t, dir, "a.go", "package second\n")
	git(t, dir, "commit", "--quiet", "-am", "second")
	return dir, first
}

// opened opens r under a new directory.
func opened(t *testing.T, r corpus.Repository) *corpus.Workspace {
	t.Helper()
	w, err := corpus.Open(t.Context(), t.TempDir(), "", r, io.Discard)
	assert.NoError(t, err, "Open of "+r.Name)
	return w
}

// git runs git in dir without the global and system configuration, with an
// author, and returns its output without the trailing newline.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=corpus", "GIT_AUTHOR_EMAIL=corpus@example.com",
		"GIT_COMMITTER_NAME=corpus", "GIT_COMMITTER_EMAIL=corpus@example.com")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "git "+strings.Join(args, " ")+": "+string(out))
	return strings.TrimSpace(string(out))
}

// writeFile writes content to the file at name under dir, with its
// directories.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755), "MkdirAll of "+name)
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o644), "WriteFile of "+name)
}

// read returns the content of the file at name under dir.
func read(t *testing.T, dir, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	assert.NoError(t, err, "ReadFile of "+name)
	return string(content)
}
