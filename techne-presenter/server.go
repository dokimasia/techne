// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package presenter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.dokimi.dev/techne/core/tool"
)

// Info identifies this server to a client.
type Info struct {
	Name    string
	Version string
}

// NewServer builds a server offering every tool in a registry.
//
// It reports an error when a tool's schemas cannot be encoded, which is
// a programming error in the tool rather than something a caller did.
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

// handler drives one tool.
//
// It never reads the payload. [tool.Result] says whether the answer is a
// failure, so a change to what a tool returns does not reach here.
func handler(t tool.Tool) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := t.Execute(ctx, req.Params.Arguments)
		if err != nil {
			// The model can correct a malformed argument or a refused
			// path, so this reaches the model rather than the wire.
			return failure(err.Error()), nil
		}

		var structured any
		if err := json.Unmarshal(result.Payload, &structured); err != nil {
			return nil, fmt.Errorf("presenter: %q produced output that is not JSON: %w", t.Name(), err)
		}

		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: string(result.Payload)}},
			StructuredContent: structured,
			IsError:           result.Failed,
		}, nil
	}
}

// failure reports a fault the model can act on.
func failure(reason string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: reason}},
		IsError: true,
	}
}
