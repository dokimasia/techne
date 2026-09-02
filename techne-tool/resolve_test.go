// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/tool"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("names the built-in it replaces", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, resolving(t).Description(), "PREFER OVER ",
				"an agent guesses from the surrounding text unless told why not to")
		})

		t.Run("answers what the name at a position denotes", func(t *testing.T) {
			t.Parallel()
			got := resolved(t, `{"scope":"a.fx","line":3,"column":10}`)
			_, failed := got["error"]
			assert.False(t, failed, "a position in a served file is answered about")
			assert.NotEmpty(t, got["items"], "and denotes something")
		})

		t.Run("counts lines and columns from one", func(t *testing.T) {
			t.Parallel()
			// Spans count from zero, editors count from one, and a
			// caller pasting a position out of a compiler message should
			// not have to subtract.
			for _, at := range []string{
				`{"scope":"a.fx","line":0,"column":1}`,
				`{"scope":"a.fx","line":1,"column":0}`,
			} {
				_, err := resolving(t).Execute(t.Context(), json.RawMessage(at))
				assert.HasError(t, err, "a coordinate counted from zero is a mistake, not a position")
			}
		})

		t.Run("says a language is not served rather than answering emptily", func(t *testing.T) {
			t.Parallel()
			got := resolved(t, `{"scope":"notes.md","line":1,"column":1}`)
			assert.Equal(t, got["error"].(map[string]any)["code"], "unsupported",
				"no language claims a markdown file")
		})

		t.Run("never claims a parser proves absence", func(t *testing.T) {
			t.Parallel()
			// A parser matched text, so several items mean several names
			// matched rather than that the name is genuinely ambiguous.
			got := resolved(t, `{"scope":"a.fx","line":3,"column":10}`)
			provenance := got["provenance"].(map[string]any)
			assert.Equal(t, provenance["supportsNegativeClaim"], false,
				"what a name denotes at this tier is what matched, not what it means")
		})
	})
}

// resolving builds the resolve tool over the fixture.
func resolving(t *testing.T) tool.Tool {
	t.Helper()
	built, err := tool.Resolve(addressable())
	assert.NoError(t, err, "the resolve tool builds from a read service")
	return built
}

// resolved runs it and decodes what came back.
func resolved(t *testing.T, input string) map[string]any {
	t.Helper()
	result, err := resolving(t).Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "a well-formed call reaches the service")

	var out map[string]any
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the answer is JSON a caller can read")
	return out
}
