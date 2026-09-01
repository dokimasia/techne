// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/tool"
)

type greetIn struct {
	Name  string `json:"name"            jsonschema:"who to greet"`
	Times int    `json:"times,omitempty" jsonschema:"how many times"`
}

type greetOut struct {
	Greeting string `json:"greeting"`
}

func greeter(t *testing.T) tool.Tool {
	t.Helper()
	built, err := tool.New("greet", "PREFER OVER saying hello by hand.",
		func(_ context.Context, in greetIn) (greetOut, error) {
			if in.Name == "" {
				return greetOut{}, errors.New("tool: greet needs a name")
			}
			return greetOut{Greeting: "hello " + in.Name}, nil
		})
	assert.NoError(t, err, "a handler over serialisable types produces a tool")
	return built
}

func TestTool(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("derives the input schema from the handler's own type", func(t *testing.T) {
			t.Parallel()
			// The schema is the agent-facing contract. Deriving it means
			// a field added to the Go type cannot go undocumented.
			schema := greeter(t).InputSchema()
			assert.NotNil(t, schema, "a tool an agent calls declares what it takes")
			encoded, err := json.Marshal(schema)
			assert.NoError(t, err, "a schema that cannot be sent cannot be advertised")
			assert.Contains(t, string(encoded), "name", "every field of the input type reaches the schema")
			assert.Contains(t, string(encoded), "who to greet", "the field's own tag carries its description")
		})

		t.Run("derives the output schema too", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(greeter(t).OutputSchema())
			assert.NoError(t, err, "a schema that cannot be sent cannot be advertised")
			assert.Contains(t, string(encoded), "greeting", "a caller validates the result against this")
		})

		t.Run("keeps the name and description it was given", func(t *testing.T) {
			t.Parallel()
			built := greeter(t)
			assert.Equal(t, built.Name(), "greet", "the name is what an agent routes on")
			assert.HasPrefix(t, built.Description(), "PREFER OVER ",
				"a description that does not name the built-in it replaces leaves the tool unchosen")
		})
	})

	t.Run("Execute", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes, runs and encodes", func(t *testing.T) {
			t.Parallel()
			got, err := greeter(t).Execute(t.Context(), json.RawMessage(`{"name":"world"}`))
			assert.NoError(t, err, "a well-formed call reaches the handler")
			assert.Contains(t, string(got.Payload), "hello world", "the handler's result is what comes back")
			assert.False(t, got.Failed, "an operation that did what was asked is not a failure")
		})

		t.Run("refuses input it cannot decode", func(t *testing.T) {
			t.Parallel()
			// Calling the handler with a zero value would run the
			// operation on arguments nobody sent.
			_, err := greeter(t).Execute(t.Context(), json.RawMessage(`{"name":`))
			assert.HasError(t, err, "malformed input is refused rather than defaulted")
			assert.HasPrefix(t, err.Error(), "tool: ", "an error names the package that refused")
		})

		t.Run("passes a handler's own failure back", func(t *testing.T) {
			t.Parallel()
			_, err := greeter(t).Execute(t.Context(), json.RawMessage(`{}`))
			assert.HasError(t, err, "a handler that refuses is not reported as success")
		})
	})
}
