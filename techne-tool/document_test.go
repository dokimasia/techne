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

func TestDocument(t *testing.T) {
	t.Parallel()

	t.Run("addressing", func(t *testing.T) {
		t.Parallel()

		t.Run("takes a name and turns it into a position", func(t *testing.T) {
			t.Parallel()
			// A planner is pointed at a span rather than at an identity,
			// because an identity is a language, a unit, a name and a
			// kind, and a unit with two Get methods satisfies one twice.
			writer := &recorder{}
			got := documenting(t, writer, `{"scope":"a.fx","name":"Store","doc":"Holds items."}`)

			assert.False(t, got.Failed(), "a name one declaration answers to is served")
			assert.Equal(t, writer.asked.Target.Kind, edit.TargetSpan,
				"the tool resolves the name, so the planner is pointed at one declaration")
			assert.Equal(t, writer.asked.Target.Span.Start.Offset, 10,
				"and at the one the name picked out")
			assert.Equal(t, writer.asked.Args[edit.ArgDoc], "Holds items.",
				"the prose reaches the planner unchanged")
		})

		t.Run("accepts the qualified form the language writes", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			got := documenting(t, writer, `{"scope":"a.fx","name":"Store.Get","doc":"x"}`)
			assert.False(t, got.Failed(), "a reader recognises a method by its container")
			assert.Equal(t, writer.asked.Target.Span.Start.Offset, 30, "the method, not its container")
		})

		t.Run("answers an ambiguous name with the candidates", func(t *testing.T) {
			t.Parallel()
			// A caller told only "ambiguous" retries with the same word.
			// One told where the three are corrects itself in the turn.
			got := documenting(t, &recorder{}, `{"scope":"a.fx","name":"Get","doc":"x"}`)
			assert.True(t, got.Failed(), "two candidates are not one declaration")
			assert.Contains(t, got.Error.Reason, "a.fx:4", "the refusal says where each one is")
			assert.Contains(t, got.Error.Reason, "a.fx:6",
				"counting from one, as an editor reports it, and not from the zero a span holds")
		})

		t.Run("narrows an ambiguous name by kind", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			got := documenting(t, writer, `{"scope":"a.fx","name":"Get","kind":"function","doc":"x"}`)
			assert.False(t, got.Failed(), "a kind narrows what a name leaves ambiguous")
			assert.Equal(t, writer.asked.Target.Span.Start.Offset, 50, "the function, not the method")
		})

		t.Run("offers back the name a typo was probably meant to be", func(t *testing.T) {
			t.Parallel()
			got := documenting(t, &recorder{}, `{"scope":"a.fx","name":"Stor","doc":"x"}`)
			assert.True(t, got.Failed(), "nothing is called Stor")
			assert.Contains(t, got.Error.Reason, "Store", "and the near name is offered back")
		})

		t.Run("offers back nothing for a name nothing resembles", func(t *testing.T) {
			t.Parallel()
			// Matching any name the query happens to contain would
			// answer a typo with every one-letter local in the file.
			got := documenting(t, &recorder{}, `{"scope":"a.fx","name":"zzzz","doc":"x"}`)
			assert.True(t, got.Failed(), "nothing is called zzzz")
			assert.NotContains(t, got.Error.Reason, "It declares",
				"a list of names that resemble nothing is noise")
		})

		t.Run("refuses a call carrying no prose to write", func(t *testing.T) {
			t.Parallel()
			got := documenting(t, &recorder{}, `{"scope":"a.fx","name":"Store","doc":""}`)
			assert.True(t, got.Failed(), "there is nothing to write")
			assert.Equal(t, got.Error.Code, "refused", "which the caller can correct")
		})
	})

	t.Run("dry_run", func(t *testing.T) {
		t.Parallel()

		t.Run("previews when the caller says nothing", func(t *testing.T) {
			t.Parallel()
			// Writing by default would make a mistyped call a change to
			// the workspace.
			writer := &recorder{}
			documenting(t, writer, `{"scope":"a.fx","name":"Store","doc":"x"}`)
			assert.True(t, writer.asked.DryRun, "the safe reading of silence is to write nothing")
		})

		t.Run("applies when the caller says so", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{}
			documenting(t, writer, `{"scope":"a.fx","name":"Store","doc":"x","dry_run":false}`)
			assert.False(t, writer.asked.DryRun, "a caller that asked to write is believed")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a preview as a diff and says what to call next", func(t *testing.T) {
			t.Parallel()
			out := tool.DocumentOutput{
				Symbol: "Store",
				Items: []tool.Changed{{
					Path: "a.fx", Sites: 1,
					Changes: []tool.Rewrite{{Line: 2, Was: "// Old.", Now: "// New."}},
				}},
				Verified: &tool.Gate{Gate: "parse", Engine: "treesitter/fx", Result: "pass"},
			}
			assert.ContainsInOrder(t, out.Render(), []string{
				"document Store — preview", "a.fx:2", "- // Old.", "+ // New.",
				"parses (treesitter/fx)", "dry_run false",
			}, "a change is read as a diff, and a passing preview is one call from being real")
		})

		t.Run("says nothing about a next call once it has been applied", func(t *testing.T) {
			t.Parallel()
			out := tool.DocumentOutput{
				Symbol: "Store", Applied: true,
				Verified: &tool.Gate{Gate: "parse", Engine: "e", Result: "pass"},
			}
			assert.Contains(t, out.Render(), "applied", "the caller is told it happened")
			assert.NotContains(t, out.Render(), "dry_run false", "and is not asked to do it again")
		})

		t.Run("names the gate that ran rather than only its verdict", func(t *testing.T) {
			t.Parallel()
			// "It parses" and "it builds" are different promises, and a
			// caller told only "pass" cannot tell which one it was given.
			out := tool.DocumentOutput{
				Symbol: "S", Applied: true,
				Verified: &tool.Gate{Gate: "parse", Engine: "treesitter/fx", Result: "pass"},
			}
			assert.Contains(t, out.Render(), "parses", "the gate says what it checked")
		})

		t.Run("says so when nothing judged the change", func(t *testing.T) {
			t.Parallel()
			out := tool.DocumentOutput{Symbol: "S", Applied: true}
			assert.Contains(t, out.Render(), "nothing checked",
				"a change nobody verified is not a change that passed")
		})
	})
}

