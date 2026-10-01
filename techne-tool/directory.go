// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
)

// WorkingDirectory is the input field that sets the directory of one call. The paths of the
// call and of its answer are relative to that directory in place of the workspace root.
const WorkingDirectory = "wd"

// Elsewhere runs the tool named name in the workspace of the directory wd, which a call named in
// its [WorkingDirectory] field. input is the input of the call without that field.
type Elsewhere func(ctx context.Context, wd, name string, input json.RawMessage) (Result, error)

// Directed returns t with the optional input field [WorkingDirectory], which the schema
// describes as about. A call without the field, or with the empty string, runs t. A call with a
// directory runs elsewhere with the directory, the name of t and the other fields of the call.
//
// It returns an error for a tool whose input schema already declares the field.
func Directed(t Tool, about string, elsewhere Elsewhere) (Tool, error) {
	in := t.InputSchema().CloneSchemas()
	if _, declared := in.Properties[WorkingDirectory]; declared {
		return nil, fmt.Errorf("tool: %q already takes %q", t.Name(), WorkingDirectory)
	}
	if in.Properties == nil {
		in.Properties = map[string]*jsonschema.Schema{}
	}
	in.Properties[WorkingDirectory] = &jsonschema.Schema{Type: "string", Description: about}
	in.PropertyOrder = append(slices.Clone(in.PropertyOrder), WorkingDirectory)
	return &directed{Tool: t, in: in, elsewhere: elsewhere}, nil
}

// directed is the tool that [Directed] returns.
type directed struct {
	Tool
	// in is the input schema of the embedded tool with the field [WorkingDirectory].
	in        *jsonschema.Schema
	elsewhere Elsewhere
}

// InputSchema returns the input schema of the embedded tool with the field [WorkingDirectory].
func (d *directed) InputSchema() *jsonschema.Schema { return d.in }

// Execute runs the call in the workspace of its [WorkingDirectory], or in the workspace of the
// embedded tool for a call without one. It returns an error for a field that is not a string.
// Input that is not a JSON object goes to the embedded tool, which returns the error of its
// decoding.
func (d *directed) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(input, &fields) != nil {
		return d.Tool.Execute(ctx, input)
	}
	raw, named := fields[WorkingDirectory]
	if !named {
		return d.Tool.Execute(ctx, input)
	}
	var wd string
	if err := json.Unmarshal(raw, &wd); err != nil {
		return Result{}, fmt.Errorf("tool: %s takes %q as a string: %w", d.Name(), WorkingDirectory, err)
	}
	delete(fields, WorkingDirectory)
	rest, err := json.Marshal(fields)
	if err != nil {
		return Result{}, fmt.Errorf("tool: %s input: %w", d.Name(), err)
	}
	if wd == "" {
		return d.Tool.Execute(ctx, rest)
	}
	return d.elsewhere(ctx, wd, d.Name(), rest)
}
