// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
)

// Tool is one operation that an agent calls. A transport translates between its wire format
// and [Tool.Execute], and knows nothing else about a tool.
type Tool interface {
	// Name is the name that an agent calls the tool by, unique within a [Registry].
	Name() string

	// Description tells an agent when to call the tool in place of the built-in operation it
	// replaces. It starts with PREFER OVER and names that operation.
	Description() string

	// InputSchema and OutputSchema describe the input and the output. [New] derives both
	// from the types of the handler.
	InputSchema() *jsonschema.Schema
	OutputSchema() *jsonschema.Schema

	// Execute decodes the input, runs the operation and encodes the result. It returns an
	// error for input that it cannot decode, and runs nothing for it.
	Execute(ctx context.Context, input json.RawMessage) (Result, error)
}

// Result is the output of a tool and whether a caller reads it as a failure. A transport
// reads Failed without decoding the payload.
type Result struct {
	// Payload is the encoded output. A failure has one too, which states the reason.
	Payload json.RawMessage

	// Rendered is the output as text, which a transport sends as the unstructured part of a
	// result. It is empty for an output that does not implement [Renderer], and a transport
	// then sends the payload.
	Rendered string

	// Failed reports that the operation did not do what was asked: no engine serves the
	// language, or the request was refused. A caller can correct either.
	Failed bool
}

// Failing returns a result with payload that a caller reads as a failure.
func Failing(payload json.RawMessage) Result {
	return Result{Payload: payload, Failed: true}
}

// Renderer is implemented by an output that renders itself as text, which a [Result] sends
// to a model beside the JSON.
type Renderer interface {
	Render() string
}

// New returns the tool that runs run under name. It derives the input schema from In and the
// output schema from Out, with the schemas of [marshalled] for the types that encode as a
// word. In is a struct, so each field of its JSON encoding is a property of the input schema.
//
// It returns an error for a type that has no schema, which is a fault of the code that calls
// New.
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

// marshalled are the schemas of the types that encode as one word of a vocabulary, which
// their Go types do not state: [sema.Kind] is a uint8 that encodes as a word. Each enum comes
// from the list of its vocabulary, so every schema lists a word added to the list. [Members]
// refers to the schema of a declaration.
var marshalled = map[reflect.Type]*jsonschema.Schema{
	reflect.TypeFor[sema.Kind]():       enumOf(sema.Kinds()),
	reflect.TypeFor[sema.Visibility](): enumOf(sema.Visibilities()),
	reflect.TypeFor[diag.Severity]():   enumOf(diag.Severities()),
	reflect.TypeFor[KindWord]():        enumOf(sema.Kinds()),
	reflect.TypeFor[RelationWord]():    enumOf(sema.RelationKinds()),
	reflect.TypeFor[FidelityWord]():    enumOf(trust.Fidelities()),
	reflect.TypeFor[Detail]():          enumOf(Levels()),
	reflect.TypeFor[Include]():         enumOf(Includes()),
	reflect.TypeFor[Members]():         nestedDeclarations(),
}

// itemSchema is the location of the schema of one declaration in the schema of a read tool's
// output. [Answer] has its items at the root, and an output that embeds Answer has them at
// the same place.
const itemSchema = "#/properties/items/items"

// nestedDeclarations returns the schema of [Members], an array whose items refer to
// [itemSchema].
func nestedDeclarations() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:  "array",
		Items: &jsonschema.Schema{Ref: itemSchema},
	}
}

// enumOf returns the schema of a string that is the word of one of values.
func enumOf[T fmt.Stringer](values []T) *jsonschema.Schema {
	out := make([]any, 0, len(values))
	for _, v := range values {
		out = append(out, v.String())
	}
	return &jsonschema.Schema{Type: "string", Enum: out}
}

// typed is the tool that [New] returns for one handler.
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

// Execute decodes input into In, runs the handler and encodes its output. An output whose
// Failed method reports true makes a failing result, and the result of an output that
// implements [Renderer] has its render.
func (t *typed[In, Out]) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	decoded, err := t.decode(input)
	if err != nil {
		return Result{}, err
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
		failing := Failing(encoded)
		failing.Rendered = text
		return failing, nil
	}
	return Result{Payload: encoded, Rendered: text}, nil
}

// decode returns input as In, and reads an absent input as the empty object. It returns an
// error for input that is not an object, for a field that the input schema does not declare,
// and for a required field that input omits. encoding/json ignores an undeclared field and
// leaves an omitted field at its zero value, so decode compares the fields with the schema
// before it decodes them.
func (t *typed[In, Out]) decode(input json.RawMessage) (In, error) {
	var decoded In
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(input, &fields); err != nil {
		return decoded, fmt.Errorf("tool: %q input: %w", t.name, err)
	}
	if err := t.fits(fields); err != nil {
		return decoded, err
	}
	if err := json.Unmarshal(input, &decoded); err != nil {
		return decoded, fmt.Errorf("tool: %q input: %w", t.name, err)
	}
	return decoded, nil
}

// fits returns an error that lists each field of fields that the input schema does not
// declare, with the fields that it declares, and else an error that lists each field that
// the schema requires and fields omits.
func (t *typed[In, Out]) fits(fields map[string]json.RawMessage) error {
	var unknown []string
	for name := range fields {
		if _, declared := t.in.Properties[name]; !declared {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return fmt.Errorf("tool: %s does not take %s. It takes %s", t.name, quoted(unknown), taken(t.in))
	}
	var missing []string
	for _, name := range t.in.Required {
		if _, given := fields[name]; !given {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("tool: %s needs %s", t.name, quoted(missing))
	}
	return nil
}

// quoted returns names in double quotes, separated by commas.
func quoted(names []string) string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = strconv.Quote(name)
	}
	return strings.Join(out, ", ")
}

// taken returns the properties of schema in the order of its fields, separated by commas, or
// "no field" for a schema without properties.
func taken(schema *jsonschema.Schema) string {
	if len(schema.PropertyOrder) == 0 {
		return "no field"
	}
	return strings.Join(schema.PropertyOrder, ", ")
}

// rendered returns the render of out, or the empty string for an output that does not
// implement [Renderer].
func rendered(out any) string {
	if r, ok := out.(Renderer); ok {
		return r.Render()
	}
	return ""
}
