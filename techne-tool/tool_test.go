// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
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

// aliasedIn is an input that takes its name under the aliases who and whom too.
type aliasedIn struct {
	Name string `json:"name" alias:"who,whom"`
}

// aliasing returns a tool named greet that returns a greeting of the name of an aliasedIn.
func aliasing(t *testing.T) tool.Tool {
	t.Helper()
	built, err := tool.New("greet", "PREFER OVER saying hello by hand.",
		func(_ context.Context, in aliasedIn) (greetOut, error) {
			return greetOut{Greeting: "hello " + in.Name}, nil
		})
	assert.NoError(t, err, "the error of New")
	return built
}

// placedIn is an input with a required and an optional integer.
type placedIn struct {
	Line  int `json:"line"`
	Limit int `json:"limit,omitempty"`
}

// placing returns a tool named place that returns its input.
func placing(t *testing.T) tool.Tool {
	t.Helper()
	built, err := tool.New("place", "PREFER OVER counting by hand.",
		func(_ context.Context, in placedIn) (placedIn, error) { return in, nil })
	assert.NoError(t, err, "the error of New")
	return built
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

		t.Run("returns an error for an alias that is the name of a field", func(t *testing.T) {
			t.Parallel()
			type clashing struct {
				Name  string `json:"name"            alias:"times"`
				Times int    `json:"times,omitempty"`
			}
			_, err := tool.New("t", "d", func(context.Context, clashing) (greetOut, error) { return greetOut{}, nil })
			assert.HasError(t, err, "the error of New")
			assert.Equal(t, err.Error(), `tool: "t" input: the alias "times" of "name" is the name of a field`,
				"the error of New")
		})

		t.Run("returns an error for an alias of two fields", func(t *testing.T) {
			t.Parallel()
			type doubled struct {
				First  string `json:"first"  alias:"x"`
				Second string `json:"second" alias:"x"`
			}
			_, err := tool.New("t", "d", func(context.Context, doubled) (greetOut, error) { return greetOut{}, nil })
			assert.HasError(t, err, "the error of New")
			assert.Equal(t, err.Error(), `tool: "t" input: "x" is the alias of "first" and of "second"`,
				"the error of New")
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

		t.Run("lists the words of each word field but kind", func(t *testing.T) {
			t.Parallel()
			outline, err := tool.Outline(serving())
			assert.NoError(t, err, "the error of Outline")
			relations, err := tool.Relations(serving(), serving())
			assert.NoError(t, err, "the error of Relations")
			in := outline.InputSchema().Properties
			assert.Empty(t, enum(in["kind"]), "the enum of kind")
			assert.Equal(t, in["kind"].Type, "string", "the type of kind")
			assert.Equal(t, enum(in["detail"]), words(tool.Levels()), "the enum of detail")
			assert.Equal(t, enum(in["include"].Items), words(tool.Includes()), "the enum of include")
			assert.Equal(t, enum(in["preferred_fidelity"]), words(trust.Fidelities()), "the enum of preferred_fidelity")
			assert.Equal(t, enum(relations.InputSchema().Properties["relation"]), words(sema.RelationKinds()),
				"the enum of relation")
		})

		t.Run("lists no alias of a field", func(t *testing.T) {
			t.Parallel()
			got := aliasing(t).InputSchema()
			assert.Equal(t, got.PropertyOrder, []string{"name"}, "the properties of the input schema")
		})

		t.Run("gives a required integer the minimum 1", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, *placing(t).InputSchema().Properties["line"].Minimum, 1.0, "the minimum of line")
		})

		t.Run("gives an optional integer the minimum 0", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, *placing(t).InputSchema().Properties["limit"].Minimum, 0.0, "the minimum of limit")
		})

		t.Run("keeps the description of a word field", func(t *testing.T) {
			t.Parallel()
			outline, err := tool.Outline(serving())
			assert.NoError(t, err, "the error of Outline")
			assert.Equal(t, outline.InputSchema().Properties["detail"].Description,
				"signatures for a file and names for a directory by default", "the description of detail")
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

		t.Run("returns the time that the engines of the call waited", func(t *testing.T) {
			t.Parallel()
			waiting, err := tool.New("wait", "PREFER OVER waiting by hand.",
				func(ctx context.Context, in greetIn) (greetOut, error) {
					done := engine.Waiting(ctx)
					time.Sleep(time.Millisecond)
					done()
					return greetOut{Greeting: in.Name}, nil
				})
			assert.NoError(t, err, "the error of New")
			got, err := waiting.Execute(t.Context(), json.RawMessage(`{"name":"world"}`))
			assert.NoError(t, err, "the error of Execute")
			assert.True(t, got.Waited >= time.Millisecond, "the time that the handler waited")
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

		aliases := []struct {
			name string
			give string
			want string
		}{
			{name: "takes a field under its alias", give: `{"who":"world"}`, want: `{"greeting":"hello world"}`},
			{
				name: "takes a field under its second alias",
				give: `{"whom":"world"}`,
				want: `{"greeting":"hello world"}`,
			},
			{
				name: "takes a field over its alias when the input names both",
				give: `{"name":"world","who":"moon"}`,
				want: `{"greeting":"hello world"}`,
			},
		}
		for _, tt := range aliases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, err := aliasing(t).Execute(t.Context(), json.RawMessage(tt.give))
				assert.NoError(t, err, "the error of Execute")
				assert.Equal(t, string(got.Payload), tt.want, "the payload")
			})
		}

		t.Run("returns an error for an input without a required field", func(t *testing.T) {
			t.Parallel()
			_, err := greeter(t).Execute(t.Context(), json.RawMessage(`{"times":2}`))
			assert.HasError(t, err, "the error of Execute")
			assert.Equal(t, err.Error(), `tool: greet needs "name"`, "the error of Execute")
		})

		t.Run("returns an error for an optional number below 0", func(t *testing.T) {
			t.Parallel()
			_, err := placing(t).Execute(t.Context(), json.RawMessage(`{"line":1,"limit":-1}`))
			assert.HasError(t, err, "the error of Execute")
			assert.Equal(t, err.Error(), `tool: place takes "limit" of at least 0, not -1`, "the error of Execute")
		})

		t.Run("returns an error for a required number below 1", func(t *testing.T) {
			t.Parallel()
			_, err := placing(t).Execute(t.Context(), json.RawMessage(`{"line":0}`))
			assert.HasError(t, err, "the error of Execute")
			assert.Equal(t, err.Error(), `tool: place takes "line" of at least 1, not 0`, "the error of Execute")
		})

		t.Run("returns the output for numbers at their minimum", func(t *testing.T) {
			t.Parallel()
			got, err := placing(t).Execute(t.Context(), json.RawMessage(`{"line":1,"limit":0}`))
			assert.NoError(t, err, "the error of Execute")
			assert.Equal(t, string(got.Payload), `{"line":1}`, "the payload")
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
