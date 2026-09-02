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

func TestMove(t *testing.T) {
	t.Parallel()

	t.Run("Move", func(t *testing.T) {
		t.Parallel()

		t.Run("names the built-in it replaces", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, moving(t, &recorder{}).Description(), "PREFER OVER ",
				"an agent moves the file and mends the imports unless told why not to")
		})

		t.Run("points at the file itself, needing nothing looked up", func(t *testing.T) {
			t.Parallel()
			// A file names itself, which is why this is the one write
			// tool built without a read service.
			writer := &recorder{}
			got := moved(t, writer, `{"path":"a/b.fx","to":"c/d.fx"}`)

			assert.False(t, got.Failed(), "a file that names itself is served")
			assert.Equal(t, writer.asked.Target.Kind, edit.TargetFile, "the target is the file")
			assert.Equal(t, string(writer.asked.Target.Path), "a/b.fx", "the one named")
			assert.Equal(t, writer.asked.Args[edit.ArgDestination], "c/d.fx", "and where it goes")
		})

		t.Run("refuses a move to nowhere", func(t *testing.T) {
			t.Parallel()
			got := moved(t, &recorder{}, `{"path":"a/b.fx","to":""}`)
			assert.True(t, got.Failed(), "a move with no destination is not a move")
		})

		t.Run("refuses a move that changes nothing", func(t *testing.T) {
			t.Parallel()
			// Planning it would produce a change that renames a file to
			// itself, which the gate would pass and the caller could not
			// tell from a move that happened.
			got := moved(t, &recorder{}, `{"path":"a/b.fx","to":"a/b.fx"}`)
			assert.True(t, got.Failed(), "the file is already there")
			assert.Contains(t, got.Error.Reason, "already", "and the caller is told so")
		})

		t.Run("refuses a path that leaves the workspace", func(t *testing.T) {
			t.Parallel()
			// An absolute path leaks the machine's layout, and a
			// destination climbing out writes outside the root.
			for _, call := range []string{
				`{"path":"../escaped.fx","to":"a.fx"}`,
				`{"path":"a.fx","to":"../escaped.fx"}`,
			} {
				_, err := moving(t, &recorder{}).Execute(t.Context(), json.RawMessage(call))
				assert.HasError(t, err, "a path outside the workspace is refused rather than moved")
			}
		})
	})
}

// moving builds the move tool.
func moving(t *testing.T, writer *recorder) tool.Tool {
	t.Helper()
	built, err := tool.Move(writer)
	assert.NoError(t, err, "the move tool builds from a write path alone")
	return built
}

// moved runs it and decodes what came back.
func moved(t *testing.T, writer *recorder, input string) tool.Written {
	t.Helper()
	result, err := moving(t, writer).Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "a well-formed call reaches the service")

	var out tool.Written
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the answer is JSON a caller can read")
	return out
}
