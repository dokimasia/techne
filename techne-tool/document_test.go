// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/tool"
)

// addressable returns the read service that the addressing tests ask: in a.fx, the struct
// Store on line 1, its methods Get and Put on lines 3 and 4, and the function Get on line 5.
func addressable() *reads {
	return serving(
		declared("Store", sema.KindStruct, "", 1, 10),
		declared("Get", sema.KindMethod, "Store", 3, 30),
		declared("Put", sema.KindMethod, "Store", 4, 40),
		declared("Get", sema.KindFunction, "", 5, 50),
	)
}

// declared returns a declaration of a.fx in the language fx, named name, of kind, a member of
// parent when parent is set, on the zero-based line line, at offset. The ID of a member is
// qualified by its parent.
func declared(name string, kind sema.Kind, parent string, line, offset int) sema.Symbol {
	out := sema.Symbol{
		ID: sema.NewID("fx", "a", name, kind), Name: name, Kind: kind, Language: "fx",
		Span: source.Span{
			Path:  "a.fx",
			Start: source.Position{Offset: offset, Line: line},
			End:   source.Position{Offset: offset + 5, Line: line},
		},
	}
	if parent != "" {
		out.ID = sema.NewID("fx", "a", sema.Qualify(parent, name), kind)
		out.Parent = sema.NewID("fx", "a", parent, sema.KindStruct)
	}
	return out
}

// recorder is a write path that records the request it receives and changes nothing. Its
// outcome has gate as the evidence of its gate.
type recorder struct {
	asked edit.Request
	gate  *trust.Provenance
}

func (r *recorder) Apply(_ context.Context, req edit.Request) (edit.Outcome, error) {
	r.asked = req
	return edit.Outcome{
		Operation: req.Operation,
		Status:    trust.OK,
		Applied:   !req.DryRun,
		Rewrites: []edit.Rewrite{{
			Path: "a.fx", Line: 1, Now: "// " + req.Args[edit.ArgDoc],
		}},
		Provenance: trust.Provenance{
			Engine: "planner", Fidelity: trust.Syntactic, Completeness: trust.ScopeTotal,
		},
		Gate: r.gate,
	}, nil
}

// documenting runs the document.symbol tool over [addressable] and writer with input, and
// decodes the output.
func documenting(t *testing.T, writer *recorder, input string) tool.Written {
	t.Helper()
	built, err := tool.Document(addressable(), writer)
	assert.NoError(t, err, "the error of Document")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.Written
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out
}

func TestDocument(t *testing.T) {
	t.Parallel()

	t.Run("Document", func(t *testing.T) {
		t.Parallel()

		t.Run("sends the documentation unchanged", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			documenting(t, writer, `{"scope":"a.fx","name":"Store","doc":"Store keeps items."}`)
			assert.Equal(t, writer.asked.Args[edit.ArgDoc], "Store keeps items.", "the documentation of the request")
		})

		t.Run("asks the language of the declaration", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			got := documenting(t, writer, `{"scope":"a.fx","name":"Store","doc":"x"}`)
			assert.Equal(t, string(writer.asked.Language), "fx", "the language of the request")
			assert.Equal(t, got.Scope.Language, "fx", "the language of the scope")
		})

		t.Run("refuses an empty documentation", func(t *testing.T) {
			t.Parallel()
			got := documenting(t, &recorder{}, `{"scope":"a.fx","name":"Store","doc":""}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Equal(t, got.Error.Code, "refused", "the code of the failure")
		})

		t.Run("previews a request without dry_run", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			documenting(t, writer, `{"scope":"a.fx","name":"Store","doc":"x"}`)
			assert.True(t, writer.asked.DryRun, "DryRun of the request")
		})

		t.Run("writes a request with dry_run false", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			documenting(t, writer, `{"scope":"a.fx","name":"Store","doc":"x","dry_run":false}`)
			assert.False(t, writer.asked.DryRun, "DryRun of the request")
		})
	})
}
