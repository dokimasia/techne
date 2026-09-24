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

// moved runs the move.file tool over writer with input, and decodes the output.
func moved(t *testing.T, writer *recorder, input string) tool.Written {
	t.Helper()
	built, err := tool.Move(writer)
	assert.NoError(t, err, "the error of Move")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.Written
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out
}

func TestMove(t *testing.T) {
	t.Parallel()

	t.Run("Move", func(t *testing.T) {
		t.Parallel()

		t.Run("starts its description with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Move(&recorder{})
			assert.NoError(t, err, "the error of Move")
			assert.HasPrefix(t, built.Description(), "PREFER OVER ", "the description")
		})

		t.Run("points the write path at the file and its destination", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			got := moved(t, writer, `{"path":"a/b.fx","to":"c/d.fx"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, writer.asked.Target.Kind, edit.TargetFile, "the kind of the target")
			assert.Equal(t, string(writer.asked.Target.Path), "a/b.fx", "the file of the target")
			assert.Equal(t, writer.asked.Args[edit.ArgDestination], "c/d.fx", "the destination of the request")
		})

		t.Run("refuses an empty destination", func(t *testing.T) {
			t.Parallel()
			got := moved(t, &recorder{}, `{"path":"a/b.fx","to":""}`)
			assert.True(t, got.Failed(), "the failure of the output")
		})

		t.Run("refuses the path of the file as its destination", func(t *testing.T) {
			t.Parallel()
			got := moved(t, &recorder{}, `{"path":"a/b.fx","to":"a/b.fx"}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Contains(t, got.Error.Reason, "already at a/b.fx", "the reason of the failure")
		})

		t.Run("refuses a path that leaves the workspace", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Move(&recorder{})
			assert.NoError(t, err, "the error of Move")
			for _, call := range []string{`{"path":"../escaped.fx","to":"a.fx"}`, `{"path":"a.fx","to":"../escaped.fx"}`} {
				_, err := built.Execute(t.Context(), json.RawMessage(call))
				assert.HasError(t, err, "the error of Execute for "+call)
			}
		})
	})
}
