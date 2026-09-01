// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package presenter_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/tool"
)

// TestDoc covers the claim the package comment makes: this package holds
// no domain knowledge and never reads a payload.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("a change to what a tool returns", func(t *testing.T) {
		t.Parallel()

		t.Run("does not reach the transport", func(t *testing.T) {
			t.Parallel()
			// Two tools returning entirely different shapes are driven
			// by the same code. The transport learns a failure from
			// tool.Result rather than from the payload.
			type unrelated struct {
				Whatever []int `json:"whatever"`
			}
			built, err := tool.New("unrelated", "PREFER OVER nothing.",
				func(context.Context, answerIn) (unrelated, error) {
					return unrelated{Whatever: []int{1, 2, 3}}, nil
				})
			assert.NoError(t, err, "a handler over serialisable types produces a tool")

			r := tool.NewRegistry()
			assert.NoError(t, r.Add(built), "the case needs a tool registered")

			got := call(t, r, "unrelated", `{"scope":"a.fx"}`)
			assert.False(t, got.IsError, "a shape the transport has never seen is served unchanged")
			assert.NotNil(t, got.StructuredContent, "the answer reaches the caller whatever its shape")
		})
	})
}
