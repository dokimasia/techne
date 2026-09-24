// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/tool"
)

// gate returns a passing gate of kind by the engine e.
func gate(kind, e string) *tool.Gate { return &tool.Gate{Gate: kind, Engine: e, Result: "pass"} }

func TestWrite(t *testing.T) {
	t.Parallel()

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("starts with the operation, the target and the state", func(t *testing.T) {
			t.Parallel()
			for _, one := range []tool.Written{
				{Operation: "rename.symbol", Target: "Kind"},
				{Operation: "move.file", Target: "a/b.go"},
				{Operation: "extract.function", Target: "parsed"},
			} {
				want := one.Operation + " " + one.Target + " — preview"
				assert.HasPrefix(t, one.Render(), want, "the render of "+one.Operation)
			}
		})

		t.Run("writes a preview as a diff with the call that applies it", func(t *testing.T) {
			t.Parallel()
			out := tool.Written{
				Operation: "document.symbol", Target: "Store",
				Items: []tool.Changed{{
					Path: "a.fx", Sites: 1,
					Changes: []tool.Rewrite{{Line: 2, Was: "// Old.", Now: "// New."}},
				}},
				Verified: gate("parse", "treesitter/fx"),
			}
			assert.ContainsInOrder(t, out.Render(), []string{
				"document.symbol Store — preview", "a.fx:2", "- // Old.", "+ // New.",
				"parses (treesitter/fx)", "dry_run false",
			}, "the render")
		})

		t.Run("writes the handle of a preview that has one", func(t *testing.T) {
			t.Parallel()
			out := tool.Written{
				Operation: "rename.symbol", Target: "S", Handle: "0a1b",
				Verified: gate("compile", "gopls"),
			}
			assert.Contains(t, out.Render(), "apply with apply.change handle 0a1b", "the render")
		})

		t.Run("writes no call for an applied change", func(t *testing.T) {
			t.Parallel()
			out := tool.Written{
				Operation: "document.symbol", Target: "Store", Applied: true,
				Verified: gate("parse", "e"),
			}
			assert.Contains(t, out.Render(), "applied", "the render")
			assert.NotContains(t, out.Render(), "dry_run false", "the render")
		})

		t.Run("writes the kind of the gate that passed", func(t *testing.T) {
			t.Parallel()
			out := tool.Written{
				Operation: "rename.symbol", Target: "S", Applied: true,
				Verified: gate("compile", "gopls"),
			}
			assert.Contains(t, out.Render(), "compiles (gopls)", "the render")
		})

		t.Run("writes nothing checked for a change without a gate", func(t *testing.T) {
			t.Parallel()
			out := tool.Written{Operation: "rename.symbol", Target: "Kind", Applied: true}
			assert.Contains(t, out.Render(), "nothing checked", "the render")
		})

		t.Run("writes the caveats of the gate", func(t *testing.T) {
			t.Parallel()
			checked := gate("compile", "rust-analyzer")
			checked.Caveats = []tool.Caveat{{Code: "partial-check", Note: "rust-analyzer does not check lifetimes"}}
			out := tool.Written{Operation: "extract.function", Target: "f", Verified: checked}
			want := []string{"compiles (rust-analyzer)", "rust-analyzer does not check lifetimes"}
			assert.ContainsInOrder(t, out.Render(), want, "the render")
		})

		t.Run("writes the evidence of the plan", func(t *testing.T) {
			t.Parallel()
			out := tool.Written{
				Operation: "rename.symbol", Target: "S", Verified: gate("compile", "gopls"),
				Provenance: tool.Provenance{
					Fidelity: "resolved", Completeness: "partial",
					Caveats: []tool.Caveat{{Code: "unrewritten", Note: "the server names a use at b.go:3"}},
				},
			}
			want := []string{"plan: resolved, partial coverage", "a use at b.go:3", "compiles"}
			assert.ContainsInOrder(t, out.Render(), want, "the render")
		})

		t.Run("writes no negative claim in the evidence of the plan", func(t *testing.T) {
			t.Parallel()
			out := tool.Written{
				Operation: "rename.symbol", Target: "S", Verified: gate("compile", "gopls"),
				Provenance: tool.Provenance{Fidelity: "resolved", Completeness: "total", SupportsNegativeClaim: true},
			}
			assert.NotContains(t, out.Render(), "an empty answer", "the render")
		})
	})

	t.Run("Execute", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the gate of the outcome with its caveats", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{gate: &trust.Provenance{
				Engine: "gopls", Fidelity: trust.Resolved, Completeness: trust.ScopeTotal,
				Caveats: []trust.Caveat{{Code: trust.CaveatDependents, Note: "the dependents were not checked"}},
			}}
			got := documenting(t, writer, `{"scope":"a.fx","name":"Store","doc":"x"}`)
			assert.NotNil(t, got.Verified, "the gate of the output")
			assert.Equal(t, got.Verified.Gate, "compile", "the kind of the gate")
			want := []tool.Caveat{{Code: "dependents", Note: "the dependents were not checked"}}
			assert.Equal(t, got.Verified.Caveats, want, "the caveats of the gate")
		})

		t.Run("returns a parse gate for a gate below the resolved tier", func(t *testing.T) {
			t.Parallel()
			writer := &recorder{gate: &trust.Provenance{Engine: "treesitter/fx", Fidelity: trust.Syntactic}}
			got := documenting(t, writer, `{"scope":"a.fx","name":"Store","doc":"x"}`)
			assert.Equal(t, got.Verified.Gate, "parse", "the kind of the gate")
		})
	})

	t.Run("Failed", func(t *testing.T) {
		t.Parallel()

		t.Run("returns false for a served change", func(t *testing.T) {
			t.Parallel()
			assert.False(t, tool.Written{Operation: "move.file"}.Failed(), "Failed")
		})

		t.Run("returns true for a change with an error", func(t *testing.T) {
			t.Parallel()
			assert.True(t, tool.Written{Error: &tool.Failure{Code: "refused"}}.Failed(), "Failed")
		})
	})

	t.Run("Touched", func(t *testing.T) {
		t.Parallel()

		planned := []edit.Change{
			{Kind: edit.ChangeEdit, Path: "a.fx"},
			{Kind: edit.ChangeEdit, Path: "b.fx"},
		}
		rewritten := []edit.Rewrite{
			{Path: "a.fx", Line: 1, Was: "one", Now: "two"},
			{Path: "b.fx", Line: 1, Was: "same", Now: "same"},
		}

		t.Run("returns the files that an applied change wrote", func(t *testing.T) {
			t.Parallel()
			got := tool.Touched(planned, rewritten, map[string]bool{"a.fx": true})
			assert.Length(t, got, 1, "the files")
			assert.Equal(t, got[0].Path, "a.fx", "the file")
		})

		t.Run("returns no file for an applied change that wrote none", func(t *testing.T) {
			t.Parallel()
			assert.Empty(t, tool.Touched(planned, rewritten, map[string]bool{}), "the files")
		})

		t.Run("returns every file of a preview", func(t *testing.T) {
			t.Parallel()
			assert.Length(t, tool.Touched(planned, rewritten, nil), 2, "the files")
		})

		t.Run("returns a moved file without a rewrite", func(t *testing.T) {
			t.Parallel()
			got := tool.Touched([]edit.Change{{Kind: edit.ChangeMove, Path: "a.fx", To: "b.fx"}}, nil, nil)
			assert.Equal(t, got, []tool.Changed{{Path: "a.fx", To: "b.fx"}}, "the files")
		})
	})
}
