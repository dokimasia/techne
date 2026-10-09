// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// claimed are the extensions of the fixture language.
var claimed = []string{".fx"}

// tree returns a workspace of claimed and unclaimed files.
func tree() fstest.MapFS {
	return fstest.MapFS{
		"a.fx":         {Data: []byte("a")},
		"a/b.fx":       {Data: []byte("b")},
		"b.txt":        {Data: []byte("b")},
		"pkg/c.fx":     {Data: []byte("c")},
		"pkg/d.other":  {Data: []byte("d")},
		"pkg/sub/e.fx": {Data: []byte("e")},
		"Makefile":     {Data: []byte("m")},
	}
}

// vendoring returns a workspace with dependency trees at two depths.
func vendoring() fstest.MapFS {
	return fstest.MapFS{
		"main.fx":                       {Data: []byte("a")},
		"pkg/own.fx":                    {Data: []byte("b")},
		"node_modules/dep/index.fx":     {Data: []byte("c")},
		"node_modules/dep/deep/one.fx":  {Data: []byte("d")},
		"pkg/node_modules/other/two.fx": {Data: []byte("e")},
		".git/objects/three.fx":         {Data: []byte("f")},
	}
}

// counting is a filesystem that counts the opens of each path.
type counting struct {
	fs.FS
	mu    sync.Mutex
	opens map[string]int
}

func (c *counting) Open(name string) (fs.File, error) {
	c.mu.Lock()
	c.opens[name]++
	c.mu.Unlock()
	return c.FS.Open(name) //nolint:wrapcheck // the errors of the file system it counts
}

// errUnreadable is the error of the directory that [unreadable] cannot read.
var errUnreadable = errors.New("lang_test: the directory cannot be read")

// unreadable is a filesystem whose directory sub cannot be read.
type unreadable struct{ fstest.MapFS }

// ReadDir returns errUnreadable for the directory sub, and the entries of any other directory.
func (u unreadable) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "sub" {
		return nil, errUnreadable
	}
	return u.MapFS.ReadDir(name) //nolint:wrapcheck // the errors of the file system it wraps
}

