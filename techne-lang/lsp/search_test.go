// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
)

func TestSearch(t *testing.T) {
	t.Parallel()

	t.Run("Search", func(t *testing.T) {
		t.Parallel()

		t.Run("reports what the server's index matched", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Search(t.Context(), engine.Request{Scope: "."}, engine.Query{
					Text: "St", Private: true,
				})

			assert.NoError(t, err, "searching succeeds")
			assert.Equal(t, names(got.Items), []string{"Store", "size"},
				"every match, in the order the server ranked them")
		})

		t.Run("reads a match that names a file and no range in it", func(t *testing.T) {
			t.Parallel()
			// The newer shape lets a server defer working the range out
			// until a caller asks about that one symbol. Dropping it
			// loses a declaration the server found.
			got, err := serving(t, modeUnranged, map[string]string{"a.fake": content}).
				Search(t.Context(), engine.Request{Scope: "."}, engine.Query{Text: "St"})

			assert.NoError(t, err, "searching succeeds")
			assert.Length(t, got.Items, 1, "a match with no range is still a match")
			assert.Equal(t, got.Items[0].Name, "Store", "and names what was found")
		})

		t.Run("fills in what the answer does not carry", func(t *testing.T) {
			t.Parallel()
			// A workspace symbol carries a name, a kind and a range, and
			// no signature and no source. A caller that found what it
			// wanted should not need a second call to read it.
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Search(t.Context(), engine.Request{Scope: "."}, engine.Query{Text: "St"})

			assert.NoError(t, err, "searching succeeds")
			assert.Equal(t, got.Items[0].Signature, "type Store struct {",
				"the declaration as it is written")
			assert.HasPrefix(t, got.Items[0].Snippet, "type Store struct {",
				"and its own source")
		})

		t.Run("is never total, because a server caps its own index", func(t *testing.T) {
			t.Parallel()
			// Resolved binding over total coverage is what lets a caller
			// conclude something does not exist. A capped index cannot
			// support that: the name it did not return may be the
			// hundred and first rather than absent.
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Search(t.Context(), engine.Request{Scope: "."}, engine.Query{Text: "St"})

			assert.NoError(t, err, "searching succeeds")
			assert.Equal(t, got.Completeness, trust.ScopePartial,
				"a workspace query says nothing about what it did not return")
			assert.False(t, trust.SupportsNegativeClaim(trust.Resolved, got.Completeness),
				"so no caller can read an absence out of it")
			assert.True(t, carries(got.Caveats, trust.CaveatTruncated), "and the answer says why")
		})

		t.Run("narrows to a kind the protocol's query cannot express", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Search(t.Context(), engine.Request{Scope: "."}, engine.Query{
					Text: "St", Kind: sema.KindField, Private: true,
				})

			assert.NoError(t, err, "searching succeeds")
			assert.Equal(t, names(got.Items), []string{"size"}, "only the kind asked for")
		})

		t.Run("leaves out what is not visible outside its unit by default", func(t *testing.T) {
			t.Parallel()
			// A caller asking what a workspace offers is asking what it
			// exposes. The protocol's query is one string and cannot say
			// so, which is why it is said here.
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Search(t.Context(), engine.Request{Scope: "."}, engine.Query{Text: "St"})

			assert.NoError(t, err, "searching succeeds")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the exported one alone")
		})

		t.Run("stops at the limit the caller set", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Search(t.Context(), engine.Request{Scope: "."}, engine.Query{
					Text: "St", Private: true, Limit: 1,
				})

			assert.NoError(t, err, "searching succeeds")
			assert.Length(t, got.Items, 1, "no more than asked for")
		})

		t.Run("leaves out a match outside the scope", func(t *testing.T) {
			t.Parallel()
			// A workspace query covers the workspace, and a caller that
			// named a directory asked about that directory.
			got, err := serving(t, modeDefault, map[string]string{"a.fake": content}).
				Search(t.Context(), engine.Request{Scope: "elsewhere"}, engine.Query{
					Text: "St", Private: true,
				})

			assert.NoError(t, err, "a scope the matches fall outside is not a fault")
			assert.Empty(t, got.Items, "and nothing in it matched")
		})
	})
}
