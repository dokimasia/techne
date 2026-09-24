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
