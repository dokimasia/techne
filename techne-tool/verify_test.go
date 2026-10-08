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
				Span:    source.Span{Path: "a.fx", Start: source.Position{Line: 227, Column: 18}},
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

		t.Run("returns each issue that the check reports", func(t *testing.T) {
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

		t.Run("returns the source line of an issue", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx"}`)
			assert.Equal(t, got.Items[0].At, "for part := range strings.Fields(x) {", "the source line")
		})

		t.Run("returns the line of an issue counted from one", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx"}`)
			assert.Equal(t, got.Items[0].Line, 228, "the line")
		})

		t.Run("returns the column of an issue counted from one", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx"}`)
			assert.Equal(t, got.Items[0].Column, 19, "the column")
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

		t.Run("refuses a path that leaves the workspace", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"../b.fx"}`)
			assert.Equal(t, got.Error.Code, "refused", "the code of the failure")
			assert.Equal(t, got.Error.Reason, `"../b.fx" leaves the workspace root`, "the reason of the failure")
		})

		t.Run("returns the weakest tiers for a refused request", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx","preferred_fidelity":"exact"}`)
			assert.Equal(t, got.Error.Code, "refused", "the code of the failure")
			assert.Equal(t, got.Provenance.Fidelity, "none", "the fidelity")
			assert.Equal(t, got.Provenance.Completeness, "unknown", "the completeness")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each issue from its site to its fix", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx"}`)
			assert.ContainsInOrder(t, got.Render(), []string{
				"a.fx — 2 issues", "a.fx:228:19", "warning", "modernize.stringsseq",
				"prefer FieldsSeq", "strings.Fields(x)", "fix available",
			}, "the render")
		})

		t.Run("writes the column of each of two issues on one line", func(t *testing.T) {
			t.Parallel()
			got := tool.VerifyOutput{Scope: tool.Scope{Path: "a.fx"}, Items: []tool.Reported{
				{Severity: "error", Message: "undefined: missing", Path: "a.fx", Line: 4, Column: 9},
				{Severity: "error", Message: "undefined: missing", Path: "a.fx", Line: 4, Column: 19},
			}}.Render()
			assert.That(t, got).
				Contains("a.fx:4:9  error", "the render of the first issue").
				Contains("a.fx:4:19  error", "the render of the second issue")
		})
	})
}
