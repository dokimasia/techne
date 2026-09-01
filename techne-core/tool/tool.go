// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// Tool is one operation an agent can call.
//
// A transport translates between its own wire format and [Tool.Execute]
// and knows nothing else. A presenter that knew about individual tools
// would be one piece of code per tool and transport pair.
type Tool interface {
	// Name is what an agent routes on. It is unique within a server.
	Name() string

	// Description tells an agent when to reach for this instead of the
	// built-in it replaces. A description that does not say so leaves
	// the tool unchosen, and a tool that is not chosen has no cost to
	// measure.
	Description() string

	// InputSchema and OutputSchema are derived from the handler's own
	// types, so a field added to the Go type cannot go undocumented.
	InputSchema() *jsonschema.Schema
	OutputSchema() *jsonschema.Schema

	// Execute decodes the input, runs the operation and encodes the
	// result. Input it cannot decode is refused rather than defaulted:
	// running an operation on arguments nobody sent is worse than
	// failing.
	Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

// New builds a tool from a typed handler.
//
// It reports an error when a schema cannot be derived from either type,
// which is a programming error rather than something a caller did.
func New[In, Out any](
	name, description string,
	run func(context.Context, In) (Out, error),
) (Tool, error) {
	in, err := jsonschema.For[In](nil)
	if err != nil {
		return nil, fmt.Errorf("tool: %q input schema: %w", name, err)
	}
	out, err := jsonschema.For[Out](nil)
	if err != nil {
		return nil, fmt.Errorf("tool: %q output schema: %w", name, err)
	}
	return &typed[In, Out]{name: name, description: description, in: in, out: out, run: run}, nil
}

// typed is a tool over one input and output type.
type typed[In, Out any] struct {
	name        string
	description string
	in          *jsonschema.Schema
	out         *jsonschema.Schema
	run         func(context.Context, In) (Out, error)
}

func (t *typed[In, Out]) Name() string                     { return t.name }
func (t *typed[In, Out]) Description() string              { return t.description }
func (t *typed[In, Out]) InputSchema() *jsonschema.Schema  { return t.in }
func (t *typed[In, Out]) OutputSchema() *jsonschema.Schema { return t.out }

func (t *typed[In, Out]) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var decoded In
	if len(input) > 0 {
		if err := json.Unmarshal(input, &decoded); err != nil {
			return nil, fmt.Errorf("tool: %q input: %w", t.name, err)
		}
	}

	result, err := t.run(ctx, decoded)
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("tool: %q output: %w", t.name, err)
	}
	return encoded, nil
}
