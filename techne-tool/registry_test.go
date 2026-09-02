// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/tool"
)

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Add", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses two tools with one name", func(t *testing.T) {
			t.Parallel()
			// A name is what an agent routes on, so two tools sharing one
			// make the choice undefined.
			r := tool.NewRegistry()
			assert.NoError(t, r.Add(greeter(t)), "the first tool registers")
			assert.HasError(t, r.Add(greeter(t)), "a second tool cannot take a name already used")
		})

		t.Run("refuses a name the protocol does not allow", func(t *testing.T) {
			t.Parallel()
			// A client that rejects the name drops the tool, and the
			// server looks like it never offered it.
			for _, bad := range []string{"", "has space", "has/slash", strings.Repeat("n", 129)} {
				built, err := tool.New(bad, "PREFER OVER nothing.",
					func(_ context.Context, in greetIn) (greetOut, error) { return greetOut{}, nil })
				assert.NoError(t, err, "the handler is fine; the name is what is wrong")
				assert.HasError(t, tool.NewRegistry().Add(built),
					"a name outside the protocol's character set or length is refused here, not by a client")
			}
		})

		t.Run("accepts a dotted name", func(t *testing.T) {
			t.Parallel()
			// The specification allows dots and gives admin.tools.list
			// as an example, which is what family.subject relies on.
			built, err := tool.New("rename.symbol", "PREFER OVER edit and grep.",
				func(_ context.Context, in greetIn) (greetOut, error) { return greetOut{}, nil })
			assert.NoError(t, err, "a handler over serialisable types produces a tool")
			assert.NoError(t, tool.NewRegistry().Add(built), "family.subject is a valid tool name")
		})
	})

	t.Run("Tools", func(t *testing.T) {
		t.Parallel()

		t.Run("answers in a stable order", func(t *testing.T) {
			t.Parallel()
			// A client caches the tool list, and a list that reorders
			// itself invalidates that cache for no reason.
			r := tool.NewRegistry()
			assert.NoError(t, r.Add(greeter(t)), "the case needs a tool registered")
			assert.Equal(t, r.Tools()[0].Name(), r.Tools()[0].Name(), "two reads answer identically")
		})
	})
}
