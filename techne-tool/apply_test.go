// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/tool"
)

// handle is a handle that a preview returns.
const handle = "0123456789abcdef0123456789abcdef"

// committer is a write path that records the handle it receives. It refuses the commit with
// the reason refuses, as for a handle without a preview, when refuses is set, and otherwise
// writes the rename of Store in a.fx that a preview about the directory pkg planned.
type committer struct {
	asked   string
	refuses string
}

func (c *committer) Commit(_ context.Context, handle string) (edit.Outcome, error) {
	c.asked = handle
	if c.refuses != "" {
		return edit.Outcome{Status: trust.Refused, Reason: c.refuses}, nil
	}
	return edit.Outcome{
		Operation: edit.RenameSymbol,
		Request:   edit.Request{Operation: edit.RenameSymbol, Scope: "pkg", Language: "fx", Subject: "Store"},
		Status:    trust.OK,
		Applied:   true,
		Changed:   []source.Path{"a.fx"},
		Rewrites:  []edit.Rewrite{{Path: "a.fx", Line: 1, Was: "Store", Now: "Vault"}},
		Provenance: trust.Provenance{
			Engine: "server", Fidelity: trust.Resolved, Completeness: trust.ScopeTotal,
		},
	}, nil
}

// applied runs the apply.change tool over writer with input, and decodes the output.
func applied(t *testing.T, writer *committer, input string) tool.Written {
	t.Helper()
	built, err := tool.Apply(writer)
	assert.NoError(t, err, "the error of Apply")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.Written
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out
}

func TestApply(t *testing.T) {
	t.Parallel()

	call := `{"handle":"` + handle + `"}`

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("starts its description with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Apply(&committer{})
			assert.NoError(t, err, "the error of Apply")
			assert.HasPrefix(t, built.Description(), "PREFER OVER ", "the description")
		})

		t.Run("sends the handle to the write path unchanged", func(t *testing.T) {
			t.Parallel()
			writer := &committer{}
			got := applied(t, writer, call)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, writer.asked, handle, "the handle of the commit")
			assert.True(t, got.Applied, "Applied of the output")
		})

		t.Run("returns the target of the preview", func(t *testing.T) {
			t.Parallel()
			got := applied(t, &committer{}, call)
			assert.Equal(t, got.Target, "Store", "the target of the output")
		})

		t.Run("returns the scope of the preview", func(t *testing.T) {
			t.Parallel()
			got := applied(t, &committer{}, call)
			assert.Equal(t, got.Scope, tool.Scope{Language: "fx", Unit: "pkg"}, "the scope of the output")
		})

		t.Run("returns the handle as the target without a preview", func(t *testing.T) {
			t.Parallel()
			got := applied(t, &committer{refuses: "no preview has that handle"}, call)
			assert.Equal(t, got.Target, handle, "the target of the output")
			assert.Equal(t, got.Scope, tool.Scope{}, "the scope of the output")
		})

		t.Run("returns the operation of the preview", func(t *testing.T) {
			t.Parallel()
			got := applied(t, &committer{}, call)
			assert.Equal(t, got.Operation, "rename.symbol", "the operation of the output")
		})

		t.Run("refuses an empty handle", func(t *testing.T) {
			t.Parallel()
			got := applied(t, &committer{}, `{"handle":""}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Equal(t, got.Error.Code, "refused", "the code of the failure")
		})

		t.Run("returns the refusal of the write path with its reason", func(t *testing.T) {
			t.Parallel()
			got := applied(t, &committer{refuses: "no preview has that handle"}, call)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Equal(t, got.Error.Reason, "no preview has that handle", "the reason of the failure")
		})
	})
}
