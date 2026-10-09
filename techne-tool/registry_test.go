// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/tool"
)

// namedTool returns a tool named name whose handler returns a zero output.
func namedTool(t *testing.T, name string) tool.Tool {
	t.Helper()
	built, err := tool.New(name, "PREFER OVER nothing.",
		func(context.Context, greetIn) (greetOut, error) { return greetOut{}, nil })
	assert.NoError(t, err, "the error of New")
	return built
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Add", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a second tool of one name", func(t *testing.T) {
			t.Parallel()
			r := tool.NewRegistry()
			assert.NoError(t, r.Add(greeter(t)), "the error of the first Add")
			assert.HasError(t, r.Add(greeter(t)), "the error of the second Add")
		})

		t.Run("refuses a name that the protocol does not allow", func(t *testing.T) {
			t.Parallel()
			for _, bad := range []string{"", "has space", "has/slash", strings.Repeat("n", 129)} {
				assert.HasError(t, tool.NewRegistry().Add(namedTool(t, bad)), "the error of Add for "+bad)
			}
		})

		t.Run("accepts a name with a dot", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, tool.NewRegistry().Add(namedTool(t, "rename.symbol")), "the error of Add")
		})
	})

	t.Run("Tool", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the tool of a name", func(t *testing.T) {
			t.Parallel()
			r := tool.NewRegistry()
			for _, name := range []string{"a.tool", "b.tool"} {
				assert.NoError(t, r.Add(namedTool(t, name)), "the error of Add for "+name)
			}
			got, found := r.Tool("b.tool")
			assert.True(t, found, "found of b.tool")
			assert.Equal(t, got.Name(), "b.tool", "the name of the tool")
		})

		t.Run("reports false for a name without a tool", func(t *testing.T) {
			t.Parallel()
			_, found := tool.NewRegistry().Tool("a.tool")
			assert.False(t, found, "found of a.tool")
		})
	})

	t.Run("Tools", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the tools in the order of Add", func(t *testing.T) {
			t.Parallel()
			r := tool.NewRegistry()
			for _, name := range []string{"b.tool", "a.tool", "c.tool"} {
				assert.NoError(t, r.Add(namedTool(t, name)), "the error of Add for "+name)
			}
			tools := r.Tools()
			got := make([]string, 0, len(tools))
			for _, one := range tools {
				got = append(got, one.Name())
			}
			assert.Equal(t, got, []string{"b.tool", "a.tool", "c.tool"}, "the names of the tools")
		})
	})
}
