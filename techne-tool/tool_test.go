// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/tool"
)

type greetIn struct {
	Name  string `json:"name"            jsonschema:"who to greet"`
	Times int    `json:"times,omitempty" jsonschema:"how many times"`
}

type greetOut struct {
	Greeting string `json:"greeting"`
}

// greeter returns a tool named greet that returns a greeting of the name of its input, and
// returns an error for an empty name.
func greeter(t *testing.T) tool.Tool {
	t.Helper()
	built, err := tool.New("greet", "PREFER OVER saying hello by hand.",
		func(_ context.Context, in greetIn) (greetOut, error) {
			if in.Name == "" {
				return greetOut{}, errors.New("tool: greet needs a name")
			}
			return greetOut{Greeting: "hello " + in.Name}, nil
		})
	assert.NoError(t, err, "the error of New")
	return built
}

// enum returns the words of the enum of a schema.
func enum(s *jsonschema.Schema) []string {
	out := make([]string, 0, len(s.Enum))
	for _, one := range s.Enum {
		word, _ := one.(string)
		out = append(out, word)
	}
	return out
}

// itemOf returns the schema of one declaration of the output of a read tool.
func itemOf(t *testing.T, out *jsonschema.Schema) map[string]*jsonschema.Schema {
	t.Helper()
	items, has := out.Properties["items"]
	assert.True(t, has, "the items of the output schema")
	assert.NotNil(t, items.Items, "the schema of an item")
	return items.Items.Properties
}

