// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package corpus_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/c"
	"go.dokimi.dev/techne/lang/csharp"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/java"
	"go.dokimi.dev/techne/lang/javascript"
	"go.dokimi.dev/techne/lang/python"
	"go.dokimi.dev/techne/lang/ruby"
	"go.dokimi.dev/techne/lang/rust"
	"go.dokimi.dev/techne/lang/scala"
	"go.dokimi.dev/techne/lang/typescript"
	"go.dokimi.dev/techne/test/corpus"
)

// declarations holds the declaration of each language that techne serves,
// for its name and its extensions.
var declarations = map[string]lang.Declaration{
	string(c.Language):          c.Declaration(),
	string(csharp.Language):     csharp.Declaration(),
	string(golang.Language):     golang.Declaration(),
	string(java.Language):       java.Declaration(),
	string(javascript.Language): javascript.Declaration(),
	string(python.Language):     python.Declaration(),
	string(ruby.Language):       ruby.Declaration(),
	string(rust.Language):       rust.Declaration(),
	string(scala.Language):      scala.Declaration(),
	string(typescript.Language): typescript.Declaration(),
}

func TestManifest(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the manifest of the corpus", func(t *testing.T) {
			t.Parallel()
			m, err := corpus.Load("corpus.json")
			assert.NoError(t, err, "Load of corpus.json")
			assert.NotEmpty(t, m.Repositories, "the repositories of corpus.json")
		})

		t.Run("reads a language that techne declares for each repository", func(t *testing.T) {
			t.Parallel()
			m, err := corpus.Load("corpus.json")
			assert.NoError(t, err, "Load of corpus.json")
			for _, r := range m.Repositories {
				_, declared := declarations[r.Language]
				assert.True(t, declared, "the language of "+r.Name)
			}
		})

		t.Run("reads a repository of each language", func(t *testing.T) {
			t.Parallel()
			m, err := corpus.Load("corpus.json")
			assert.NoError(t, err, "Load of corpus.json")
			for language := range declarations {
				assert.NotEmpty(t, m.Select(language), "the repositories of "+language)
			}
		})

		refused := []struct {
			name   string
			change func(*corpus.Manifest)
		}{
			{
				name:   "returns an error for a manifest without a budget",
				change: func(m *corpus.Manifest) { m.Budget = 0 },
			},
			{
				name:   "returns an error for two repositories with one name",
				change: func(m *corpus.Manifest) { m.Repositories = append(m.Repositories, m.Repositories[0]) },
			},
			{
				name:   "returns an error for a clone without a full commit",
				change: func(m *corpus.Manifest) { m.Repositories[0].Commit = "f78e722" },
			},
			{
				name:   "returns an error for a repository with a URL and a path",
				change: func(m *corpus.Manifest) { m.Repositories[0].Path = "../elsewhere" },
			},
			{
				name:   "returns an error for a clone without a build command",
				change: func(m *corpus.Manifest) { m.Repositories[0].Build = nil },
			},
			{
				name:   "returns an error for an errors pattern without a group",
				change: func(m *corpus.Manifest) { m.Repositories[0].Errors = `\d+ errors` },
			},
			{
				name:   "returns an error for a repository without a warmup",
				change: func(m *corpus.Manifest) { m.Repositories[0].Warmup = 0 },
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				m := valid()
				tt.change(&m)
				_, err := corpus.Load(written(t, m))
				assert.HasError(t, err, "Load of the changed manifest")
			})
		}
	})

	t.Run("Select", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			only string
			want []string
		}{
			{name: "returns every repository for an empty list", only: "", want: []string{"alpha", "beta"}},
			{name: "returns the repository of a name", only: "beta", want: []string{"beta"}},
			{name: "returns the repositories of a language", only: " go ", want: []string{"alpha"}},
			{name: "returns nothing for an unknown name", only: "gamma", want: nil},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				var got []string
				for _, r := range valid().Select(tt.only) {
					got = append(got, r.Name)
				}
				assert.Equal(t, got, tt.want, "the repositories of "+tt.only)
			})
		}
	})

	t.Run("Count", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the number that the pattern captures", func(t *testing.T) {
			t.Parallel()
			count, found := corpus.Repository{Errors: `(\d+) errors?`}.Count([]byte("12 errors, 3 warnings\n"))
			assert.True(t, found, "the match of the pattern")
			assert.Equal(t, count, 12, "the number of errors")
		})

		t.Run("returns the sum of the numbers of every match", func(t *testing.T) {
			t.Parallel()
			count, found := corpus.Repository{Errors: `# fail (\d+)`}.Count([]byte("# fail 0\nok\n# fail 3\n"))
			assert.True(t, found, "the match of the pattern")
			assert.Equal(t, count, 3, "the failures of both suites")
		})

		t.Run("returns false for output that the pattern does not match", func(t *testing.T) {
			t.Parallel()
			_, found := corpus.Repository{Errors: `(\d+) errors?`}.Count([]byte("no problems\n"))
			assert.False(t, found, "the match of the pattern")
		})

		t.Run("returns false for a repository without a pattern", func(t *testing.T) {
			t.Parallel()
			_, found := corpus.Repository{}.Count([]byte("12 errors\n"))
			assert.False(t, found, "the match of no pattern")
		})
	})

	t.Run("Writable", func(t *testing.T) {
		t.Parallel()

		t.Run("returns true for a repository with a URL", func(t *testing.T) {
			t.Parallel()
			assert.True(t, corpus.Repository{URL: "https://example.com/a.git"}.Writable(), "a clone")
		})

		t.Run("returns false for a repository with a path", func(t *testing.T) {
			t.Parallel()
			assert.False(t, corpus.Repository{Path: "../stealth"}.Writable(), "a directory")
		})
	})

	t.Run("Duration", func(t *testing.T) {
		t.Parallel()

		t.Run("reads a duration string", func(t *testing.T) {
			t.Parallel()
			var d corpus.Duration
			assert.NoError(t, json.Unmarshal([]byte(`"1m30s"`), &d), "Unmarshal of 1m30s")
			assert.Equal(t, time.Duration(d), 90*time.Second, "the duration")
		})

		t.Run("returns an error for a number", func(t *testing.T) {
			t.Parallel()
			var d corpus.Duration
			assert.HasError(t, json.Unmarshal([]byte(`90`), &d), "Unmarshal of 90")
		})

		t.Run("writes a duration string", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(corpus.Duration(90 * time.Second))
			assert.NoError(t, err, "Marshal of 90 seconds")
			assert.Equal(t, string(encoded), `"1m30s"`, "the encoding")
		})
	})
}

// valid returns a manifest that Load accepts: a clone of Go and a
// read-only repository of Python.
func valid() corpus.Manifest {
	return corpus.Manifest{
		Budget: corpus.Duration(2 * time.Second),
		Sample: 4,
		Repositories: []corpus.Repository{
			{
				Name: "alpha", Language: "go", URL: "https://example.com/alpha.git",
				Commit: "f78e722310e50bcaca9276be22276d9e91d91308", Build: []string{"go", "build", "./..."},
				Warmup: corpus.Duration(time.Minute),
			},
			{Name: "beta", Language: "python", Path: "../beta", Warmup: corpus.Duration(time.Minute)},
		},
	}
}

// written writes m to a file and returns its path.
func written(t *testing.T, m corpus.Manifest) string {
	t.Helper()
	encoded, err := json.Marshal(m)
	assert.NoError(t, err, "Marshal of the manifest")
	path := filepath.Join(t.TempDir(), "corpus.json")
	assert.NoError(t, os.WriteFile(path, encoded, 0o644), "WriteFile of the manifest")
	return path
}
