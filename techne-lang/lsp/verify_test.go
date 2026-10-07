// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// diagnosingNote is a part of the note of the caveat of a verification whose diagnosis did not
// end, pinned as the engine words it.
const diagnosingNote = "did not finish diagnosing the files"

func TestVerify(t *testing.T) {
	t.Parallel()

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the diagnostics of a server with pull diagnostics", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify")
			assert.Length(t, got.Items, 3, "the findings of a.fake")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("declines a suite", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, []string{"vet"})
			assert.ErrorIs(t, err, engine.ErrDecline, "Verify with the suite vet")
			assert.Contains(t, err.Error(), "runs no suite, and not vet", "the reason of the decline")
		})

		t.Run("skips a scope without a file of the language", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, map[string]string{"notes.md": "# notes\n"}).
				Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of a scope without a file of the language")
			assert.True(t, got.Skipped, "Skipped of the answer")
		})

		t.Run("returns a partial answer for a file without a report", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Ungated, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify with a server that reports nothing")
			assert.Empty(t, got.Items, "the findings of a server that reports nothing")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatIndexWarming), "the answer has a warming caveat")
		})

		t.Run("returns a total answer when every file has a report", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Pushes, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify with a server that publishes")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatIndexWarming), "the answer has a warming caveat")
		})

		t.Run("returns a total answer after a check of clean content with a quiet server", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Quiet, sample())
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify opens a.fake")
			_, err = e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(lsptest.Content + "\n")})
			assert.NoError(t, err, "Check of clean content")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify after the check")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("reads the report of a file that returns after the engine released its buffer", func(t *testing.T) {
			t.Parallel()
			e, root := rooted(t, lsptest.Quiet, map[string]string{"a.fake": lsptest.Content, "b.fake": lsptest.Content})
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify opens a.fake")
			assert.NoError(t, os.Remove(filepath.Join(root, "a.fake")), "Remove of a.fake")
			// The question releases the buffer of a.fake. The server publishes an empty report of
			// a.fake a QuietClose after the close, before the report of b.fake.
			_, err = e.Verify(t.Context(), engine.Request{Scope: "b.fake"}, nil)
			assert.NoError(t, err, "Verify of b.fake releases the buffer of a.fake")
			rewrite(t, root, "a.fake", lsptest.Content+lsptest.Broken+"\n")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify of a.fake after it returns")
			assert.Equal(t, mentions(got.Items, lsptest.Broken), 1, "the findings of a.fake")
		})

		t.Run("returns within 3 seconds for ten files without a report", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{}
			for i := range 10 {
				files[fmt.Sprintf("f%d.fake", i)] = lsptest.Content
			}
			e := serving(t, lsptest.Ungated, files)
			_, err := e.Verify(t.Context(), engine.Request{Scope: "f0.fake"}, nil)
			assert.NoError(t, err, "Verify starts the server")

			began := time.Now()
			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			took := time.Since(began)
			assert.NoError(t, err, "Verify of ten files")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.InRange(t, took, -1<<63, float64(3*time.Second-1), "Verify of ten files took "+took.String())
		})

		t.Run("returns the findings of the check on disk beside the pulled diagnostics", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.DiskChecks, map[string]string{
				"a.fake": lsptest.Content + lsptest.Broken + " " + lsptest.Unsound + "\n",
			}).Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify with a server that checks the files on disk")
			assert.Equal(t, mentions(got.Items, lsptest.Broken), 1, "the pulled findings of a.fake")
			assert.Equal(t, mentions(got.Items, lsptest.Unsound), 1, "the findings of the check on disk")
		})

		t.Run("waits for the check on disk after a file changes on disk", func(t *testing.T) {
			t.Parallel()
			e, root := rooted(t, lsptest.DiskChecks, sample())
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify before the change")
			rewrite(t, root, "a.fake", lsptest.Content+lsptest.Unsound+"\n")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify after the change")
			assert.Equal(t, mentions(got.Items, lsptest.Unsound), 1, "the findings of the check on disk")
		})

		t.Run("checks a file that the workspace gained after the server started", func(t *testing.T) {
			t.Parallel()
			e, root := rooted(t, lsptest.DiskChecks, sample())
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify starts the server")
			rewrite(t, root, "b.fake", lsptest.Content+lsptest.Unsound+"\n")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "b.fake"}, nil)
			assert.NoError(t, err, "Verify of the new file")
			assert.Equal(t, mentions(got.Items, lsptest.Unsound), 1, "the findings of the check on disk")
		})

		t.Run("starts no check on disk after a check of other content", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.DiskChecks, sample())
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify before the check")
			_, err = e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(lsptest.Content + "\n")})
			assert.NoError(t, err, "Check of other content")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify after the check")
			assert.Equal(t, mentions(got.Items, "checks=1"), 1, "the note of the first check on disk")
		})

		t.Run("keeps the findings of the check on disk after a check of other content", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.DiskChecks, map[string]string{"a.fake": lsptest.Content + lsptest.Unsound + "\n"})
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify before the check")
			_, err = e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(lsptest.Content)})
			assert.NoError(t, err, "Check of other content")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify after the check")
			assert.Equal(t, mentions(got.Items, lsptest.Unsound), 1, "the findings of the check on disk")
		})

		t.Run("adds no partial-check caveat after the check on disk ends", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.DiskChecks, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify with a server that checks the files on disk")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatPartialCheck), "the answer has a partial-check caveat")
		})

		t.Run("adds a partial-check caveat when the check on disk does not end", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.DiskStuck, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify with a check on disk that never ends")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatPartialCheck), "the answer has a partial-check caveat")
		})

		t.Run("waits for the diagnosis of each changed file", func(t *testing.T) {
			t.Parallel()
			e, root := rooted(t, lsptest.Diagnoses,
				map[string]string{"a.fake": lsptest.Content, "b.fake": lsptest.Content})
			_, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify waits for the load of the server")
			rewrite(t, root, "a.fake", lsptest.Faulty)
			rewrite(t, root, "b.fake", lsptest.Faulty)

			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of the changed files")
			assert.Equal(t, mentions(got.Items, lsptest.Broken), 2, "the findings of a.fake and b.fake")
		})

		t.Run("returns a total answer after every diagnosis ends", func(t *testing.T) {
			t.Parallel()
			e, root := rooted(t, lsptest.Diagnoses, sample())
			_, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify waits for the load of the server")
			rewrite(t, root, "a.fake", lsptest.Faulty)

			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify of the changed file")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("returns a total answer for a scope whose only file is a test that the request leaves out",
			func(t *testing.T) {
				t.Parallel()
				got, err := serving(t, lsptest.DiagnosisStuck, map[string]string{lsptest.Test: lsptest.Content}).
					Verify(t.Context(), engine.Request{Scope: lsptest.Test}, nil)
				assert.NoError(t, err, "Verify of a test file without tests")
				assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
			})

		t.Run("sends a server without a diagnosis no request after the files", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "requests")
			_, err := serving(t, lsptest.Default, sample(), lsptest.RecordRequests(log)).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify with a server without a diagnosis")
			assert.Equal(t, requested(t, log, "textDocument/documentSymbol"), 0, "the requests for symbols")
		})

		t.Run("returns a partial answer while a diagnosis runs", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.DiagnosisStuck, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify with a diagnosis that never ends")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.True(t, undiagnosed(got.Caveats), "the answer has the caveat of a diagnosis that did not end")
		})

		t.Run("adds the caveat of an unfinished diagnosis when the question ends before the diagnosis",
			func(t *testing.T) {
				t.Parallel()
				server := lsptest.Server(lsptest.Diagnoses)
				server.Answering = 200 * time.Millisecond
				got, err := lsptest.Engine(t, lsptest.Workspace(t, sample()), server).
					Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
				assert.NoError(t, err, "Verify whose question ends while the server loads")
				assert.True(t, undiagnosed(got.Caveats), "the answer has the caveat of a diagnosis that did not end")
			})

		t.Run("adds the caveat of an unfinished diagnosis when the question ends during it", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.DiagnosisStuck)
			server.Answering = 600 * time.Millisecond
			got, err := lsptest.Engine(t, lsptest.Workspace(t, sample()), server).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify whose question ends during a diagnosis")
			assert.True(t, undiagnosed(got.Caveats), "the answer has the caveat of a diagnosis that did not end")
		})

		t.Run("adds a partial-check caveat for a server that leaves checks out", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Compiles)
			server.Unchecked = lsptest.Unchecked
			got, err := lsptest.Engine(t, lsptest.Workspace(t, sample()), server).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify with a server that leaves checks out")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatPartialCheck), "the answer has a partial-check caveat")
		})

		t.Run("returns a partial answer for a scope with a file larger than lang.Largest", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, map[string]string{
				"a.fake":   lsptest.Content,
				"big.fake": strings.Repeat("x", lang.Largest+1),
			}).Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of a scope with a large file")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.True(t, unreadIn(got.Caveats, "big.fake"), "the unread caveat names big.fake")
		})
	})
}

// undiagnosed reports whether caveats contain the caveat of a verification whose diagnosis did
// not end.
func undiagnosed(caveats []trust.Caveat) bool {
	return slices.ContainsFunc(caveats, func(one trust.Caveat) bool {
		return one.Code == trust.CaveatIndexWarming && strings.Contains(one.Note, diagnosingNote)
	})
}

// unreadIn reports whether a [trust.CaveatUnread] caveat of caveats names p.
func unreadIn(caveats []trust.Caveat, p source.Path) bool {
	for _, one := range caveats {
		if one.Code == trust.CaveatUnread && slices.Contains(one.Paths, p) {
			return true
		}
	}
	return false
}
