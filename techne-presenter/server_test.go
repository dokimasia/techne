// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package presenter_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/tool"
	"go.dokimi.dev/techne/presenter"
)

type answerIn struct {
	Scope string `json:"scope"`
}

type answerOut struct {
	Status string `json:"status"`
}

// Failed marks the two statuses a model can correct.
func (a answerOut) Failed() bool {
	return a.Status == "unsupported" || a.Status == "refused"
}

func registry(t *testing.T, run func(context.Context, answerIn) (answerOut, error)) *tool.Registry {
	t.Helper()
	built, err := tool.New("outline", "PREFER OVER read for this.", run)
	assert.NoError(t, err, "a handler over serialisable types produces a tool")
	r := tool.NewRegistry()
	assert.NoError(t, r.Add(built), "the case needs a tool registered")
	return r
}

func answering(status string) func(context.Context, answerIn) (answerOut, error) {
	return func(context.Context, answerIn) (answerOut, error) {
		return answerOut{Status: status}, nil
	}
}

// call drives one tool through the server the presenter builds.
func call(t *testing.T, r *tool.Registry, name, input string) *mcp.CallToolResult {
	t.Helper()
	server, err := presenter.NewServer(r, presenter.Info{Name: "techne", Version: "test"})
	assert.NoError(t, err, "a registry of well-formed tools produces a server")

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	clientSide, serverSide := mcp.NewInMemoryTransports()

	serving, err := server.Connect(t.Context(), serverSide, nil)
	assert.NoError(t, err, "the server accepts a connection")
	t.Cleanup(func() { _ = serving.Wait() })

	session, err := client.Connect(t.Context(), clientSide, nil)
	assert.NoError(t, err, "the client connects")
	t.Cleanup(func() { _ = session.Close() })

	got, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name: name, Arguments: json.RawMessage(input),
	})
	assert.NoError(t, err, "calling a registered tool is not a protocol error")
	return got
}

func TestServer(t *testing.T) {
	t.Parallel()

	t.Run("NewServer", func(t *testing.T) {
		t.Parallel()

		t.Run("offers every registered tool", func(t *testing.T) {
			t.Parallel()
			server, err := presenter.NewServer(registry(t, answering("ok")),
				presenter.Info{Name: "techne", Version: "test"})
			assert.NoError(t, err, "a registry of well-formed tools produces a server")
			assert.NotNil(t, server, "a server an agent can connect to")
		})
	})

	t.Run("CallTool", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the tool's own answer as structured content", func(t *testing.T) {
			t.Parallel()
			got := call(t, registry(t, answering("ok")), "outline", `{"scope":"a.fx"}`)
			assert.False(t, got.IsError, "an operation that did what was asked is not an error")
			assert.NotNil(t, got.StructuredContent, "a caller validates this against the output schema")
		})

		t.Run("also sends the answer as text", func(t *testing.T) {
			t.Parallel()
			// The specification asks a tool returning structured content
			// to send the same JSON in a text block.
			got := call(t, registry(t, answering("ok")), "outline", `{"scope":"a.fx"}`)
			assert.NotEmpty(t, got.Content, "a client that reads only text still gets the answer")
		})

		t.Run("marks a refusal as a tool execution error", func(t *testing.T) {
			t.Parallel()
			// A model can correct "no engine serves this language". It
			// can do nothing with a protocol error.
			got := call(t, registry(t, answering("unsupported")), "outline", `{"scope":"a.md"}`)
			assert.True(t, got.IsError, "a caller is told the operation did not do what was asked")
			assert.NotEmpty(t, got.Content, "the reason travels with the failure")
		})

		t.Run("does not mark a degraded answer as an error", func(t *testing.T) {
			t.Parallel()
			got := call(t, registry(t, answering("degraded")), "outline", `{"scope":"a.fx"}`)
			assert.False(t, got.IsError, "weaker evidence than asked for is still worth reading")
		})

		t.Run("reports a handler's own failure to the model", func(t *testing.T) {
			t.Parallel()
			// A decode failure or a refused path is something the model
			// can fix, so it reaches the model rather than the wire.
			failing := func(context.Context, answerIn) (answerOut, error) {
				return answerOut{}, errors.New("tool: \"/etc/passwd\" is absolute")
			}
			got := call(t, registry(t, failing), "outline", `{"scope":"/etc/passwd"}`)
			assert.True(t, got.IsError, "the operation did not do what was asked")
		})
	})
}
