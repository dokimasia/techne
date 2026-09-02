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

// A server that has not finished reading the workspace answers every
// question with nothing — not "I do not know", but nothing, in the same
// shape as a real answer. Reported as it stands that is resolved binding
// over total coverage saying a declaration has no references, which is
// the claim a caller acts on by deleting it.
//
// These are the cases that stop that. The fake announces the work the
// way a server does, and answers with nothing until it is done.
func TestWorking(t *testing.T) {
	t.Parallel()

	t.Run("a server still reading the workspace", func(t *testing.T) {
		t.Parallel()

		t.Run("is waited for rather than believed", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeLoading, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.ReferencedBy)

			assert.NoError(t, err, "relating succeeds")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"},
				"the answer the server gives once it has read the workspace, "+
					"not the empty one it gives before")
		})

		t.Run("gives a total answer once it has finished", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeLoading, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.ReferencedBy)

			assert.NoError(t, err, "relating succeeds")
			assert.Equal(t, got.Completeness, trust.ScopeTotal,
				"a server that finished read everything it was going to")
			assert.False(t, carries(got.Caveats, trust.CaveatIndexWarming),
				"so there is nothing to warn about")
		})
	})

	t.Run("a server that never finishes", func(t *testing.T) {
		t.Parallel()

		t.Run("is answered from, and the answer says so", func(t *testing.T) {
			t.Parallel()
			// Waiting forever is a tool that hangs. What it must not do
			// is pass off what a half-read workspace produced as
			// everything there is.
			e := serving(t, modeStuck, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.ReferencedBy)

			assert.NoError(t, err, "a slow server is not a fault")
			assert.Equal(t, got.Completeness, trust.ScopePartial,
				"what it returned is not everything there is")
			assert.True(t, carries(got.Caveats, trust.CaveatIndexWarming),
				"and the answer names why")
		})

		t.Run("supports no claim that something is absent", func(t *testing.T) {
			t.Parallel()
			// The whole point. Resolved binding over total coverage is
			// what lets a caller conclude a declaration is unused, and a
			// server that has read half the workspace cannot support it.
			e := serving(t, modeStuck, map[string]string{"a.fake": content})
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake"},
				subject(t, e, "Store"), sema.ReferencedBy)

			assert.NoError(t, err, "relating succeeds")
			assert.Empty(t, got.Items, "the server had nothing to say yet")
			assert.False(t, trust.SupportsNegativeClaim(trust.Resolved, got.Completeness),
				"so nothing may be read out of its silence")
		})
	})
}
