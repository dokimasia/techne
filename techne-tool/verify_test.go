// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/tool"
)

// gating returns [addressable] with two findings: a warning with one obvious fix, and an error
// without one.
func gating() *reads {
	over := addressable()
	over.found = []edit.Finding{
		{
			Diagnostic: diag.Diagnostic{
				Severity: diag.SeverityWarning, Code: "stringsseq", Source: "modernize",
				Message: "prefer FieldsSeq",
				Span:    source.Span{Path: "a.fx", Start: source.Position{Line: 227}},
				Snippet: "for part := range strings.Fields(x) {",
			},
			Fix: []edit.Change{{
				Kind: edit.ChangeEdit, Path: "a.fx",
				Edits: []edit.TextEdit{{New: "for part := range strings.FieldsSeq(x) {"}},
			}},
		},
		{
			Diagnostic: diag.Diagnostic{
				Severity: diag.SeverityError, Message: "undefined: Missing",
				Span: source.Span{Path: "a.fx", Start: source.Position{Line: 300}},
			},
		},
	}
	return over
}

// verified runs the verify tool over [gating] with input, and decodes the output.
func verified(t *testing.T, input string) tool.VerifyOutput {
	t.Helper()
	built, err := tool.Verify(gating())
	assert.NoError(t, err, "the error of Verify")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.VerifyOutput
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out
}

func TestVerify(t *testing.T) {
	t.Parallel()

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("starts its description with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Verify(gating())
			assert.NoError(t, err, "the error of Verify")
			assert.HasPrefix(t, built.Description(), "PREFER OVER ", "the description")
		})

		t.Run("returns each issue with its severity and its message", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Length(t, got.Items, 2, "the issues")
			assert.Equal(t, got.Items[0].Severity, "warning", "the severity of the first issue")
			assert.Equal(t, got.Items[0].Message, "prefer FieldsSeq", "the message of the first issue")
		})

		t.Run("marks no result with issues as failed", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Verify(gating())
			assert.NoError(t, err, "the error of Verify")
			result, err := built.Execute(t.Context(), json.RawMessage(`{"scope":"a.fx"}`))
			assert.NoError(t, err, "the error of Execute")
			assert.False(t, result.Failed, "Failed of the result")
		})

		t.Run("returns the source line of an issue and its line counted from one", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx"}`)
			assert.Equal(t, got.Items[0].At, "for part := range strings.Fields(x) {", "the source line")
			assert.Equal(t, got.Items[0].Line, 228, "the line")
		})

		t.Run("returns the text of the one obvious fix", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx"}`)
			assert.Length(t, got.Items[0].Fix, 1, "the fixes of the warning")
			assert.Equal(t, got.Items[0].Fix[0].Now, "for part := range strings.FieldsSeq(x) {", "the text of the fix")
			assert.Empty(t, got.Items[1].Fix, "the fixes of the error")
		})

		t.Run("returns the first issues up to max_issues with a truncation caveat", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx","max_issues":1}`)
			assert.Length(t, got.Items, 1, "the issues")
			assert.Equal(t, truncations(got.Provenance), []string{"1 of 2 issues returned"},
				"the notes of the truncation caveats")
		})

		t.Run("returns an unsupported failure for a scope that no engine serves", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"notes.md"}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Equal(t, got.Error.Code, "unsupported", "the code of the failure")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each issue with its site, its code and its fix", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx"}`)
			assert.ContainsInOrder(t, got.Render(), []string{
				"a.fx — 2 issues", "a.fx:228", "warning", "modernize.stringsseq",
				"prefer FieldsSeq", "strings.Fields(x)", "fix available",
			}, "the render")
		})
	})
}
