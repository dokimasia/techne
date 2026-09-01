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
	Execute(ctx context.Context, input json.RawMessage) (Result, error)
}

// Result is what a tool produced and whether the caller should read it
// as a failure.
//
// A transport needs to know which without parsing the payload, because a
// transport that understood a domain shape would have to change every
// time one did.
type Result struct {
	// Payload is the encoded output, and is present either way: a
	// refusal a caller can act on says why in the same shape as a
	// success.
	Payload json.RawMessage

	// Failed reports that the operation did not do what was asked. It
	// covers a language nothing serves and a request that was declined,
	// both of which a model can correct.
	Failed bool
}

// Failing marks a result as one the caller should read as a failure.
//
// A tool whose output carries a status uses this so the transport does
// not have to read the payload to find out.
func Failing(payload json.RawMessage) Result {
	return Result{Payload: payload, Failed: true}
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

func (t *typed[In, Out]) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var decoded In
	if len(input) > 0 {
		if err := json.Unmarshal(input, &decoded); err != nil {
			return Result{}, fmt.Errorf("tool: %q input: %w", t.name, err)
		}
	}

	out, err := t.run(ctx, decoded)
	if err != nil {
		return Result{}, err
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return Result{}, fmt.Errorf("tool: %q output: %w", t.name, err)
	}
	if failer, marks := any(out).(interface{ Failed() bool }); marks && failer.Failed() {
		return Failing(encoded), nil
	}
	return Result{Payload: encoded}, nil
}
