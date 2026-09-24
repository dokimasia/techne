// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"fmt"
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

		t.Run("adds a caveat for the suites it ignores", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, []string{"vet"})
			assert.NoError(t, err, "Verify with the suite vet")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatUnsupported), "the answer has an unsupported caveat")
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
			assert.True(t, took < 3*time.Second, "Verify of ten files took "+took.String())
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

// unreadIn reports whether a [trust.CaveatUnread] caveat of caveats names p.
func unreadIn(caveats []trust.Caveat, p source.Path) bool {
	for _, one := range caveats {
		if one.Code == trust.CaveatUnread && slices.Contains(one.Paths, p) {
			return true
		}
	}
	return false
}
