// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/tool"
)

// about is the description of the working directory in the tools of the tests.
const about = "the directory of the call"

// elsewhereCall is what a call of an [tool.Elsewhere] received.
type elsewhereCall struct {
	wd, name string
	input    map[string]any
}

// directed returns greeter with the working directory, and the calls of its [tool.Elsewhere],
// which returns result.
func directed(t *testing.T, result tool.Result) (tool.Tool, *[]elsewhereCall) {
	t.Helper()
	var calls []elsewhereCall
	elsewhere := func(_ context.Context, wd, name string, input json.RawMessage) (tool.Result, error) {
		var decoded map[string]any
		assert.NoError(t, json.Unmarshal(input, &decoded), "the input of Elsewhere")
		calls = append(calls, elsewhereCall{wd: wd, name: name, input: decoded})
		return result, nil
	}
	built, err := tool.Directed(greeter(t), about, elsewhere)
	assert.NoError(t, err, "the error of Directed")
	return built, &calls
}

// greeting returns the greeting of the payload of a result of greeter.
func greeting(t *testing.T, got tool.Result) string {
	t.Helper()
	var out struct {
		Greeting string `json:"greeting"`
	}
	assert.NoError(t, json.Unmarshal(got.Payload, &out), "the payload of the result")
	return out.Greeting
}

// fieldless is the input of a tool that takes no field.
type fieldless struct{}

// rooted is the input of a tool that takes a working directory of its own.
type rooted struct {
	Wd string `json:"wd"`
}

func TestDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Directed", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a tool that takes wd", func(t *testing.T) {
			t.Parallel()
			taking, err := tool.New("take", "PREFER OVER nothing.",
				func(context.Context, rooted) (greetOut, error) { return greetOut{}, nil })
			assert.NoError(t, err, "the error of New")
			_, err = tool.Directed(taking, about, nil)
			assert.HasError(t, err, "the error of Directed")
			assert.Equal(t, err.Error(), `tool: "take" already takes "wd"`, "the error of Directed")
		})

		t.Run("adds wd to a tool that takes no field", func(t *testing.T) {
			t.Parallel()
			bare, err := tool.New("bare", "PREFER OVER nothing.",
				func(context.Context, fieldless) (greetOut, error) { return greetOut{}, nil })
			assert.NoError(t, err, "the error of New")
			got, err := tool.Directed(bare, about, nil)
			assert.NoError(t, err, "the error of Directed")
			assert.Equal(t, got.InputSchema().PropertyOrder, []string{tool.WorkingDirectory},
				"the fields of the input schema")
			assert.NotNil(t, got.InputSchema().Properties[tool.WorkingDirectory], "the schema of wd")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name of the tool", func(t *testing.T) {
			t.Parallel()
			got, _ := directed(t, tool.Result{})
			assert.Equal(t, got.Name(), "greet", "the name of the tool")
		})
	})

	t.Run("InputSchema", func(t *testing.T) {
		t.Parallel()

		t.Run("declares wd as a string with its description", func(t *testing.T) {
			t.Parallel()
			got, _ := directed(t, tool.Result{})
			wd := got.InputSchema().Properties[tool.WorkingDirectory]
			assert.Equal(t, wd.Type, "string", "the type of wd")
			assert.Equal(t, wd.Description, about, "the description of wd")
		})

		t.Run("lists wd after the fields of the tool", func(t *testing.T) {
			t.Parallel()
			got, _ := directed(t, tool.Result{})
			assert.Equal(t, got.InputSchema().PropertyOrder, []string{"name", "times", tool.WorkingDirectory},
				"the fields of the input schema")
		})

		t.Run("leaves the input schema of the tool without wd", func(t *testing.T) {
			t.Parallel()
			inner := greeter(t)
			_, err := tool.Directed(inner, about, nil)
			assert.NoError(t, err, "the error of Directed")
			_, declared := inner.InputSchema().Properties[tool.WorkingDirectory]
			assert.False(t, declared, "wd in the schema of the tool")
			assert.Equal(t, inner.InputSchema().PropertyOrder, []string{"name", "times"},
				"the fields of the schema of the tool")
		})
	})

	t.Run("Execute", func(t *testing.T) {
		t.Parallel()

		t.Run("runs the tool for a call without wd", func(t *testing.T) {
			t.Parallel()
			got, calls := directed(t, tool.Result{})
			result, err := got.Execute(t.Context(), json.RawMessage(`{"name":"ann"}`))
			assert.NoError(t, err, "the error of Execute")
			assert.Equal(t, greeting(t, result), "hello ann", "the greeting")
			assert.Empty(t, *calls, "the calls of Elsewhere")
		})

		t.Run("runs the tool for an empty wd", func(t *testing.T) {
			t.Parallel()
			got, calls := directed(t, tool.Result{})
			result, err := got.Execute(t.Context(), json.RawMessage(`{"name":"ann","wd":""}`))
			assert.NoError(t, err, "the error of Execute")
			assert.Equal(t, greeting(t, result), "hello ann", "the greeting")
			assert.Empty(t, *calls, "the calls of Elsewhere")
		})

		t.Run("runs elsewhere with the input of a call without its wd", func(t *testing.T) {
			t.Parallel()
			got, calls := directed(t, tool.Result{Payload: json.RawMessage(`{"greeting":"far"}`)})
			result, err := got.Execute(t.Context(), json.RawMessage(`{"name":"ann","wd":"/projects/b"}`))
			assert.NoError(t, err, "the error of Execute")
			assert.Equal(t, greeting(t, result), "far", "the greeting")
			assert.Equal(t, *calls, []elsewhereCall{
				{wd: "/projects/b", name: "greet", input: map[string]any{"name": "ann"}},
			}, "the calls of Elsewhere")
		})

		t.Run("returns an error for a wd that is not a string", func(t *testing.T) {
			t.Parallel()
			got, calls := directed(t, tool.Result{})
			_, err := got.Execute(t.Context(), json.RawMessage(`{"name":"ann","wd":3}`))
			assert.HasError(t, err, "the error of Execute")
			assert.HasPrefix(t, err.Error(), `tool: greet takes "wd" as a string: `, "the error of Execute")
			assert.Empty(t, *calls, "the calls of Elsewhere")
		})

		t.Run("returns the error of the tool for input that is not an object", func(t *testing.T) {
			t.Parallel()
			got, calls := directed(t, tool.Result{})
			_, err := got.Execute(t.Context(), json.RawMessage(`[1]`))
			assert.HasError(t, err, "the error of Execute")
			assert.HasPrefix(t, err.Error(), `tool: "greet" input: `, "the error of Execute")
			assert.Empty(t, *calls, "the calls of Elsewhere")
		})
	})
}