func TestTool(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name and the description", func(t *testing.T) {
			t.Parallel()
			built := greeter(t)
			assert.Equal(t, built.Name(), "greet", "the name")
			assert.Equal(t, built.Description(), "PREFER OVER saying hello by hand.", "the description")
		})
	})

	t.Run("InputSchema", func(t *testing.T) {
		t.Parallel()

		t.Run("describes each field of the input with its tag", func(t *testing.T) {
			t.Parallel()
			got := greeter(t).InputSchema()
			assert.Equal(t, got.Properties["name"].Description, "who to greet", "the description of name")
			assert.Equal(t, got.Required, []string{"name"}, "the required fields")
		})

		t.Run("lists the words of each word field", func(t *testing.T) {
			t.Parallel()
			outline, err := tool.Outline(serving())
			assert.NoError(t, err, "the error of Outline")
			relations, err := tool.Relations(serving(), serving())
			assert.NoError(t, err, "the error of Relations")
			in := outline.InputSchema().Properties
			assert.Equal(t, enum(in["kind"]), words(sema.Kinds()), "the enum of kind")
			assert.Equal(t, enum(in["detail"]), words(tool.Levels()), "the enum of detail")
			assert.Equal(t, enum(in["include"].Items), words(tool.Includes()), "the enum of include")
			assert.Equal(t, enum(in["preferred_fidelity"]), words(trust.Fidelities()), "the enum of preferred_fidelity")
			assert.Equal(t, enum(relations.InputSchema().Properties["relation"]), words(sema.RelationKinds()),
				"the enum of relation")
		})

		t.Run("keeps the description of a word field", func(t *testing.T) {
			t.Parallel()
			outline, err := tool.Outline(serving())
			assert.NoError(t, err, "the error of Outline")
			assert.Equal(t, outline.InputSchema().Properties["kind"].Description, "keep the declarations of one kind",
				"the description of kind")
		})
	})

	t.Run("OutputSchema", func(t *testing.T) {
		t.Parallel()

		answering := func(t *testing.T) tool.Tool {
			t.Helper()
			built, err := tool.New("t", "d",
				func(context.Context, struct{}) (tool.Answer, error) { return tool.Answer{}, nil })
			assert.NoError(t, err, "the error of New")
			return built
		}

		t.Run("describes a kind as a word of sema.Kinds", func(t *testing.T) {
			t.Parallel()
			kind := itemOf(t, answering(t).OutputSchema())["kind"]
			assert.Equal(t, kind.Type, "string", "the type of kind")
			assert.Equal(t, enum(kind), words(sema.Kinds()), "the enum of kind")
		})

		t.Run("describes a visibility as a word of sema.Visibilities", func(t *testing.T) {
			t.Parallel()
			visibility := itemOf(t, answering(t).OutputSchema())["visibility"]
			assert.Equal(t, visibility.Type, "string", "the type of visibility")
			assert.Equal(t, enum(visibility), words(sema.Visibilities()), "the enum of visibility")
		})

		t.Run("describes the members by a reference to the schema of an item", func(t *testing.T) {
			t.Parallel()
			members := itemOf(t, answering(t).OutputSchema())["members"]
			assert.Equal(t, members.Items.Ref, "#/properties/items/items", "the reference of members")
		})
	})

	t.Run("Execute", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the output of the handler", func(t *testing.T) {
			t.Parallel()
			got, err := greeter(t).Execute(t.Context(), json.RawMessage(`{"name":"world"}`))
			assert.NoError(t, err, "the error of Execute")
			assert.Equal(t, string(got.Payload), `{"greeting":"hello world"}`, "the payload")
			assert.False(t, got.Failed, "Failed of the result")
		})

		t.Run("returns an error for input that does not decode", func(t *testing.T) {
			t.Parallel()
			_, err := greeter(t).Execute(t.Context(), json.RawMessage(`{"name":`))
			assert.HasError(t, err, "the error of Execute")
			assert.HasPrefix(t, err.Error(), "tool: ", "the error of Execute")
		})

		t.Run("returns an error for a field that the input schema does not declare", func(t *testing.T) {
			t.Parallel()
			_, err := greeter(t).Execute(t.Context(), json.RawMessage(`{"name":"world","query":"x","Times":2}`))
			assert.HasError(t, err, "the error of Execute")
			assert.Equal(t, err.Error(), `tool: greet does not take "Times", "query". It takes name, times`,
				"the error of Execute")
		})

		t.Run("returns an error for an input without a required field", func(t *testing.T) {
			t.Parallel()
			_, err := greeter(t).Execute(t.Context(), json.RawMessage(`{"times":2}`))
			assert.HasError(t, err, "the error of Execute")
			assert.Equal(t, err.Error(), `tool: greet needs "name"`, "the error of Execute")
		})

		t.Run("reads an absent input as an empty object", func(t *testing.T) {
			t.Parallel()
			built, err := tool.New("t", "d", func(context.Context, struct{}) (greetOut, error) {
				return greetOut{Greeting: "hello"}, nil
			})
			assert.NoError(t, err, "the error of New")
			got, err := built.Execute(t.Context(), nil)
			assert.NoError(t, err, "the error of Execute")
			assert.Equal(t, string(got.Payload), `{"greeting":"hello"}`, "the payload")
		})

		t.Run("returns the error of the handler", func(t *testing.T) {
			t.Parallel()
			_, err := greeter(t).Execute(t.Context(), json.RawMessage(`{"name":""}`))
			assert.HasError(t, err, "the error of Execute")
			assert.Equal(t, err.Error(), "tool: greet needs a name", "the error of Execute")
		})

		t.Run("returns a failed result for an output that reports a failure", func(t *testing.T) {
			t.Parallel()
			built, err := tool.New("t", "d", func(context.Context, struct{}) (tool.Answer, error) {
				return tool.Answer{Error: &tool.Failure{Code: "refused", Reason: "no"}}, nil
			})
			assert.NoError(t, err, "the error of New")
			got, err := built.Execute(t.Context(), json.RawMessage(`{}`))
			assert.NoError(t, err, "the error of Execute")
			assert.True(t, got.Failed, "Failed of the result")
			assert.Equal(t, got.Rendered, "refused: no\n", "the render of the result")
		})
	})
}