func TestFiles(t *testing.T) {
	t.Parallel()

	t.Run("Walk", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give source.Path
			want []source.Path
		}{
			{
				name: "returns every claimed file under a directory in byte order",
				give: ".",
				want: []source.Path{"a.fx", "a/b.fx", "pkg/c.fx", "pkg/sub/e.fx"},
			},
			{
				name: "returns the files of the subtree that the scope names",
				give: "pkg/sub",
				want: []source.Path{"pkg/sub/e.fx"},
			},
			{
				name: "returns a file scope that the extensions claim",
				give: "pkg/c.fx",
				want: []source.Path{"pkg/c.fx"},
			},
			{
				name: "returns no files for a file scope with another extension",
				give: "b.txt",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, err := lang.Walk(tree(), tt.give, claimed)
				assert.NoError(t, err, "Walk")
				assert.Equal(t, got.Read, tt.want, "Read")
			})
		}

		t.Run("refuses a scope that does not exist", func(t *testing.T) {
			t.Parallel()
			_, err := lang.Walk(tree(), "nowhere", claimed)
			assert.ErrorIs(t, err, engine.ErrRefuse, "Walk")
			why, _ := engine.Refusal(err)
			assert.Equal(t, why, "nowhere does not exist", "reason")
		})

		t.Run("returns a file larger than Largest in Unread", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{
				"big.fx":   {Data: bytes.Repeat([]byte("x"), lang.Largest+1)},
				"small.fx": {Data: []byte("x")},
			}
			got, err := lang.Walk(fsys, ".", claimed)
			assert.NoError(t, err, "Walk")
			assert.Equal(t, got, lang.Files{Read: []source.Path{"small.fx"}, Unread: []source.Path{"big.fx"}}, "files")
		})

		t.Run("returns the files larger than Largest in byte order", func(t *testing.T) {
			t.Parallel()
			big := bytes.Repeat([]byte("x"), lang.Largest+1)
			fsys := fstest.MapFS{"a.fx": {Data: big}, "a/b.fx": {Data: big}}
			got, err := lang.Walk(fsys, ".", claimed)
			assert.NoError(t, err, "Walk")
			assert.Equal(t, got.Unread, []source.Path{"a.fx", "a/b.fx"}, "Unread")
		})

		t.Run("returns a file of Largest bytes in Read", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"edge.fx": {Data: bytes.Repeat([]byte("x"), lang.Largest)}}
			got, err := lang.Walk(fsys, ".", claimed)
			assert.NoError(t, err, "Walk")
			assert.Equal(t, got.Read, []source.Path{"edge.fx"}, "Read")
		})

		t.Run("measures the target of a symbolic link", func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			target := filepath.Join(dir, "bundle.dat")
			assert.NoError(t, os.WriteFile(target, bytes.Repeat([]byte("x"), lang.Largest+1), 0o600), "write")
			assert.NoError(t, os.Symlink(target, filepath.Join(dir, "link.fx")), "symlink")
			got, err := lang.Walk(os.DirFS(dir), ".", claimed)
			assert.NoError(t, err, "Walk")
			assert.Equal(t, got.Unread, []source.Path{"link.fx"}, "Unread")
		})

		t.Run("skips a directory that Vendored names", func(t *testing.T) {
			t.Parallel()
			got, err := lang.Walk(vendoring(), ".", claimed)
			assert.NoError(t, err, "Walk")
			assert.Equal(t, got.Read, []source.Path{"main.fx", "pkg/own.fx"}, "Read")
		})

		t.Run("walks a vendored directory that the scope names", func(t *testing.T) {
			t.Parallel()
			got, err := lang.Walk(vendoring(), "node_modules", claimed)
			assert.NoError(t, err, "Walk")
			assert.Equal(t, got.Read,
				[]source.Path{"node_modules/dep/deep/one.fx", "node_modules/dep/index.fx"}, "Read")
		})

		t.Run("opens each .gitignore once", func(t *testing.T) {
			t.Parallel()
			fsys := &counting{
				FS: fstest.MapFS{
					".gitignore":        {Data: []byte("*.gen.fx\n")},
					"pkg/.gitignore":    {Data: []byte("tmp/\n")},
					"pkg/a.fx":          {Data: []byte("a")},
					"pkg/sub/b.fx":      {Data: []byte("b")},
					"pkg/sub/c.gen.fx":  {Data: []byte("c")},
					"pkg/sub/deep/d.fx": {Data: []byte("d")},
				},
				opens: map[string]int{},
			}
			_, err := lang.Walk(fsys, "pkg/sub", claimed)
			assert.NoError(t, err, "Walk")
			for name, opens := range fsys.opens {
				if path.Base(name) == ".gitignore" {
					assert.Equal(t, opens, 1, name)
				}
			}
		})
	})

	t.Run("Visit", func(t *testing.T) {
		t.Parallel()

		unclaimed := func(p string) bool { return !lang.Claims(p, claimed) }
		tests := []struct {
			name string
			give source.Path
			want []string
		}{
			{
				name: "calls visit for each file under a directory that wanted selects",
				give: ".",
				want: []string{"Makefile", "b.txt", "pkg/d.other"},
			},
			{name: "calls visit for a file scope that wanted selects", give: "b.txt", want: []string{"b.txt"}},
			{name: "calls visit for no file of a file scope that wanted leaves out", give: "a.fx"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				var got []string
				err := lang.Visit(tree(), tt.give, unclaimed, func(p string, _ fs.DirEntry) { got = append(got, p) })
				assert.NoError(t, err, "Visit")
				assert.Equal(t, got, tt.want, "the visited paths")
			})
		}

		t.Run("leaves out a file that a .gitignore excludes", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{
				".gitignore": {Data: []byte("*.gen.fx\n")},
				"a.fx":       {Data: []byte("a")},
				"b.gen.fx":   {Data: []byte("b")},
			}
			var got []string
			err := lang.Visit(fsys, ".", func(string) bool { return true },
				func(p string, _ fs.DirEntry) { got = append(got, p) })
			assert.NoError(t, err, "Visit")
			assert.Equal(t, got, []string{".gitignore", "a.fx"}, "the visited paths")
		})

		t.Run("refuses a scope that does not exist", func(t *testing.T) {
			t.Parallel()
			err := lang.Visit(tree(), "nowhere", unclaimed, func(string, fs.DirEntry) {})
			assert.ErrorIs(t, err, engine.ErrRefuse, "Visit")
		})

		t.Run("returns the error of a directory that it cannot read", func(t *testing.T) {
			t.Parallel()
			fsys := unreadable{fstest.MapFS{"a.fx": {Data: []byte("a")}, "sub/b.fx": {Data: []byte("b")}}}
			err := lang.Visit(fsys, ".", unclaimed, func(string, fs.DirEntry) {})
			assert.ErrorIs(t, err, errUnreadable, "Visit")
		})
	})

	t.Run("Info", func(t *testing.T) {
		t.Parallel()

		// linked returns the entry of name in a directory with a file of three bytes, a link to
		// it, a subdirectory, a link to the subdirectory and a link to nothing.
		linked := func(t *testing.T, name string) (fs.FS, fs.DirEntry) {
			t.Helper()
			dir := t.TempDir()
			assert.NoError(t, os.WriteFile(filepath.Join(dir, "file.fx"), []byte("abc"), 0o600), "write")
			assert.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700), "mkdir")
			assert.NoError(t, os.Symlink(filepath.Join(dir, "file.fx"), filepath.Join(dir, "to-file.fx")), "symlink")
			assert.NoError(t, os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "to-sub.fx")), "symlink")
			assert.NoError(t, os.Symlink(filepath.Join(dir, "none"), filepath.Join(dir, "to-none.fx")), "symlink")
			fsys := os.DirFS(dir)
			entries, err := fs.ReadDir(fsys, ".")
			assert.NoError(t, err, "ReadDir")
			for _, one := range entries {
				if one.Name() == name {
					return fsys, one
				}
			}
			t.Fatalf("the directory has no entry %s", name)
			return nil, nil
		}

		t.Run("returns the FileInfo of a file", func(t *testing.T) {
			t.Parallel()
			fsys, entry := linked(t, "file.fx")
			info, ok := lang.Info(fsys, "file.fx", entry)
			assert.True(t, ok, "Info of a file")
			assert.Equal(t, info.Size(), int64(3), "the size of the file")
		})

		t.Run("returns the FileInfo of the target of a link", func(t *testing.T) {
			t.Parallel()
			fsys, entry := linked(t, "to-file.fx")
			info, ok := lang.Info(fsys, "to-file.fx", entry)
			assert.True(t, ok, "Info of a link to a file")
			assert.Equal(t, info.Size(), int64(3), "the size of the target")
		})

		t.Run("reports false for a link to a directory", func(t *testing.T) {
			t.Parallel()
			fsys, entry := linked(t, "to-sub.fx")
			_, ok := lang.Info(fsys, "to-sub.fx", entry)
			assert.False(t, ok, "Info of a link to a directory")
		})

		t.Run("reports false for a link to nothing", func(t *testing.T) {
			t.Parallel()
			fsys, entry := linked(t, "to-none.fx")
			_, ok := lang.Info(fsys, "to-none.fx", entry)
			assert.False(t, ok, "Info of a link to nothing")
		})
	})

	t.Run("Claims", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{
				name: "returns true for a claimed extension of a path that does not exist",
				give: "no/such/a.fx",
				want: true,
			},
			{name: "returns false for a path without an extension", give: "Makefile"},
			{name: "returns false for an unclaimed extension", give: "b.txt"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, lang.Claims(tt.give, claimed), tt.want, "Claims")
			})
		}

		t.Run("agrees with Walk on every file scope", func(t *testing.T) {
			t.Parallel()
			fsys := tree()
			for name := range fsys {
				got, err := lang.Walk(fsys, source.Path(name), claimed)
				assert.NoError(t, err, name)
				assert.Equal(t, len(got.Read) == 1, lang.Claims(name, claimed), name)
			}
		})
	})
}
