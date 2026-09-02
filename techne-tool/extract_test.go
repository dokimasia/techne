// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/tool"
)

func TestExtract(t *testing.T) {
	t.Parallel()

	t.Run("Extract", func(t *testing.T) {
		t.Parallel()

		t.Run("names the built-in it replaces", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, extracting(t, &recorder{}).Description(), "PREFER OVER ",
				"an agent cuts the lines out by hand unless told why not to")
		})

		t.Run("takes the lines as an editor numbers them", func(t *testing.T) {
			t.Parallel()
			// Counting from one, and including the last: a caller
			// selecting lines 10 to 12 means three lines.
			writer := &recorder{}
			got := extracted(t, writer,
				`{"path":"a.fx","first_line":10,"last_line":12,"new_name":"parsed"}`)

			assert.False(t, got.Failed(), "a selection an editor could make is served")
			assert.Equal(t, writer.asked.Target.Kind, edit.TargetSpan, "the target is the selection")
			assert.Equal(t, writer.asked.Target.Span.Start.Line, 9, "counted from zero inside")
			assert.Equal(t, writer.asked.Target.Span.End.Line, 11, "at both ends")
		})

		t.Run("passes a receiver only where the caller gave one", func(t *testing.T) {
			t.Parallel()
			// A key nobody set is an argument the operation would be
			// validated against and refuse.
			writer := &recorder{}
			extracted(t, writer, `{"path":"a.fx","first_line":1,"last_line":2,"new_name":"parsed"}`)
			_, held := writer.asked.Args[edit.ArgReceiver]
			assert.False(t, held, "an unset receiver is absent rather than empty")

			extracted(t, writer,
				`{"path":"a.fx","first_line":1,"last_line":2,"new_name":"parsed","receiver":"Store"}`)
			assert.Equal(t, writer.asked.Args[edit.ArgReceiver], "Store",
				"and one the caller gave reaches the planner")
		})

		t.Run("refuses a selection counted from zero", func(t *testing.T) {
			t.Parallel()
			got := extracted(t, &recorder{},
				`{"path":"a.fx","first_line":0,"last_line":2,"new_name":"parsed"}`)
			assert.True(t, got.Failed(), "lines count from one")
			assert.Contains(t, got.Error.Reason, "count from one", "and the caller is told which way")
		})

		t.Run("refuses a selection that ends before it starts", func(t *testing.T) {
			t.Parallel()
			got := extracted(t, &recorder{},
				`{"path":"a.fx","first_line":12,"last_line":10,"new_name":"parsed"}`)
			assert.True(t, got.Failed(), "an empty selection holds nothing to extract")
		})

		t.Run("refuses a call with nothing to call the function", func(t *testing.T) {
			t.Parallel()
			got := extracted(t, &recorder{},
				`{"path":"a.fx","first_line":1,"last_line":2,"new_name":""}`)
			assert.True(t, got.Failed(), "a function needs a name")
		})
	})
}

// extracting builds the extract tool.
func extracting(t *testing.T, writer *recorder) tool.Tool {
	t.Helper()
	built, err := tool.Extract(writer)
	assert.NoError(t, err, "the extract tool builds from a write path alone")
	return built
}

// extracted runs it and decodes what came back.
func extracted(t *testing.T, writer *recorder, input string) tool.Written {
	t.Helper()
	result, err := extracting(t, writer).Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "a well-formed call reaches the service")

	var out tool.Written
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the answer is JSON a caller can read")
	return out
}
