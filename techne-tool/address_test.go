// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/tool"
)

// addressing runs the document.symbol tool over found with input, and returns the output and
// the request of the write path.
func addressing(t *testing.T, found []sema.Symbol, input string) (tool.Written, edit.Request) {
	t.Helper()
	writer := &recorder{}
	built, err := tool.Document(serving(found...), writer)
	assert.NoError(t, err, "the error of Document")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.Written
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out, writer.asked
}

// stored returns the declarations of [addressable].
func stored() []sema.Symbol { return addressable().engine.found }

// TestAddress covers the rule by which a name addresses a declaration. The tools that take a
// name share the rule, so the tests run it through the document.symbol tool and compare it
// with the relations tool.
func TestAddress(t *testing.T) {
	t.Parallel()

	t.Run("Execute", func(t *testing.T) {
		t.Parallel()

		t.Run("points the write path at the span of the declaration", func(t *testing.T) {
			t.Parallel()
			got, asked := addressing(t, stored(), `{"scope":"a.fx","name":"Store","doc":"x"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, asked.Target.Kind, edit.TargetSpan, "the kind of the target")
			assert.Equal(t, asked.Target.Span.Start.Offset, 10, "the offset of the target")
		})

		t.Run("addresses a member by its qualified name", func(t *testing.T) {
			t.Parallel()
			got, asked := addressing(t, stored(), `{"scope":"a.fx","name":"Store.Get","doc":"x"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, asked.Target.Span.Start.Offset, 30, "the offset of the method Get")
		})

		t.Run("narrows an ambiguous name by its kind", func(t *testing.T) {
			t.Parallel()
			got, asked := addressing(t, stored(), `{"scope":"a.fx","name":"Get","kind":"function","doc":"x"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, asked.Target.Span.Start.Offset, 50, "the offset of the function Get")
		})

		t.Run("refuses a name of two declarations with the site of each", func(t *testing.T) {
			t.Parallel()
			got, _ := addressing(t, stored(), `{"scope":"a.fx","name":"Get","doc":"x"}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Contains(t, got.Error.Reason, "method at a.fx:4", "the reason of the failure")
			assert.Contains(t, got.Error.Reason, "function at a.fx:6", "the reason of the failure")
		})

		t.Run("refuses an ambiguous name with one reason in two tools", func(t *testing.T) {
			t.Parallel()
			documented, _ := addressing(t, stored(), `{"scope":"a.fx","name":"Get","doc":"x"}`)
			related := relatedOver(t, addressable(), `{"scope":"a.fx","name":"Get","relation":"called-by"}`)
			assert.True(t, related.Failed(), "the failure of the relations output")
			assert.Equal(t, related.Error.Reason, documented.Error.Reason, "the reason of the relations tool")
		})

		t.Run("refuses a name that nothing declares with the similar names", func(t *testing.T) {
			t.Parallel()
			got, _ := addressing(t, stored(), `{"scope":"a.fx","name":"Stor","doc":"x"}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Contains(t, got.Error.Reason, "It declares Store", "the reason of the failure")
		})

		t.Run("lists no name for a name that no name resembles", func(t *testing.T) {
			t.Parallel()
			got, _ := addressing(t, stored(), `{"scope":"a.fx","name":"zzzz","doc":"x"}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.NotContains(t, got.Error.Reason, "It declares", "the reason of the failure")
		})

		t.Run("names the unread file of a scope for a name that nothing declares", func(t *testing.T) {
			t.Parallel()
			over := addressable()
			over.engine.found = nil
			writer := &recorder{}
			built, err := tool.Document(unreadOver(over), writer)
			assert.NoError(t, err, "the error of Document")
			result, err := built.Execute(t.Context(), json.RawMessage(`{"scope":"a.fx","name":"Store","doc":"x"}`))
			assert.NoError(t, err, "the error of Execute")
			var got tool.Written
			assert.NoError(t, json.Unmarshal(result.Payload, &got), "the decoding of the output")
			assert.Contains(t, got.Error.Reason, "big.fx", "the reason of the failure")
		})

		t.Run("addresses an import of one name in several files once", func(t *testing.T) {
			t.Parallel()
			first := declared("fmt", sema.KindImport, "", 0, 0)
			second := first
			second.ID, second.Span.Path = sema.NewID("fx", "b", "fmt", sema.KindImport), "b.fx"
			got, asked := addressing(t, []sema.Symbol{first, second}, `{"scope":"a.fx","name":"fmt","doc":"x"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, asked.Target.Span.Path, source.Path("a.fx"), "the file of the target")
		})

		t.Run("addresses a type over its own constructors", func(t *testing.T) {
			t.Parallel()
			class := declared("Reader", sema.KindStruct, "", 0, 0)
			class.Span.End.Offset = 100
			constructors := []sema.Symbol{
				declared("Reader", sema.KindConstructor, "Reader", 1, 10),
				declared("Reader", sema.KindConstructor, "Reader", 2, 30),
			}
			got, asked := addressing(t, append([]sema.Symbol{class}, constructors...),
				`{"scope":"a.fx","name":"Reader","doc":"x"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, asked.Target.Span.Start.Offset, 0, "the offset of the class Reader")
		})

		t.Run("refuses a type beside a constructor of another type", func(t *testing.T) {
			t.Parallel()
			class := declared("Reader", sema.KindStruct, "", 0, 0)
			foreign := declared("Reader", sema.KindConstructor, "Other", 1, 10)
			got, _ := addressing(t, []sema.Symbol{class, foreign}, `{"scope":"a.fx","name":"Reader","doc":"x"}`)
			assert.True(t, got.Failed(), "the failure of the output")
		})

		t.Run("addresses the first of the declarations of one ID", func(t *testing.T) {
			t.Parallel()
			prototype := declared("stem", sema.KindFunction, "", 2, 20)
			definition := declared("stem", sema.KindFunction, "", 9, 90)
			both := []sema.Symbol{prototype, definition}
			got, asked := addressing(t, both, `{"scope":"a.fx","name":"stem","doc":"x"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, asked.Target.Span.Start.Offset, 20, "the offset of the prototype of stem")
		})
	})
}

// unreadOver returns over with an unread caveat that names big.fx on each outline.
func unreadOver(over *reads) tool.Outliner { return unread{over} }

// unread is a read service whose outline has an unread caveat for big.fx.
type unread struct{ *reads }

func (u unread) Outline(ctx context.Context, req engine.Request) (engine.Answer[sema.Symbol], error) {
	answered, err := u.reads.Outline(ctx, req)
	answered.Provenance.Caveats = append(answered.Provenance.Caveats, trust.Caveat{
		Code: trust.CaveatUnread, Note: "larger than an engine reads", Paths: []source.Path{"big.fx"},
	})
	return answered, err
}
