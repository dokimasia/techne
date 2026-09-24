// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// gitignores are the .gitignore files of the fixture, keyed by directory.
// Each line exercises one rule of gitignore(5).
var gitignores = map[string]string{
	".": strings.Join([]string{
		"# a comment",
		"*.log",
		"!keep.log",
		"/rooted.txt",
		"build/",
		"docs/**",
		"!docs/keep.md",
		"out/**/gen",
		"[!a]bc.txt",
		`\#hash.txt`,
		`\!bang.txt`,
		"trailing.txt   ",
		"lib/*.o",
		"a/**/b.txt",
		"excluded/",
		"!excluded/keep.txt",
		"**/cache",
	}, "\n") + "\n",
	"sub": "!app.log\nlocal.txt\n",
}

// sample are the files of the fixture workspace.
var sample = []string{
	"app.log", "keep.log", "sub/app.log", "sub/keep.log",
	"rooted.txt", "sub/rooted.txt",
	"build/x.txt", "sub/build/y.txt", "build.txt",
	"docs/a.md", "docs/keep.md", "docs/deep/b.md",
	"out/gen", "out/x/gen", "out/x/y/gen/z.txt",
	"abc.txt", "xbc.txt", "#hash.txt", "!bang.txt", "trailing.txt",
	"lib/a.o", "lib/sub/a.o",
	"a/b.txt", "a/x/b.txt", "a/x/y/b.txt",
	"excluded/keep.txt", "excluded/other.txt",
	"cache/x.txt", "deep/cache/y.txt",
	"sub/local.txt", "local.txt", "plain.txt",
}

// written writes the fixture workspace to a new directory and returns it.
func written(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for dir, content := range gitignores {
		assert.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755), "mkdir "+dir)
		assert.NoError(t, os.WriteFile(filepath.Join(root, dir, ".gitignore"), []byte(content), 0o600), dir)
	}
	for _, name := range sample {
		p := filepath.Join(root, filepath.FromSlash(name))
		assert.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755), "mkdir "+name)
		assert.NoError(t, os.WriteFile(p, nil, 0o600), name)
	}
	return root
}

// gitIgnored returns the files of root that git ignores. It runs git without
// the global and system configuration, so only the .gitignore files of root
// apply.
func gitIgnored(t *testing.T, root string) map[string]bool {
	t.Helper()
	home := t.TempDir()
	git := func(args ...string) string {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"HOME="+home, "XDG_CONFIG_HOME="+home)
		out, err := cmd.Output()
		assert.NoError(t, err, "git "+strings.Join(args, " "))
		return string(out)
	}
	git("init", "--quiet")
	assert.NoError(t, os.RemoveAll(filepath.Join(root, ".git", "info", "exclude")), "remove info/exclude")

	ignored := map[string]bool{}
	for name := range strings.SplitSeq(git("ls-files", "--others", "--ignored", "--exclude-standard", "-z"), "\x00") {
		if name != "" {
			ignored[name] = true
		}
	}
	assert.NotEmpty(t, ignored, "files git ignores")
	return ignored
}

func TestIgnored(t *testing.T) {
	t.Parallel()

	root := written(t)
	ignored := gitIgnored(t, root)
	fsys := os.DirFS(root)

	t.Run("Readable", func(t *testing.T) {
		t.Parallel()

		t.Run("returns GeneratedError for the files git ignores", func(t *testing.T) {
			t.Parallel()
			for _, name := range sample {
				_, generated := errors.AsType[lang.GeneratedError](lang.Readable(fsys, source.Path(name)))
				assert.Equal(t, generated, ignored[name], name)
			}
		})
	})

	t.Run("Walk", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the files git does not ignore", func(t *testing.T) {
			t.Parallel()
			extensions := []string{".log", ".txt", ".md", ".o"}
			var want []source.Path
			for _, name := range sample {
				if !ignored[name] && lang.Claims(name, extensions) {
					want = append(want, source.Path(name))
				}
			}
			slices.Sort(want)
			got, err := lang.Walk(fsys, ".", extensions)
			assert.NoError(t, err, "Walk")
			assert.Equal(t, got.Read, want, "Read")
		})

		t.Run("returns GeneratedError for a scope the .gitignore excludes", func(t *testing.T) {
			t.Parallel()
			_, err := lang.Walk(fsys, "build", []string{".txt"})
			generated, ok := errors.AsType[lang.GeneratedError](err)
			assert.True(t, ok, "GeneratedError")
			assert.Equal(t, generated.Scope, source.Path("build"), "Scope")
		})

		t.Run("walks the root under a .gitignore that excludes everything", func(t *testing.T) {
			t.Parallel()
			allow := fstest.MapFS{
				".gitignore": {Data: []byte("*\n!*.fx\n")},
				"a.fx":       {Data: []byte("a")},
				"notes.txt":  {Data: []byte("n")},
				"pkg/b.fx":   {Data: []byte("b")},
			}
			got, err := lang.Walk(allow, ".", []string{".fx", ".txt"})
			assert.NoError(t, err, "Walk")
			assert.Equal(t, got.Read, []source.Path{"a.fx"}, "Read")
		})

		t.Run("applies the .gitignore above a directory scope", func(t *testing.T) {
			t.Parallel()
			got, err := lang.Walk(fsys, "sub", []string{".log", ".txt"})
			assert.NoError(t, err, "Walk")
			assert.Equal(t, got.Read, []source.Path{"sub/app.log", "sub/keep.log", "sub/rooted.txt"}, "Read")
		})
	})

	t.Run("GeneratedError.Error", func(t *testing.T) {
		t.Parallel()

		t.Run("names the excluded path", func(t *testing.T) {
			t.Parallel()
			got := lang.GeneratedError{Scope: "web/dist"}.Error()
			assert.Equal(t, got, "lang: the .gitignore files of the workspace exclude web/dist", "message")
		})
	})
}
