// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"go.dokimi.dev/techne/core/sema"
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

	// Rendered is the answer written for a reader rather than a parser,
	// and is what a transport puts in the unstructured half of a result.
	// It is empty for an output that renders itself no better than its
	// JSON does, and a transport then sends the payload.
	Rendered string

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

// Renderer is implemented by an output that reads better as text than
// as JSON.
//
// A tool result carries an unstructured block and a structured one, with
// different readers: a model reads the first and a program reads the
// second. An output implementing this is sent as both, each in the form
// its reader wants, rather than as the same JSON twice.
type Renderer interface {
	Render() string
}

// New builds a tool from a typed handler.
//
// It reports an error when a schema cannot be derived from either type,
// which is a programming error rather than something a caller did.
func New[In, Out any](
	name, description string,
	run func(context.Context, In) (Out, error),
) (Tool, error) {
	in, err := jsonschema.For[In](&jsonschema.ForOptions{TypeSchemas: marshalled})
	if err != nil {
		return nil, fmt.Errorf("tool: %q input schema: %w", name, err)
	}
	out, err := jsonschema.For[Out](&jsonschema.ForOptions{TypeSchemas: marshalled})
	if err != nil {
		return nil, fmt.Errorf("tool: %q output schema: %w", name, err)
	}
	return &typed[In, Out]{name: name, description: description, in: in, out: out, run: run}, nil
}

// marshalled names the types whose schema is not the one their Go type
// implies.
//
// A schema is derived from the Go type, and a type with its own
// MarshalJSON writes something else: [sema.Kind] is a uint8 that
// marshals as a word. Without this a tool would advertise an integer and
// return a string, and a caller validating against the schema would
// reject every answer it got.
//
// The values come from the vocabulary itself rather than a list here, so
// a kind added there reaches the schema without a second edit.
var marshalled = map[reflect.Type]*jsonschema.Schema{
	reflect.TypeFor[sema.Kind]():       enumOf(sema.Kinds()),
	reflect.TypeFor[sema.Visibility](): enumOf(sema.Visibilities()),
	reflect.TypeFor[Members]():         nestedDeclarations(),
}

// itemSchema points at where a read tool's answer describes one
// declaration. Answer states its items at the root, and a tool
// embedding Answer flattens into the same place.
const itemSchema = "#/properties/items/items"

// nestedDeclarations describes the members a declaration holds.
//
// A declaration holds declarations, and deriving a schema from a type
// that contains itself does not terminate. The members are described by
// pointing at the description the answer already carries, which is what
// a reference is for and what keeps the two from drifting apart.
func nestedDeclarations() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:  "array",
		Items: &jsonschema.Schema{Ref: itemSchema},
	}
}

// enumOf builds the schema for a type that marshals as one of a closed
// set of words.
func enumOf[T fmt.Stringer](values []T) *jsonschema.Schema {
	out := make([]any, 0, len(values))
	for _, v := range values {
		out = append(out, v.String())
	}
	return &jsonschema.Schema{Type: "string", Enum: out}
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

// rendered returns the reading form of an output, or nothing when the
// output has none.
func rendered(out any) string {
	if r, ok := out.(Renderer); ok {
		return r.Render()
	}
	return ""
}

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
	text := rendered(out)
	if failer, marks := any(out).(interface{ Failed() bool }); marks && failer.Failed() {
		failed := Failing(encoded)
		failed.Rendered = text
		return failed, nil
	}
	return Result{Payload: encoded, Rendered: text}, nil
}
