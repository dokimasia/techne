// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

func TestVerify(t *testing.T) {
	t.Parallel()

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("asks a server that answers when asked", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)

			assert.NoError(t, err, "verifying succeeds")
			assert.Length(t, got.Items, 3, "every diagnostic the server reported")
		})

		t.Run("reads a server that reports unasked", func(t *testing.T) {
			t.Parallel()
			// A server with no pull request is not a server with nothing
			// to say. Reading only the request reports it as clean, which
			// is a clean bill of health from something never asked.
			got, err := serving(t, modePushes, map[string]string{"a.fake": content}).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)

			assert.NoError(t, err, "verifying succeeds")
			assert.Length(t, got.Items, 3, "what the server published when it finished")
		})

		t.Run("grades a diagnostic the way this vocabulary does", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)

			assert.NoError(t, err, "verifying succeeds")
			assert.Equal(t, grades(got.Items),
				[]diag.Severity{diag.SeverityError, diag.SeverityWarning, diag.SeverityUnset},
				"each as the server graded it")
		})

		t.Run("leaves a diagnostic nobody graded ungraded", func(t *testing.T) {
			t.Parallel()
			// A server is not required to send a severity. Defaulting one
			// to the least serious hides it from a caller filtering for
			// errors, and defaulting it to error invents one.
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)

			assert.NoError(t, err, "verifying succeeds")
			assert.Equal(t, got.Items[2].Diagnostic.Severity, diag.SeverityUnset,
				"nobody graded it, which is different from grading it least")
		})

		t.Run("carries the rule and the tool that reported it", func(t *testing.T) {
			t.Parallel()
			// A caller suppressing by rule must not have to match the
			// message text, and a broken build must be tellable from a
			// linter's objection.
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)

			assert.NoError(t, err, "verifying succeeds")
			assert.Equal(t, got.Items[0].Diagnostic.Code, "E101", "the rule's own name")
			assert.Equal(t, got.Items[0].Diagnostic.Source, "fakecheck", "and what reported it")
		})

		t.Run("reads a rule the server numbered rather than named", func(t *testing.T) {
			t.Parallel()
			// The protocol writes a code as either a string or a number.
			// Reading one arm leaves the other empty, and a caller
			// suppressing by rule has nothing to suppress by.
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)

			assert.NoError(t, err, "verifying succeeds")
			assert.Equal(t, got.Items[1].Diagnostic.Code, "42", "the number, written out")
		})

		t.Run("carries the line each diagnostic is about", func(t *testing.T) {
			t.Parallel()
			// Whoever renders it has no filesystem, and a message with no
			// line to read it against costs a read per diagnostic.
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)

			assert.NoError(t, err, "verifying succeeds")
			assert.Equal(t, got.Items[0].Diagnostic.Snippet, "type Store struct {",
				"the source the diagnostic is about")
		})

		t.Run("says a suite it was given was not honoured", func(t *testing.T) {
			t.Parallel()
			// A server has one analysis and no notion of which linter to
			// run. Answering from it silently would report a suite as
			// having passed when it was never run.
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, []string{"vet"})

			assert.NoError(t, err, "verifying succeeds")
			assert.True(t, carries(got.Caveats, trust.CaveatUnsupported),
				"the caveat says the suite named was answered from the one analysis")
		})

		t.Run("says it read nothing where the scope holds none of its files", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, modeDefault, map[string]string{"notes.md": "# notes\n"}).
				Verify(t.Context(), engine.Request{Scope: "."}, nil)

			assert.NoError(t, err, "a scope with nothing to read is not a fault")
			assert.True(t, got.Skipped, "and the engine says it read nothing")
			assert.Empty(t, got.Items, "rather than reporting the scope as clean")
		})
	})
}

// grades is how each finding was graded, in order.
func grades(held []edit.Finding) []diag.Severity {
	out := make([]diag.Severity, 0, len(held))
	for _, one := range held {
		out = append(out, one.Diagnostic.Severity)
	}
	return out
}
