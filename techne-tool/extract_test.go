// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/tool"
)

// extracted runs the extract.function tool over writer with input, and decodes the output.
func extracted(t *testing.T, writer *recorder, input string) tool.Written {
	t.Helper()
	built, err := tool.Extract(writer)
	assert.NoError(t, err, "the error of Extract")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.Written
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out
}

func TestExtract(t *testing.T) {
	t.Parallel()

	t.Run("Extract", func(t *testing.T) {
		t.Parallel()

		t.Run("starts its description with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Extract(&recorder{})
			assert.NoError(t, err, "the error of Extract")
			assert.HasPrefix(t, built.Description(), "PREFER OVER ", "the description")
		})

		t.Run("points the write path at the lines counted from zero", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			got := extracted(t, writer, `{"path":"a.fx","first_line":10,"last_line":12,"new_name":"parsed"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, writer.asked.Target.Kind, edit.TargetSpan, "the kind of the target")
			assert.Equal(t, writer.asked.Target.Span.Start.Line, 9, "the first line of the target")
			assert.Equal(t, writer.asked.Target.Span.End.Line, 11, "the last line of the target")
		})

		t.Run("sends only the arguments that the operation declares", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			extracted(t, writer, `{"path":"a.fx","first_line":1,"last_line":2,"new_name":"parsed"}`)
			spec, declared := edit.SpecFor(edit.ExtractFunction)
			assert.True(t, declared, "the spec of extract.function")
			for key := range writer.asked.Args {
				assert.True(t, slices.Contains(spec.Required, key) || slices.Contains(spec.Optional, key),
					"the argument "+string(key))
			}
		})

		t.Run("refuses a first line of zero", func(t *testing.T) {
			t.Parallel()
			got := extracted(t, &recorder{}, `{"path":"a.fx","first_line":0,"last_line":2,"new_name":"parsed"}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Contains(t, got.Error.Reason, "count from one", "the reason of the failure")
		})

		t.Run("refuses a last line before the first line", func(t *testing.T) {
			t.Parallel()
			got := extracted(t, &recorder{}, `{"path":"a.fx","first_line":12,"last_line":10,"new_name":"parsed"}`)
			assert.True(t, got.Failed(), "the failure of the output")
		})

		t.Run("refuses an empty name", func(t *testing.T) {
			t.Parallel()
			got := extracted(t, &recorder{}, `{"path":"a.fx","first_line":1,"last_line":2,"new_name":""}`)
			assert.True(t, got.Failed(), "the failure of the output")
		})
	})
}
