// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package presenter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.dokimi.dev/techne/tool"
)

// Info is the name and the version that a server reports to a client in its initialize
// response.
type Info struct {
	Name    string
	Version string
}

// NewServer returns a server that offers every tool of r and identifies itself by about.
//
// It returns an error for a tool whose schemas do not encode, which is a fault of the tool.
// It panics for a tool whose input schema is not of type object, as [mcp.Server.AddTool]
// does.
func NewServer(r *tool.Registry, about Info) (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{Name: about.Name, Version: about.Version}, nil)

	for _, t := range r.Tools() {
		in, err := json.Marshal(t.InputSchema())
		if err != nil {
			return nil, fmt.Errorf("presenter: %q input schema: %w", t.Name(), err)
		}
		out, err := json.Marshal(t.OutputSchema())
		if err != nil {
			return nil, fmt.Errorf("presenter: %q output schema: %w", t.Name(), err)
		}

		server.AddTool(&mcp.Tool{
			Name:         t.Name(),
			Description:  t.Description(),
			InputSchema:  json.RawMessage(in),
			OutputSchema: json.RawMessage(out),
		}, handler(t))
	}
	return server, nil
}

// handler returns the handler of the calls of t.
//
// The structured content of a result is the payload of t, and the SDK writes the payload
// into the response without decoding it. The text block is the render, or the payload for a
// result without one. A failed result, an error of Execute and a panic in the goroutine of
// the call return an error result. The stack of a panic goes to standard error.
//
// A payload that is not JSON returns a protocol error, because the SDK does not send a
// response for a result that it cannot encode.
func handler(t tool.Tool) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (called *mcp.CallToolResult, err error) {
		defer func() {
			if fault := recover(); fault != nil {
				reason := fmt.Sprintf("presenter: %q panicked: %v", t.Name(), fault)
				fmt.Fprintf(os.Stderr, "%s\n%s", reason, debug.Stack())
				called, err = failure(reason+". The stack is on the standard error of the server."), nil
			}
		}()

		result, err := t.Execute(ctx, req.Params.Arguments)
		if err != nil {
			return failure(err.Error()), nil
		}
		if !json.Valid(result.Payload) {
			return nil, fmt.Errorf("presenter: %q returned a payload that is not JSON", t.Name())
		}

		text := result.Rendered
		if text == "" {
			text = string(result.Payload)
		}
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: result.Payload,
			IsError:           result.Failed,
		}, nil
	}
}

// failure returns an error result whose text is reason.
func failure(reason string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: reason}},
		IsError: true,
	}
}

// Serve runs s over standard input and output. It returns nil when the client closes standard
// input, and the error of ctx when ctx is done.
func Serve(ctx context.Context, s *mcp.Server) error {
	return s.Run(ctx, &mcp.StdioTransport{})
}
