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

func TestApply(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("names the built-in it replaces", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, applying(t, &committer{}).Description(), "PREFER OVER ",
				"an agent previews and asks again unless told why not to")
		})

		t.Run("sends the handle and nothing else", func(t *testing.T) {
			t.Parallel()
			// The changes stay where they were computed. A rename over
			// thirty sites is kilobytes a caller would have to reproduce
			// exactly, and reproducing bytes exactly is the least
			// reliable thing a model does.
			writer := &committer{}
			got := applied(t, writer, `{"handle":"0123456789abcdef0123456789abcdef"}`)

			assert.False(t, got.Failed(), "a handle the service holds is applied")
			assert.Equal(t, writer.asked, "0123456789abcdef0123456789abcdef",
				"the handle reaches the service unchanged")
			assert.True(t, got.Applied, "and the change was written")
			assert.Equal(t, got.Target, "a.fx",
				"what is reported back is the file that changed, not how it was asked for")
		})

		t.Run("reports the operation the preview was for", func(t *testing.T) {
			t.Parallel()
			// A caller reading a result should not have to remember what
			// it previewed.
			got := applied(t, &committer{}, `{"handle":"0123456789abcdef0123456789abcdef"}`)
			assert.Equal(t, got.Operation, "rename.symbol",
				"the change says which operation it was, not that it was applied by handle")
		})

		t.Run("refuses a call with no handle", func(t *testing.T) {
			t.Parallel()
			got := applied(t, &committer{}, `{"handle":""}`)
			assert.True(t, got.Failed(), "there is nothing to apply")
			assert.Equal(t, got.Error.Code, "refused", "which the caller can correct")
		})

		t.Run("passes back a refusal rather than raising it", func(t *testing.T) {
			t.Parallel()
			// A stale handle is something a caller corrects by
			// previewing again.
			got := applied(t, &committer{refuses: "no preview is held under that handle"},
				`{"handle":"0123456789abcdef0123456789abcdef"}`)
			assert.True(t, got.Failed(), "a handle nobody holds is refused")
			assert.Contains(t, got.Error.Reason, "no preview is held", "with the reason")
		})
	})
}

// committer is a write path that records the handle it was given.
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
		Status:    trust.OK,
		Applied:   true,
		Changed:   []source.Path{"a.fx"},
		Rewrites:  []edit.Rewrite{{Path: "a.fx", Line: 1, Was: "Store", Now: "Vault"}},
		Provenance: trust.Provenance{
			Engine: "server", Fidelity: trust.Resolved, Completeness: trust.ScopeTotal,
		},
	}, nil
}

// applying builds the apply tool.
func applying(t *testing.T, writer *committer) tool.Tool {
	t.Helper()
	built, err := tool.Apply(writer)
	assert.NoError(t, err, "the apply tool builds from a write path alone")
	return built
}

// applied runs it and decodes what came back.
func applied(t *testing.T, writer *committer, input string) tool.Written {
	t.Helper()
	result, err := applying(t, writer).Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "a well-formed call reaches the service")

	var out tool.Written
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the answer is JSON a caller can read")
	return out
}
