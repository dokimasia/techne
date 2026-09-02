// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/tool"
)

// TestAddress covers the rule every tool that names a declaration
// addresses by. It is driven through the tools rather than called
// directly, because the rule is only worth anything where a caller
// meets it.
func TestAddress(t *testing.T) {
	t.Parallel()

	t.Run("a name", func(t *testing.T) {
		t.Parallel()

		t.Run("means the same thing to every tool that takes one", func(t *testing.T) {
			t.Parallel()
			// One rule, in one place. Two tools where a name meant
			// slightly different things would be two tools an agent has
			// to learn separately.
			documented := documenting(t, &recorder{}, `{"scope":"a.fx","name":"Get","doc":"x"}`)
			assert.True(t, documented.Failed(), "the name is ambiguous")

			var related tool.RelationsOutput
			result, err := relating(t).Execute(t.Context(),
				json.RawMessage(`{"scope":"a.fx","name":"Get","relation":"called-by"}`))
			assert.NoError(t, err, "a well-formed call is served")
			assert.NoError(t, json.Unmarshal(result.Payload, &related), "the answer is JSON")

			assert.True(t, related.Failed(), "and it is ambiguous to the other tool too")
			assert.Equal(t, related.Error.Reason, documented.Error.Reason,
				"both tools refuse it for the same stated reason")
		})
	})
}
