// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app_test

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/internal/app"
)

// workspace holds one file per language techne ships with, plus one it
// serves for no language.
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

func built(t *testing.T) *app.Server {
	t.Helper()
	s, err := app.Build(workspace())
	assert.NoError(t, err, "every language techne ships with registers into one workspace")
	return s
}

func run(t *testing.T, name, input string) map[string]any {
	t.Helper()
	tools := built(t).Tools.Tools()
	for _, candidate := range tools {
		if candidate.Name() != name {
			continue
		}
		got, err := candidate.Execute(t.Context(), json.RawMessage(input))
		assert.NoError(t, err, "a well-formed call reaches the service")
		var decoded map[string]any
		assert.NoError(t, json.Unmarshal(got.Payload, &decoded), "the answer is JSON a caller can read")
		return decoded
	}
	t.Fatalf("no tool named %q is registered", name)
	return nil
}

func TestApp(t *testing.T) {
	t.Parallel()

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("registers every language techne ships with", func(t *testing.T) {
			t.Parallel()
			assert.Length(t, built(t).Languages, 10,
				"a language is registered by an explicit call, so the set is what this root chose")
		})

		t.Run("offers the read tools", func(t *testing.T) {
			t.Parallel()
			var names []string
			for _, registered := range built(t).Tools.Tools() {
				names = append(names, registered.Name())
			}
			assert.Contains(t, names, "outline", "an agent can ask what a file declares")
			assert.Contains(t, names, "search", "an agent can ask where something is declared")
			assert.Contains(t, names, "capabilities", "an agent can ask what the server can do")
		})

		t.Run("names the built-in each tool replaces", func(t *testing.T) {
			t.Parallel()
			// An agent shown a tool with no reason to prefer it reaches
			// for grep instead, and a tool nothing calls has no cost to
			// measure.
			for _, registered := range built(t).Tools.Tools() {
				assert.HasPrefix(t, registered.Description(), "PREFER OVER ",
					"every description says which built-in it replaces and why")
			}
		})
	})

	t.Run("outline", func(t *testing.T) {
		t.Parallel()

		t.Run("answers about every registered language", func(t *testing.T) {
			t.Parallel()
			for path, want := range map[string]string{
				"src/service.go":   "go",
				"src/service.py":   "python",
				"src/Service.java": "java",
				"src/service.rs":   "rust",
				"src/service.ts":   "typescript",
			} {
				got := run(t, "outline", `{"scope":"`+path+`"}`)
				assert.Equal(t, got["status"], "ok", "a file of a registered language is answered")
				items, ok := got["items"].([]any)
				assert.True(t, ok, "an answer carries its items")
				assert.NotEmpty(t, items, "a file declaring something outlines to something")
				first := items[0].(map[string]any)
				assert.Equal(t, first["language"].(string), want,
					"the answer names the language that served it")
			}
		})

		t.Run("says a file it serves no language for is unsupported", func(t *testing.T) {
			t.Parallel()
			// A capability gap is something a caller routes around.
			got := run(t, "outline", `{"scope":"README.md"}`)
			assert.Equal(t, got["status"], "unsupported", "no language claims a markdown file")
		})

		t.Run("never claims a parser proves absence", func(t *testing.T) {
			t.Parallel()
			got := run(t, "outline", `{"scope":"src/service.go"}`)
			provenance := got["provenance"].(map[string]any)
			assert.Equal(t, provenance["fidelity"], "syntactic", "only a parser is registered so far")
			assert.Equal(t, provenance["supportsNegativeClaim"], false,
				"a name matched across files is coincidence, so an empty answer proves nothing")
		})
	})

	t.Run("search", func(t *testing.T) {
		t.Parallel()

		t.Run("finds a declaration across the workspace", func(t *testing.T) {
			t.Parallel()
			got := run(t, "search", `{"text":"new","scope":"src","language":"go"}`)
			assert.Equal(t, got["status"], "ok", "a search over a registered language is answered")
			assert.NotEmpty(t, got["items"], "the workspace declares something by that name")
		})

		t.Run("returns one match whole, with no second call", func(t *testing.T) {
			t.Parallel()
			got := run(t, "search", `{"text":"New","scope":"src/service.go"}`)
			items := got["items"].([]any)
			assert.Length(t, items, 1, "one declaration matched")
			first := items[0].(map[string]any)
			assert.Equal(t, first["name"].(string), "New", "the declaration searched for comes back")
			assert.NotEmpty(t, first["span"], "a single match carries where it lives")
		})
	})

	t.Run("capabilities", func(t *testing.T) {
		t.Parallel()

		t.Run("reports every language, so an agent need not guess", func(t *testing.T) {
			t.Parallel()
			raw := run(t, "capabilities", `{}`)
			items, ok := raw["items"].([]any)
			assert.True(t, ok, "the report carries its items")

			seen := map[string]bool{}
			for _, item := range items {
				seen[item.(map[string]any)["language"].(string)] = true
			}
			for _, language := range []string{"go", "python", "java", "rust", "typescript"} {
				assert.True(t, seen[language],
					"a caller asks the server what it serves rather than inferring it from tool names")
			}
		})

		t.Run("reports the outline role as available", func(t *testing.T) {
			t.Parallel()
			raw := run(t, "capabilities", `{"language":"go"}`)
			items := raw["items"].([]any)
			assert.NotEmpty(t, items, "Go is served")
			first := items[0].(map[string]any)
			assert.Equal(t, first["role"], "outline", "the parser serves the outline role")
			assert.Equal(t, first["available"], true, "an in-process parser can always run")
			assert.False(t, strings.Contains(first["fidelity"].(string), "resolved"),
				"no type checker is registered yet, so nothing claims resolved evidence")
		})
	})
}
