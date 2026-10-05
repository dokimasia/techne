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
	return addressedOver(t, serving(found...), writer, input), writer.asked
}

// addressedOver runs the document.symbol tool over reads and writer with input, and decodes the
// output.
func addressedOver(t *testing.T, reads tool.Outliner, writer *recorder, input string) tool.Written {
	t.Helper()
	built, err := tool.Document(reads, writer)
	assert.NoError(t, err, "the error of Document")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.Written
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out
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

		t.Run("points the write path at the ID of a declaration whose span another declaration shares",
			func(t *testing.T) {
				t.Parallel()
				have := declared("have", sema.KindParameter, "", 3, 30)
				want := declared("want", sema.KindParameter, "", 3, 30)
				got, asked := addressing(t, []sema.Symbol{have, want}, `{"scope":"a.fx","name":"want","doc":"x"}`)
				assert.False(t, got.Failed(), "the failure of the output")
				assert.Equal(t, asked.Target.Symbol, want.ID, "the ID of the target")
				assert.Equal(t, asked.Target.Span, want.Span, "the span of the target")
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

		t.Run("addresses a declaration qualified by the name of its unit", func(t *testing.T) {
			t.Parallel()
			got, asked := addressing(t, stored(), `{"scope":"a.fx","name":"a.Store","doc":"x"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, asked.Target.Span.Start.Offset, 10, "the offset of Store")
		})

		t.Run("picks the declaration that starts on the line over one that spans it", func(t *testing.T) {
			t.Parallel()
			spanning := declared("Clock", sema.KindStruct, "", 21, 0)
			spanning.Span.End.Line = 42
			starting := declared("Clock", sema.KindStruct, "", 22, 0)
			starting.ID, starting.Span.Path = sema.NewID("fx", "b", "Clock", sema.KindStruct), "b.fx"
			got, asked := addressing(t, []sema.Symbol{spanning, starting},
				`{"scope":"a.fx","name":"Clock","line":23,"doc":"x"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, asked.Target.Span.Path, source.Path("b.fx"), "the file of the target")
		})

		t.Run("names the kinds of a name that another kind was asked for", func(t *testing.T) {
			t.Parallel()
			got, _ := addressing(t, stored(), `{"scope":"a.fx","name":"Store","kind":"function","doc":"x"}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Contains(t, got.Error.Reason, `declares no function called "Store". It declares struct at a.fx:2`,
				"the reason of the failure")
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
			got := addressedOver(t, unreadOver(over), &recorder{}, `{"scope":"a.fx","name":"Store","doc":"x"}`)
			assert.Contains(t, got.Error.Reason, "big.fx", "the reason of the failure")
		})

		t.Run("refuses a scope that the read service refuses with its reason", func(t *testing.T) {
			t.Parallel()
			got := addressedOver(t, refusedOver(addressable(), "gone.fx does not exist"), &recorder{},
				`{"scope":"gone.fx","name":"Store","doc":"x"}`)
			assert.Equal(t, got.Error.Code, "refused", "the code of the failure")
			assert.Equal(t, got.Error.Reason, "gone.fx does not exist", "the reason of the failure")
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
			got, asked := addressing(t, stems(), `{"scope":"a.fx","name":"stem","doc":"x"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, asked.Target.Span.Start.Offset, 20, "the offset of the prototype of stem")
		})

		t.Run("addresses a declaration of one ID by a line of it", func(t *testing.T) {
			t.Parallel()
			got, asked := addressing(t, stems(), `{"scope":"a.fx","name":"stem","line":10,"doc":"x"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, asked.Target.Span.Start.Offset, 90, "the offset of the definition of stem")
		})

		t.Run("addresses a declaration by a line inside its span", func(t *testing.T) {
			t.Parallel()
			found := stems()
			found[1].Span.End.Line = 14
			got, asked := addressing(t, found, `{"scope":"a.fx","name":"stem","line":12,"doc":"x"}`)
			assert.False(t, got.Failed(), "the failure of the output")
			assert.Equal(t, asked.Target.Span.Start.Offset, 90, "the offset of the definition of stem")
		})

		t.Run("addresses a declaration by its line in the relations tool", func(t *testing.T) {
			t.Parallel()
			over := serving(stems()...)
			related := relatedOver(t, over, `{"scope":"a.fx","name":"stem","line":10,"relation":"called-by"}`)
			assert.False(t, related.Failed(), "the failure of the relations output")
			assert.Equal(t, over.related[0].Declared, stems()[1].Span, "the declared span of the request")
		})

		t.Run("refuses a line without a declaration of the name with the site of each", func(t *testing.T) {
			t.Parallel()
			got, _ := addressing(t, stems(), `{"scope":"a.fx","name":"stem","line":5,"doc":"x"}`)
			assert.True(t, got.Failed(), "the failure of the output")
			assert.Equal(t, got.Error.Reason,
				`"a.fx" declares no "stem" on line 5. It declares function at a.fx:3, function at a.fx:10`,
				"the reason of the failure")
		})

		t.Run("returns an error for a negative line", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Document(serving(stored()...), &recorder{})
			assert.NoError(t, err, "the error of Document")
			_, err = built.Execute(t.Context(), json.RawMessage(`{"scope":"a.fx","name":"Store","line":-1,"doc":"x"}`))
			assert.HasError(t, err, "the error of Execute")
			assert.Contains(t, err.Error(), `"line" of at least 0, not -1`, "the error of Execute")
		})
	})
}

// stems returns the prototype of stem on line 3 and its definition on line 10, which share an
// ID as the prototype and the definition of a C function do.
func stems() []sema.Symbol {
	return []sema.Symbol{
		declared("stem", sema.KindFunction, "", 2, 20),
		declared("stem", sema.KindFunction, "", 9, 90),
	}
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

// refusedOver returns over with each outline refused for the reason why, as a service refuses a
// scope that does not exist.
func refusedOver(over *reads, why string) tool.Outliner { return refusing{over, why} }

// refusing is a read service whose outline is refused.
type refusing struct {
	*reads
	why string
}

func (r refusing) Outline(context.Context, engine.Request) (engine.Answer[sema.Symbol], error) {
	return engine.Refused[sema.Symbol](r.why), nil
}
