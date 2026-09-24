// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/tool"
)

// TestDoc covers the claims of the package comment about the tools and the budget.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Fit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a fitted answer unchanged", func(t *testing.T) {
			t.Parallel()
			budget := tool.Budget{MaxTokens: 300}
			once := tool.Fit(many(400), budget)
			assert.Equal(t, tool.Fit(once, budget), once, "the answer fitted twice")
		})

		t.Run("leaves the evidence of the answer unchanged", func(t *testing.T) {
			t.Parallel()
			full := many(400)
			got := tool.Fit(full, tool.Budget{MaxTokens: 300})
			assert.Equal(t, got.Failed(), full.Failed(), "Failed of the answer")
			assert.Equal(t, got.Provenance.Completeness, full.Provenance.Completeness, "the completeness")
			assert.Equal(t, got.Provenance.SupportsNegativeClaim, full.Provenance.SupportsNegativeClaim,
				"the negative claim")
		})
	})

	t.Run("Description", func(t *testing.T) {
		t.Parallel()

		t.Run("starts with PREFER OVER for every tool", func(t *testing.T) {
			t.Parallel()
			for _, one := range every(t) {
				assert.True(t, strings.HasPrefix(one.Description(), "PREFER OVER "), "the description of "+one.Name())
			}
		})
	})
}

// every returns one of each tool of the package.
func every(t *testing.T) []tool.Tool {
	t.Helper()
	over := addressable()
	var out []tool.Tool
	for _, build := range []func() (tool.Tool, error){
		func() (tool.Tool, error) { return tool.Outline(over) },
		func() (tool.Tool, error) { return tool.Search(over) },
		func() (tool.Tool, error) { return tool.Resolve(over) },
		func() (tool.Tool, error) { return tool.Relations(over, over) },
		func() (tool.Tool, error) { return tool.Verify(over) },
		func() (tool.Tool, error) { return tool.Capabilities(catalogue{}) },
		func() (tool.Tool, error) { return tool.Document(over, &recorder{}) },
		func() (tool.Tool, error) { return tool.Rename(over, &recorder{}) },
		func() (tool.Tool, error) { return tool.Move(&recorder{}) },
		func() (tool.Tool, error) { return tool.Extract(&recorder{}) },
		func() (tool.Tool, error) { return tool.Apply(&committer{}) },
	} {
		built, err := build()
		assert.NoError(t, err, "the error of a tool constructor")
		out = append(out, built)
	}
	return out
}
