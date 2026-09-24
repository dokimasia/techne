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
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// lifting plans the extraction of line 6 of a.fake into a function called name with e.
func lifting(t *testing.T, e *lsp.Engine, name string) (engine.Result[edit.Change], error) {
	t.Helper()
	return e.Plan(t.Context(), engine.Request{Scope: "a.fake"}, edit.ExtractFunction,
		edit.Target{Kind: edit.TargetSpan, Span: source.Span{
			Path: "a.fake", Start: source.Position{Line: 6}, End: source.Position{Line: 6},
		}},
		edit.Args{edit.ArgNewName: name})
}

// written returns the new text of every edit of changes, joined.
func written(changes []edit.Change) string {
	var out strings.Builder
	for _, one := range changes {
		for _, e := range one.Edits {
			out.WriteString(e.New)
		}
	}
	return out.String()
}

func TestExtract(t *testing.T) {
	t.Parallel()

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("names the function as the caller asks", func(t *testing.T) {
			t.Parallel()
			got, err := lifting(t, serving(t, lsptest.Extracts, sample()), "weigh")
			assert.NoError(t, err, "Plan of an extraction")
			assert.Length(t, got.Items, 1, "the changes of the plan")
			assert.Contains(t, written(got.Items), "func weigh() int { return 1 }", "the text the plan writes")
			assert.NotContains(t, written(got.Items), lsptest.Placeholder, "the text the plan writes")
		})

		t.Run("takes the action that the language module names", func(t *testing.T) {
			t.Parallel()
			got, err := lifting(t, serving(t, lsptest.Extracts, sample()), "weigh")
			assert.NoError(t, err, "Plan of an extraction")
			assert.NotContains(t, written(got.Items), "var extracted", "the text the plan writes")
		})

		t.Run("takes the edit of an action that the server performs", func(t *testing.T) {
			t.Parallel()
			got, err := lifting(t, serving(t, lsptest.Commands, sample()), "weigh")
			assert.NoError(t, err, "Plan of an extraction by command")
			assert.Contains(t, written(got.Items), "func weigh()", "the text the plan writes")
		})

		t.Run("restores the content on disk after the extraction", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Extracts, sample())
			_, err := lifting(t, e, "weigh")
			assert.NoError(t, err, "Plan of an extraction")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify after the extraction")
			assert.Equal(t, messages(got.Items), []string{"declares=After"}, "the functions the server holds")
		})

		t.Run("declines a server declared without an extraction", func(t *testing.T) {
			t.Parallel()
			_, err := lifting(t, serving(t, lsptest.Default, sample()), "weigh")
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Plan")
		})

		t.Run("declines a server without code actions", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Thin)
			server.Extracts = lsp.Refactor{Kind: "refactor.extract"}
			_, err := lifting(t, lsptest.Engine(t, lsptest.Workspace(t, sample()), server), "weigh")
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Plan")
			assert.Contains(t, err.Error(), "codeAction", "the error of Plan")
		})

		t.Run("names the lines for which the server offers no extraction", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Compiles)
			server.Extracts = lsp.Refactor{Kind: "refactor.extract"}
			_, err := lifting(t, lsptest.Engine(t, lsptest.Workspace(t, sample()), server), "weigh")
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
			assert.Contains(t, err.Error(), "offers no extraction of lines 7 to 7 of a.fake", "the error of Plan")
		})

		t.Run("refuses an extraction without a name", func(t *testing.T) {
			t.Parallel()
			_, err := lifting(t, serving(t, lsptest.Extracts, sample()), "")
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("refuses lines past the end of the file", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Extracts, sample()).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.ExtractFunction,
				edit.Target{Kind: edit.TargetSpan, Span: source.Span{
					Path: "a.fake", Start: source.Position{Line: 400}, End: source.Position{Line: 900},
				}},
				edit.Args{edit.ArgNewName: "weigh"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("refuses a target that is not a span", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Extracts, sample()).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.ExtractFunction,
				edit.Target{Kind: edit.TargetSymbol, Symbol: "fake::Store"},
				edit.Args{edit.ArgNewName: "weigh"})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Plan")
		})

		t.Run("skips lines in a file of another language", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Extracts, map[string]string{"notes.md": "# notes\n"}).Plan(t.Context(),
				engine.Request{Scope: "notes.md"}, edit.ExtractFunction,
				edit.Target{Kind: edit.TargetSpan, Span: source.Span{Path: "notes.md"}},
				edit.Args{edit.ArgNewName: "weigh"})
			assert.NoError(t, err, "Plan of lines in notes.md")
			assert.True(t, got.Skipped, "Skipped of the plan")
		})
	})
}
