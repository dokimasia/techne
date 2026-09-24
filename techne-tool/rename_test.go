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

// renamed runs the rename.symbol tool over [addressable] and writer with input, and decodes
// the output.
func renamed(t *testing.T, writer *recorder, input string) tool.Written {
	t.Helper()
	built, err := tool.Rename(addressable(), writer)
	assert.NoError(t, err, "the error of Rename")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.Written
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out
}

func TestRename(t *testing.T) {
	t.Parallel()

	t.Run("Rename", func(t *testing.T) {
		t.Parallel()

		t.Run("starts its description with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Rename(addressable(), &recorder{})
			assert.NoError(t, err, "the error of Rename")
			assert.HasPrefix(t, built.Description(), "PREFER OVER ", "the description")
		})

		t.Run("points the write path at the span with the new name", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			got := renamed(t, writer, `{"scope":"a.fx","name":"Store","new_name":"Vault"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, writer.asked.Target.Kind, edit.TargetSpan, "the kind of the target")
			assert.Equal(t, writer.asked.Args[edit.ArgNewName], "Vault", "the new name of the request")
		})

		t.Run("refuses an empty new name", func(t *testing.T) {
			t.Parallel()
			got := renamed(t, &recorder{}, `{"scope":"a.fx","name":"Store","new_name":""}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Equal(t, got.Error.Code, "refused", "the code of the failure")
		})

		t.Run("refuses an ambiguous name with the site of each declaration", func(t *testing.T) {
			t.Parallel()
			got := renamed(t, &recorder{}, `{"scope":"a.fx","name":"Get","new_name":"Fetch"}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Contains(t, got.Error.Reason, "a.fx:4", "the reason of the failure")
		})

		t.Run("previews a request without dry_run", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			renamed(t, writer, `{"scope":"a.fx","name":"Store","new_name":"Vault"}`)
			assert.True(t, writer.asked.DryRun, "DryRun of the request")
		})
	})
}
