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

func TestVerify(t *testing.T) {
	t.Parallel()

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("names the built-in it replaces", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, verifying(t).Description(), "PREFER OVER ",
				"an agent runs the build in a shell unless told why not to")
		})

		t.Run("reports what the gate said", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx"}`)
			assert.False(t, got.Failed(), "a gate that ran answered the question")
			assert.Length(t, got.Items, 2, "both findings reach the caller")
			assert.Equal(t, got.Items[0].Severity, "warning", "the severity is a word")
			assert.Equal(t, got.Items[0].Message, "prefer FieldsSeq", "with what was said")
		})

		t.Run("does not read issues as a failure", func(t *testing.T) {
			t.Parallel()
			// A gate that ran and found twelve problems answered the
			// question. A caller that treats that as a fault cannot act
			// on the twelve.
			result, err := verifying(t).Execute(t.Context(), json.RawMessage(`{"scope":"a.fx"}`))
			assert.NoError(t, err, "running a gate succeeds")
			assert.False(t, result.Failed, "finding issues is an answer")
		})

		t.Run("carries the line a diagnostic is about", func(t *testing.T) {
			t.Parallel()
			// Whoever reads this has no filesystem, so a message without
			// its line costs a read each.
			got := verified(t, `{"scope":"a.fx"}`)
			assert.Equal(t, got.Items[0].At, "for part := range strings.Fields(x) {",
				"the code the complaint is about comes with it")
			assert.Equal(t, got.Items[0].Line, 228, "counting from one")
		})

		t.Run("carries a remedy where there is one obvious one", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx"}`)
			assert.Length(t, got.Items[0].Fix, 1, "the one obvious change comes with the complaint")
			assert.Equal(t, got.Items[0].Fix[0].Now, "for part := range strings.FieldsSeq(x) {",
				"so a lint, fix and verify cycle is two round trips rather than five")
			assert.Empty(t, got.Items[1].Fix,
				"and a complaint with no obvious change carries none")
		})

		t.Run("caps the issues when asked", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx","max_issues":1}`)
			assert.Length(t, got.Items, 1, "a caller that asked for one gets one")
		})

		t.Run("says a language is not served rather than reporting it clean", func(t *testing.T) {
			t.Parallel()
			// Reporting no issues for a language nothing gates is the
			// worst answer available: it reads as a pass.
			got := verified(t, `{"scope":"notes.md"}`)
			assert.True(t, got.Failed(), "nothing gates a markdown file")
			assert.Equal(t, got.Error.Code, "unsupported",
				"a capability gap is something a caller routes around")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each issue with its code and its line", func(t *testing.T) {
			t.Parallel()
			got := verified(t, `{"scope":"a.fx"}`)
			assert.ContainsInOrder(t, got.Render(), []string{
				"a.fx — 2 issues", "a.fx:228", "warning", "modernize.stringsseq",
				"prefer FieldsSeq", "strings.Fields(x)", "fix available",
			}, "a reader gets where, how much it matters, what is wrong and whether it is fixable")
		})
	})
}

// verifying builds the verify tool over two findings, one with an
// obvious remedy and one without.
func verifying(t *testing.T) tool.Tool {
	t.Helper()
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
	built, err := tool.Verify(over)
	assert.NoError(t, err, "the verify tool builds from a read service")
	return built
}

// verified runs it and decodes what came back.
func verified(t *testing.T, input string) tool.VerifyOutput {
	t.Helper()
	result, err := verifying(t).Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "a well-formed call reaches the service")

	var out tool.VerifyOutput
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the answer is JSON a caller can read")
	return out
}
