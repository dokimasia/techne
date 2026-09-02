// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/lsp"
)

// lifting asks for lines 7 and 8 of the fixture to be lifted into a
// function called weigh, which is how every case here begins.
func lifting(t *testing.T, e *lsp.Engine, name string) (engine.Result[edit.Change], error) {
	t.Helper()
	return e.Plan(t.Context(), engine.Request{Scope: "a.fake"}, edit.ExtractFunction,
		edit.Target{Kind: edit.TargetSpan, Span: source.Span{
			Path:  "a.fake",
			Start: source.Position{Line: 6},
			End:   source.Position{Line: 6},
		}},
		edit.Args{edit.ArgNewName: name})
}

func TestExtract(t *testing.T) {
	t.Parallel()

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("names the function what the caller asked for", func(t *testing.T) {
			t.Parallel()
			// No server takes a name: in an editor the name is typed into
			// the box that opens after the extraction. So the extraction
			// is computed, shown to the server as an unsaved buffer, and
			// what appeared in it is renamed.
			e := serving(t, modeExtracts, map[string]string{"a.fake": content})
			got, err := lifting(t, e, "weigh")

			assert.NoError(t, err, "lifting lines into a function succeeds")
			assert.Length(t, got.Items, 1, "one file changes")
			assert.Contains(t, written(got.Items), "func weigh()",
				"the function carries the name that was asked for")
			assert.NotContains(t, written(got.Items), placeholder,
				"and not the one the server gave it")
		})

		t.Run("takes the action the language module named", func(t *testing.T) {
			t.Parallel()
			// Every server offers several extractions under one kind and
			// none of them marks one preferred. Taking whichever came
			// first would extract a variable when a function was asked
			// for.
			e := serving(t, modeExtracts, map[string]string{"a.fake": content})
			got, err := lifting(t, e, "weigh")

			assert.NoError(t, err, "lifting succeeds")
			assert.NotContains(t, written(got.Items), "var extracted",
				"the variable beside it in the menu was not taken")
		})

		t.Run("measures the naming against the text the server was shown", func(t *testing.T) {
			t.Parallel()
			// The rename is computed over the extraction's result, which
			// is not what is on disk. A range converted against the file
			// names different bytes, and writing over them still parses.
			e := serving(t, modeExtracts, map[string]string{"a.fake": content})
			got, err := lifting(t, e, "weigh")

			assert.NoError(t, err, "lifting succeeds")
			assert.Contains(t, written(got.Items), "func weigh() int { return 1 }",
				"the composed result is the extraction with the name in it")
		})

		t.Run("takes an edit a server performs rather than describes", func(t *testing.T) {
			t.Parallel()
			// Some servers expose a refactoring only as a command: they
			// do the work and offer the client the result to apply.
			// techne asked for that edit, so it keeps it — and still
			// gates it rather than letting the server write.
			e := serving(t, modeCommands, map[string]string{"a.fake": content})
			got, err := lifting(t, e, "weigh")

			assert.NoError(t, err, "a refactoring behind a command succeeds")
			assert.Contains(t, written(got.Items), "func weigh()",
				"the edit the server offered became the plan")
		})

		t.Run("leaves the server holding what is on disk", func(t *testing.T) {
			t.Parallel()
			// The buffer shown to the server is not a file, and a plan is
			// not an apply. A server left holding text nobody wrote
			// answers every later question about code that is nowhere.
			e := serving(t, modeExtracts, map[string]string{"a.fake": content})
			_, err := lifting(t, e, "weigh")
			assert.NoError(t, err, "lifting succeeds")

			after, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "asking again succeeds")
			assert.Equal(t, names(after.Items), []string{"After"},
				"the extracted function is not there, because nothing wrote it")
		})

		t.Run("declines where the server offers no such refactoring", func(t *testing.T) {
			t.Parallel()
			// Declared per server because it was established per server:
			// pyright, clangd and metals offer nothing over a run of
			// statements. Declining lets something else answer.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			_, err := lifting(t, e, "weigh")

			assert.ErrorIs(t, err, engine.ErrDecline,
				"a server nobody declared this for is passed over, not failed")
		})

		t.Run("declines where the server answers no code action", func(t *testing.T) {
			t.Parallel()
			held := pretending(modeThin)
			held.Extracts = lsp.Refactor{Kind: "refactor.extract"}
			e := servingAs(t, held, map[string]string{"a.fake": content})
			_, err := lifting(t, e, "weigh")

			assert.ErrorIs(t, err, engine.ErrDecline, "the request is not offered")
			assert.Contains(t, err.Error(), "codeAction", "and the reason names it")
		})

		t.Run("refuses without a name to give the function", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeExtracts, map[string]string{"a.fake": content})
			_, err := lifting(t, e, "")

			assert.ErrorIs(t, err, engine.ErrRefuse, "there is nothing to call it")
		})

		t.Run("refuses a selection past the end of the file", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeExtracts, map[string]string{"a.fake": content})
			_, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
				edit.ExtractFunction,
				edit.Target{Kind: edit.TargetSpan, Span: source.Span{
					Path:  "a.fake",
					Start: source.Position{Line: 400},
					End:   source.Position{Line: 900},
				}},
				edit.Args{edit.ArgNewName: "weigh"})

			assert.ErrorIs(t, err, engine.ErrRefuse, "there are no such lines to lift")
		})

		t.Run("refuses a target that is not a run of lines", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeExtracts, map[string]string{"a.fake": content})
			_, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
				edit.ExtractFunction,
				edit.Target{Kind: edit.TargetSymbol, Symbol: "fake::Store"},
				edit.Args{edit.ArgNewName: "weigh"})

			assert.ErrorIs(t, err, engine.ErrRefuse, "an extraction is pointed at lines")
		})
	})
}

// written is the text a plan's edits put into the files, joined so a
// case can say what is in it without walking the shape.
func written(held []edit.Change) string {
	var out strings.Builder
	for _, one := range held {
		for _, e := range one.Edits {
			out.WriteString(e.New)
		}
	}
	return out.String()
}
