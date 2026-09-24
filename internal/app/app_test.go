// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/internal/app"
	"go.dokimi.dev/techne/lang"
)

// inMemory is the files of a workspace in memory, which the write path writes to.
type inMemory struct{ files fstest.MapFS }

func (m inMemory) Read(p source.Path) ([]byte, error) {
	one, there := m.files[string(p)]
	if !there {
		return nil, fs.ErrNotExist
	}
	return one.Data, nil
}

func (m inMemory) Write(p source.Path, content []byte) error {
	m.files[string(p)] = &fstest.MapFile{Data: content}
	return nil
}

func (m inMemory) Remove(p source.Path) error {
	delete(m.files, string(p))
	return nil
}

func (m inMemory) Move(from, to source.Path) error {
	one, there := m.files[string(from)]
	if !there {
		return fs.ErrNotExist
	}
	m.files[string(to)] = one
	delete(m.files, string(from))
	return nil
}

func (inMemory) Lock(context.Context) (func(), error) { return func() {}, nil }

// modules are the languages of the ten language modules, sorted.
var modules = []source.Language{
	"c", "csharp", "go", "java", "javascript", "python", "ruby", "rust", "scala", "typescript",
}

// workspace returns a file of each of five languages and a file of no language.
func workspace() fstest.MapFS {
	return fstest.MapFS{
		"src/service.go":   {Data: []byte("package src\n\nfunc New() int { return 1 }\n")},
		"src/service.py":   {Data: []byte("def new():\n    return 1\n")},
		"src/Service.java": {Data: []byte("class Service { int get() { return 1; } }\n")},
		"src/service.rs":   {Data: []byte("pub fn new() -> i32 { 1 }\n")},
		"src/service.ts":   {Data: []byte("export function make(): number { return 1 }\n")},
		"README.md":        {Data: []byte("# notes\n")},
	}
}

// built returns the server of files with the write tools and the mock languages of mocks, and
// closes it when the test ends.
func built(t *testing.T, files fstest.MapFS, mocks string) *app.Server {
	t.Helper()
	s, err := app.Build(lang.Workspace{FS: files}, inMemory{files: files}, mocks)
	assert.NoError(t, err, "the error of Build")
	t.Cleanup(func() { _ = s.Close(context.WithoutCancel(t.Context())) })
	return s
}

// named returns the names of the tools of s, in the order of the registry.
func named(s *app.Server) []string {
	var out []string
	for _, one := range s.Tools.Tools() {
		out = append(out, one.Name())
	}
	return out
}

// languages returns the languages of s, sorted.
func languages(s *app.Server) []source.Language {
	out := slices.Clone(s.Languages)
	slices.Sort(out)
	return out
}

// call returns the payload of a call of the tool name of s with input, decoded as an object.
func call(t *testing.T, s *app.Server, name, input string) map[string]any {
	t.Helper()
	for _, candidate := range s.Tools.Tools() {
		if candidate.Name() != name {
			continue
		}
		got, err := candidate.Execute(t.Context(), json.RawMessage(input))
		assert.NoError(t, err, "the error of Execute of "+name)
		var decoded map[string]any
		assert.NoError(t, json.Unmarshal(got.Payload, &decoded), "the payload of "+name)
		return decoded
	}
	t.Fatalf("no tool is named %q", name)
	return nil
}

