// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// asking is how long a test waits after it starts a question for the question to read the
// buffers: long enough for a running server to answer the first requests of the question, and
// shorter than the second for which the Hangs mode holds a question.
const asking = 200 * time.Millisecond

func TestCheck(t *testing.T) {
	t.Parallel()

	faulty := map[source.Path][]byte{"a.fake": []byte(lsptest.Faulty)}

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the error of content that is not on disk", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.Length(t, got.Items, 1, "the findings of faulty content")
			assert.Equal(t, got.Items[0].Diagnostic.Severity, diag.SeverityError, "the severity of the finding")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("waits for a question that reads the buffers", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Hangs)
			server.Answering = time.Second
			e := lsptest.Engine(t, lsptest.Workspace(t, sample()), server)
			request := engine.Request{Scope: "a.fake"}
			_, err := e.Resolve(t.Context(), request, store())
			assert.NoError(t, err, "Resolve, which starts the server")
			// ends numbers the end of the question and the end of the check in the order in which they
			// happen.
			var ends atomic.Int32
			related := make(chan int32, 1)
			go func() {
				_, _ = e.Relate(t.Context(), request, declared("Store", sema.KindStruct), sema.ReferencedBy)
				related <- ends.Add(1)
			}()
			time.Sleep(asking)
			_, _ = e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(lsptest.Content)})
			checked := ends.Add(1)
			assert.Equal(t, <-related, checked-1, "the question ended before the check")
		})

		t.Run("returns nothing for content that compiles", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).
				Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(lsptest.Content)})
			assert.NoError(t, err, "Check of content that compiles")
			assert.Empty(t, got.Items, "the findings of content that compiles")
		})

		t.Run("returns the one fix the server offers", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.Length(t, got.Items[0].Fix, 1, "the changes of the fix")
			assert.Equal(t, got.Items[0].Fix[0].Edits[0].New, "declared", "the text of the fix")
		})

		t.Run("converts the fix against the checked content", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			at := got.Items[0].Fix[0].Edits[0].Span
			assert.Equal(t, lsptest.Faulty[at.Start.Offset:at.End.Offset], lsptest.Broken,
				"the text that the fix replaces")
		})

		t.Run("sends the content on disk again after the check", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Compiles, sample())
			_, err := e.Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify after the check")
			assert.Empty(t, got.Items, "the findings of the file on disk")
		})

		t.Run("declines content of another language", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Compiles, sample()).
				Check(t.Context(), map[source.Path][]byte{"notes.md": []byte("# notes\n")})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check")
		})

		t.Run("declines a change that deletes the file", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Compiles, sample()).
				Check(t.Context(), map[source.Path][]byte{"a.fake": nil})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check")
		})

		t.Run("returns a report of clean content from a quiet server", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Quiet, sample())
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify opens a.fake")

			got, err := e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(lsptest.Content + "\n")})
			assert.NoError(t, err, "Check of clean content")
			assert.Empty(t, got.Items, "the findings of clean content")
		})

		t.Run("reads the report of new content after the empty report of a close", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Quiet, sample())
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify opens a.fake")

			got, err := e.Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.Length(t, got.Items, 1, "the findings of faulty content")
		})

		t.Run("declines content the server reports nothing about", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Ungated)
			server.Checking = asking
			_, err := lsptest.Engine(t, lsptest.Workspace(t, sample()), server).Check(t.Context(), faulty)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check")
		})

		t.Run("declines content the server reports nothing about within Checking", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Ungated)
			server.Checking = asking
			e := lsptest.Engine(t, lsptest.Workspace(t, sample()), server)
			began := time.Now()
			_, err := e.Check(t.Context(), faulty)
			took := time.Since(began)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check")
			assert.InRange(t, took, -1<<63, float64(10*time.Second-1), "Check took "+took.String())
		})

		t.Run("returns the report of a quiet server that publishes it after two seconds", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Quiet, sample(), lsptest.Delaying(2500*time.Millisecond))
			got, err := e.Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.Length(t, got.Items, 1, "the findings of faulty content")
		})

		t.Run("declines while the server loads the workspace", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Stuck, sample()).Check(t.Context(), faulty)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check")
			assert.Contains(t, err.Error(), "loading", "the error of Check")
		})

		t.Run("waits for the diagnosis of the change", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Diagnoses, sample())
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify waits for the load of the server")

			got, err := e.Check(t.Context(), faulty)
			assert.NoError(t, err, "Check with a server that diagnoses each change")
			assert.Length(t, got.Items, 1, "the findings of faulty content")
		})

		t.Run("declines while a diagnosis runs", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.DiagnosisStuck, sample()).Check(t.Context(), faulty)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Check")
			assert.Contains(t, err.Error(), "did not finish diagnosing the change", "the error of Check")
		})

		t.Run("returns an error in a file that depends on the change", func(t *testing.T) {
			t.Parallel()
			renamed := strings.Replace(lsptest.Content, "type Store", "type Vault", 1)
			got, err := serving(t, lsptest.WorkspaceDiagnostics, map[string]string{
				"a.fake": lsptest.Content, "b.fake": "var _ Store\n",
			}).Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(renamed)})
			assert.NoError(t, err, "Check of a rename that misses b.fake")
			assert.True(t, slices.ContainsFunc(got.Items, func(one edit.Finding) bool {
				return one.Diagnostic.Span.Path == "b.fake"
			}), "a finding in b.fake")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatDependents), "the answer has a dependents caveat")
		})

		t.Run("shows the server a file that the change deletes as removed", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.WorkspaceDiagnostics, sample()).Check(t.Context(),
				map[source.Path][]byte{"a.fake": nil, "b.fake": []byte("var _ Store\n")})
			assert.NoError(t, err, "Check of a change that deletes a.fake")
			assert.True(t, slices.ContainsFunc(got.Items, func(one edit.Finding) bool {
				return one.Diagnostic.Span.Path == "b.fake" && one.Diagnostic.Message == "Store is not declared"
			}), "b.fake uses a Store that no file declares")
			assert.False(t, slices.ContainsFunc(got.Items, func(one edit.Finding) bool {
				return one.Diagnostic.Message == lsptest.EmptyFile
			}), "a.fake is shown as removed and not as empty")
		})

		t.Run("adds a dependents caveat for a server without workspace diagnostics", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatDependents), "the answer has a dependents caveat")
		})

		t.Run("adds a partial-check caveat for a server that leaves checks out", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Compiles)
			server.Unchecked = "lifetimes or borrows"
			got, err := lsptest.Engine(t, lsptest.Workspace(t, sample()), server).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatPartialCheck), "the answer has a partial-check caveat")
		})

		t.Run("adds no partial-check caveat for a server that checks what the compiler checks", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Compiles, sample()).Check(t.Context(), faulty)
			assert.NoError(t, err, "Check of faulty content")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatPartialCheck), "the answer has a partial-check caveat")
		})

		t.Run("leaves out the findings of the check on disk", func(t *testing.T) {
			t.Parallel()
			unsound := lsptest.Content + lsptest.Unsound + "\n"
			e := serving(t, lsptest.DiskChecks, map[string]string{"a.fake": unsound})
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify waits for the check on disk")

			got, err := e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(unsound + "\n")})
			assert.NoError(t, err, "Check of content that is not on disk")
			assert.Equal(t, mentions(got.Items, lsptest.Unsound), 0, "the findings of the check on disk")
		})
	})
}