// documenting runs the document tool over one fixture file and decodes what
// came back.
func documenting(t *testing.T, writer *recorder, input string) tool.DocumentOutput {
	t.Helper()
	built, err := tool.Document(reader{}, writer)
	assert.NoError(t, err, "the tool's schemas derive from its own types")

	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "a well-formed call reaches the service")

	var out tool.DocumentOutput
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the answer is JSON a caller can read")
	return out
}

// reader outlines one file holding a container, two methods on it and a
// function sharing one of their names.
type reader struct{}

func (reader) Outline(context.Context, engine.Request) (engine.Answer[sema.Symbol], error) {
	return engine.Answer[sema.Symbol]{
		Items: []sema.Symbol{
			declared("Store", sema.KindStruct, "", 1, 10),
			declared("Get", sema.KindMethod, "store", 3, 30),
			declared("Put", sema.KindMethod, "store", 4, 40),
			declared("Get", sema.KindFunction, "", 5, 50),
		},
		Status: trust.OK,
		Provenance: trust.Provenance{
			Engine: "reader", Fidelity: trust.Syntactic, Completeness: trust.ScopeTotal,
		},
	}, nil
}

// declared is one symbol the reader reports.
func declared(name string, kind sema.Kind, parent string, line, offset int) sema.Symbol {
	held := sema.Symbol{
		ID: sema.NewID("fx", "a", name, kind), Name: name, Kind: kind, Language: "fx",
		Span: source.Span{
			Path:  "a.fx",
			Start: source.Position{Offset: offset, Line: line},
			End:   source.Position{Offset: offset + 5, Line: line},
		},
	}
	if parent != "" {
		held.Parent = sema.NewID("fx", "a", "Store", sema.KindStruct)
	}
	return held
}

// recorder is a write path that records what it was asked and changes
// nothing.
type recorder struct{ asked edit.Request }

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
	}, nil
}
