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

func TestRename(t *testing.T) {
	t.Parallel()

	t.Run("Rename", func(t *testing.T) {
		t.Parallel()

		t.Run("names the built-in it replaces", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, renaming(t, &recorder{}).Description(), "PREFER OVER ",
				"an agent reaches for find-and-replace unless told why not to")
		})

		t.Run("points the planner at one declaration", func(t *testing.T) {
			t.Parallel()
			// A name and a kind do not pick out one declaration, so the
			// tool resolves it and says which by position.
			writer := &recorder{}
			got := renamed(t, writer, `{"scope":"a.fx","name":"Store","new_name":"Vault"}`)

			assert.False(t, got.Failed(), "a name one declaration answers to is served")
			assert.Equal(t, writer.asked.Target.Kind, edit.TargetSpan, "by position")
			assert.Equal(t, writer.asked.Args[edit.ArgNewName], "Vault", "and carries the new name")
		})

		t.Run("refuses a call with nothing to rename it to", func(t *testing.T) {
			t.Parallel()
			got := renamed(t, &recorder{}, `{"scope":"a.fx","name":"Store","new_name":""}`)
			assert.True(t, got.Failed(), "a rename to nothing is not a rename")
			assert.Equal(t, got.Error.Code, "refused", "which the caller can correct")
		})

		t.Run("answers an ambiguous name with the candidates", func(t *testing.T) {
			t.Parallel()
			got := renamed(t, &recorder{}, `{"scope":"a.fx","name":"Get","new_name":"Fetch"}`)
			assert.True(t, got.Failed(), "two candidates are not one declaration")
			assert.Contains(t, got.Error.Reason, "a.fx:4", "and the refusal says where they are")
		})

		t.Run("previews when the caller says nothing", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			renamed(t, writer, `{"scope":"a.fx","name":"Store","new_name":"Vault"}`)
			assert.True(t, writer.asked.DryRun, "writing by default would make a typo a change")
		})
	})
}

// renaming builds the rename tool over the fixture.
func renaming(t *testing.T, writer *recorder) tool.Tool {
	t.Helper()
	built, err := tool.Rename(addressable(), writer)
	assert.NoError(t, err, "the rename tool builds from a read service and a write path")
	return built
}

// renamed runs it and decodes what came back.
func renamed(t *testing.T, writer *recorder, input string) tool.Written {
	t.Helper()
	result, err := renaming(t, writer).Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "a well-formed call reaches the service")

	var out tool.Written
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the answer is JSON a caller can read")
	return out
}