func TestApp(t *testing.T) {
	t.Parallel()

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("registers the ten languages of the binary", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, languages(built(t, workspace(), "")), modules, "the languages of the server")
		})

		t.Run("registers the mock languages of a specification", func(t *testing.T) {
			t.Parallel()
			want := append(slices.Clone(modules), "alpha", "beta")
			slices.Sort(want)
			assert.Equal(t, languages(built(t, workspace(), "alpha, beta")), want, "the languages of the server")
		})

		t.Run("registers the language mock for the specification 1", func(t *testing.T) {
			t.Parallel()
			got := built(t, workspace(), "1").Languages
			assert.True(t, slices.Contains(got, source.Language("mock")), "the languages of the server")
		})

		t.Run("registers no mock language for the specification 0", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, languages(built(t, workspace(), "0")), modules, "the languages of the server")
		})

		t.Run("registers a mock language at the tiers of a specification", func(t *testing.T) {
			t.Parallel()
			files := fstest.MapFS{"a.alpha": {Data: []byte("type Store\n")}}
			got := call(t, built(t, files, "alpha@indexed/partial"), "outline", `{"scope":"a.alpha"}`)
			provenance := got["provenance"].(map[string]any)
			assert.Equal(t, provenance["fidelity"], any("indexed"), "the fidelity of the outline")
			assert.Equal(t, provenance["completeness"], any("partial"), "the completeness of the outline")
		})

		t.Run("returns an error for a fidelity that trust does not declare", func(t *testing.T) {
			t.Parallel()
			_, err := app.Build(lang.Workspace{FS: fstest.MapFS{}}, nil, "alpha@resolvd")
			assert.HasError(t, err, "the error of Build")
			assert.Equal(t, err.Error(), `app: the mock language alpha does not take the fidelity "resolvd". `+
				"It takes none, syntactic, indexed, resolved", "the error of Build")
		})

		t.Run("returns an error for a completeness that trust does not declare", func(t *testing.T) {
			t.Parallel()
			_, err := app.Build(lang.Workspace{FS: fstest.MapFS{}}, nil, "alpha@resolved/whole")
			assert.HasError(t, err, "the error of Build")
			assert.Equal(t, err.Error(), `app: the mock language alpha does not take the completeness "whole". `+
				"It takes unknown, partial, total", "the error of Build")
		})

		t.Run("offers the read tools without files", func(t *testing.T) {
			t.Parallel()
			s, err := app.Build(lang.Workspace{FS: workspace()}, nil, "")
			assert.NoError(t, err, "the error of Build")
			t.Cleanup(func() { _ = s.Close(context.WithoutCancel(t.Context())) })
			assert.Equal(t, named(s), []string{"outline", "search", "resolve", "relations", "verify", "capabilities"},
				"the tools of the server")
		})

		t.Run("offers the write tools with files", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, named(built(t, workspace(), "")), []string{
				"outline", "search", "resolve", "relations", "verify", "capabilities",
				"document.symbol", "rename.symbol", "move.file", "extract.function", "apply.change",
			}, "the tools of the server")
		})

		t.Run("starts the description of every tool with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			for _, one := range built(t, workspace(), "").Tools.Tools() {
				assert.HasPrefix(t, one.Description(), "PREFER OVER ", "the description of "+one.Name())
			}
		})

		t.Run("outlines a file of each of five languages", func(t *testing.T) {
			t.Parallel()
			s := built(t, workspace(), "")
			for path, language := range map[string]string{
				"src/service.go":   "go",
				"src/service.py":   "python",
				"src/Service.java": "java",
				"src/service.rs":   "rust",
				"src/service.ts":   "typescript",
			} {
				got := call(t, s, "outline", `{"scope":"`+path+`"}`)
				_, failed := got["error"]
				assert.False(t, failed, "the error of the outline of "+path)
				assert.NotEmpty(t, got["items"], "the items of the outline of "+path)
				assert.Equal(t, got["scope"].(map[string]any)["language"], any(language),
					"the language of the outline of "+path)
			}
		})

		t.Run("returns unsupported for a file of no language", func(t *testing.T) {
			t.Parallel()
			got := call(t, built(t, workspace(), ""), "outline", `{"scope":"README.md"}`)
			assert.Equal(t, got["error"].(map[string]any)["code"], any("unsupported"), "the code of the error")
		})

		t.Run("outlines a Go file at the syntactic tier", func(t *testing.T) {
			t.Parallel()
			got := call(t, built(t, workspace(), ""), "outline", `{"scope":"src/service.go"}`)
			provenance := got["provenance"].(map[string]any)
			assert.Equal(t, provenance["fidelity"], any("syntactic"), "the fidelity of the outline")
			assert.Equal(t, provenance["supportsNegativeClaim"], any(false), "the negative claim of the outline")
		})

		t.Run("searches the workspace for a name", func(t *testing.T) {
			t.Parallel()
			got := call(t, built(t, workspace(), ""), "search", `{"text":"new","scope":"src","language":"go"}`)
			_, failed := got["error"]
			assert.False(t, failed, "the error of the search")
			assert.NotEmpty(t, got["items"], "the items of the search")
		})

		t.Run("returns a single match with its signature", func(t *testing.T) {
			t.Parallel()
			got := call(t, built(t, workspace(), ""), "search", `{"text":"New","scope":"src/service.go"}`)
			items := got["items"].([]any)
			assert.Length(t, items, 1, "the items of the search")
			first := items[0].(map[string]any)
			assert.Equal(t, first["name"], any("New"), "the name of the match")
			assert.NotNil(t, first["line"], "the line of the match")
			assert.NotEmpty(t, first["signature"], "the signature of the match")
		})

		t.Run("reports the capabilities of the ten languages", func(t *testing.T) {
			t.Parallel()
			var got []source.Language
			for _, item := range call(t, built(t, workspace(), ""), "capabilities", `{}`)["items"].([]any) {
				language := source.Language(item.(map[string]any)["language"].(string))
				if !slices.Contains(got, language) {
					got = append(got, language)
				}
			}
			slices.Sort(got)
			assert.Equal(t, got, modules, "the languages of the capabilities")
		})

		t.Run("reports the outline of Go as available at the syntactic tier", func(t *testing.T) {
			t.Parallel()
			items := call(t, built(t, workspace(), ""), "capabilities", `{"language":"go"}`)["items"].([]any)
			assert.NotEmpty(t, items, "the capabilities of Go")
			first := items[0].(map[string]any)
			assert.Equal(t, first["role"], any("outline"), "the role of the first capability")
			assert.Equal(t, first["available"], any(true), "Available of the first capability")
			assert.Equal(t, first["fidelity"], any("syntactic"), "the fidelity of the first capability")
		})
	})

	t.Run("Close", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a server that served no call", func(t *testing.T) {
			t.Parallel()
			s, err := app.Build(lang.Workspace{FS: workspace()}, nil, "")
			assert.NoError(t, err, "the error of Build")
			assert.NoError(t, s.Close(t.Context()), "the error of Close")
		})
	})

	t.Run("Root", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves a symbolic link in the root", func(t *testing.T) {
			t.Parallel()
			target := t.TempDir()
			link := filepath.Join(t.TempDir(), "link")
			assert.NoError(t, os.Symlink(target, link), "the error of Symlink")
			want, err := filepath.EvalSymlinks(target)
			assert.NoError(t, err, "the error of EvalSymlinks")
			got, err := app.Root(link)
			assert.NoError(t, err, "the error of Root")
			assert.Equal(t, got, want, "the root of the link")
		})

		t.Run("returns the working directory for the empty root", func(t *testing.T) {
			t.Parallel()
			working, err := os.Getwd()
			assert.NoError(t, err, "the error of Getwd")
			want, err := filepath.EvalSymlinks(working)
			assert.NoError(t, err, "the error of EvalSymlinks")
			got, err := app.Root("")
			assert.NoError(t, err, "the error of Root")
			assert.Equal(t, got, want, "the root of the empty path")
		})

		t.Run("resolves a relative root against the working directory", func(t *testing.T) {
			t.Parallel()
			working, err := os.Getwd()
			assert.NoError(t, err, "the error of Getwd")
			want, err := filepath.EvalSymlinks(filepath.Dir(working))
			assert.NoError(t, err, "the error of EvalSymlinks")
			got, err := app.Root("..")
			assert.NoError(t, err, "the error of Root")
			assert.Equal(t, got, want, "the root of ..")
		})

		t.Run("returns an error for a root that does not exist", func(t *testing.T) {
			t.Parallel()
			_, err := app.Root(filepath.Join(t.TempDir(), "nowhere"))
			assert.HasError(t, err, "the error of Root")
			assert.HasPrefix(t, err.Error(), "app: the workspace root ", "the error of Root")
		})
	})
}
