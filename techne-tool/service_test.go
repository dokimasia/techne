// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/tool"
)

func TestService(t *testing.T) {
	t.Parallel()

	t.Run("what a tool is given", func(t *testing.T) {
		t.Parallel()

		t.Run("is only what it uses", func(t *testing.T) {
			t.Parallel()
			// Each interface asks one question, so a change to a service
			// method no tool calls cannot break a tool. A type serving
			// outline alone builds the outline tool, and that is the
			// check: it would not compile if the tool asked for more.
			var reads tool.Outliner = serving()
			_, err := tool.Outline(reads)
			assert.NoError(t, err, "outline needs somewhere to outline, and nothing else")
		})

		t.Run("is declared here rather than exported by whoever implements it", func(t *testing.T) {
			t.Parallel()
			// The interfaces name no service and this module depends on
			// none. A tool that could name a service would be tested
			// against one, and what a tool does with an answer would
			// stop being separable from how the answer was assembled.
			for _, satisfied := range []bool{
				satisfies[tool.Outliner](serving()),
				satisfies[tool.Searcher](serving()),
			} {
				assert.True(t, satisfied,
					"a double in this package satisfies the port, so no service is needed to test a tool")
			}
		})
	})
}

// satisfies reports whether a value implements an interface, without
// naming the thing that implements it in production.
func satisfies[T any](v any) bool {
	_, ok := v.(T)
	return ok
}
