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

// Output selects what the result of a call contains beside its text block.
type Output uint8

const (
	// Text sends the text block alone, and the server does not declare an output schema. A
	// client that hands a model the structured content of a result, as Claude Code does, then
	// hands it the render, which takes about half the characters of the JSON.
	Text Output = 0
	// Structured sends the payload of the tool as the structured content of the result, and
	// the server declares the output schema of each tool, for a client that reads the JSON.
	Structured Output = 1
)

// NewServer returns a server that offers every tool of r, identifies itself by about and
// answers each call with the content that output selects.
//
// It returns an error for a tool whose schemas do not encode, which is a fault of the tool.
// It panics for a tool whose input schema is not of type object, as [mcp.Server.AddTool]
// does.
func NewServer(r *tool.Registry, about Info, output Output) (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{Name: about.Name, Version: about.Version}, nil)

	for _, t := range r.Tools() {
		in, err := json.Marshal(t.InputSchema())
		if err != nil {
			return nil, fmt.Errorf("presenter: %q input schema: %w", t.Name(), err)
		}
		declared := &mcp.Tool{Name: t.Name(), Description: t.Description(), InputSchema: json.RawMessage(in)}
		if output == Structured {
			out, err := json.Marshal(t.OutputSchema())
			if err != nil {
				return nil, fmt.Errorf("presenter: %q output schema: %w", t.Name(), err)
			}
			declared.OutputSchema = json.RawMessage(out)
		}
		server.AddTool(declared, handler(t, output))
	}
	return server, nil
}

// handler returns the handler of the calls of t.
//
// The text block of a result is the render, or the payload for a result without one. For
// [Structured] the structured content of a result is the payload of t, and the SDK writes the
// payload into the response without decoding it. The _meta of the result states
// [tool.Result.Waited] under [tool.WaitedMeta]. A failed result, an error of Execute and a
// panic in the goroutine of the call return an error result. The stack of a panic goes to
// standard error.
//
// A payload that is not JSON returns a protocol error, because the SDK does not send a
// response for a result that it cannot encode.
func handler(t tool.Tool, output Output) mcp.ToolHandler {
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
		called = &mcp.CallToolResult{
			Meta:    mcp.Meta{tool.WaitedMeta: result.Waited.Milliseconds()},
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
			IsError: result.Failed,
		}
		if output == Structured {
			called.StructuredContent = result.Payload
		}
		return called, nil
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
