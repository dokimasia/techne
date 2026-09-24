// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// recovering returns faults that mark each line that contains BAD, and the last line of a
// file that starts with a comment, as a grammar does when an edit changes how it recovers
// from a fault.
func recovering(p source.Path, content string) []diag.Diagnostic {
	out := marked("BAD")(p, content)
	if strings.HasPrefix(content, "// ") {
		out = append(out, fault(p, content, strings.Count(content, "\n")-1))
	}
	return out
}

// relocation returns a request to move a.fx to b.fx.
func relocation() edit.Request {
	return edit.Request{
		Operation: edit.MoveFile,
		Scope:     "a.fx",
		Language:  fixture,
		Target:    edit.Target{Kind: edit.TargetFile, Path: "a.fx"},
		Args:      edit.Args{edit.ArgDestination: "b.fx"},
	}
}

// unresolved returns an error at each use of ./b.fx, which a server reports for an import
// of a file that is not on disk, and at a line use a of a file that imports ./b.fx.
func unresolved(p source.Path, content string) []diag.Diagnostic {
	at := strings.Index(content, "./b.fx")
	if at < 0 {
		return nil
	}
	out := []diag.Diagnostic{{
		Severity: diag.SeverityError,
		Message:  "cannot find ./b.fx",
		Span: source.Span{
			Path:  p,
			Start: source.Position{Offset: at},
			End:   source.Position{Offset: at + len("./b.fx"), Column: at + len("./b.fx")},
		},
	}}
	for i, line := range strings.Split(content, "\n") {
		if line == "use a" {
			out = append(out, fault(p, content, i))
		}
	}
	return out
}

func TestGate(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a change that stops the file parsing", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, marking("// Doc."))
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Length(t, got.Diagnostics, 1, "the errors of the outcome")
			assert.Equal(t, got.Reason, "the change stops a.fx parsing, so it was not written",
				"the reason of the refusal")
			assert.Equal(t, files.at("a.fx"), original, "the content of a.fx")
		})

		t.Run("writes a change to a file whose errors the change did not add", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, marking("one"))
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "the content of a.fx")
		})

		t.Run("returns a degraded outcome for a change that no engine checks", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, got.Status, trust.Degraded, "the status of the outcome")
			assert.Nil(t, got.Gate, "the gate of the outcome")
			assert.NotEmpty(t, got.Provenance.Caveats, "the caveats of the outcome")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "the content of a.fx")
		})

		t.Run("returns the evidence of the engine that checked the change", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Gate.Engine, "clean", "the engine of the gate")
			assert.Equal(t, got.Gate.Fidelity, trust.Syntactic, "the tier of the gate")
		})

		t.Run("counts only the errors on the edited lines of a file with errors", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, checker{name: "parser", faults: recovering})
			files.put("a.fx", "one\nBAD\ntwo\n")
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
		})

		t.Run("counts every error of a file that parsed before the change", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, checker{name: "parser", faults: recovering})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, files.at("a.fx"), original, "the content of a.fx")
		})

		t.Run("refuses an error on an edited line of a file with errors", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, marking("BAD"))
			files.put("a.fx", "one\nBAD\ntwo\n")
			req := asking(false)
			req.Args = edit.Args{edit.ArgDoc: "BAD"}
			got, err := s.Apply(t.Context(), req)
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
		})

		t.Run("counts every error of a resolved gate", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, checker{name: "compiler", fidelity: trust.Resolved, faults: recovering})
			files.put("a.fx", "one\nBAD\ntwo\n")
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, got.Reason, "the change stops a.fx compiling, so it was not written",
				"the reason of the refusal")
		})

		t.Run("leaves out an error inside a rewritten import of a moved file", func(t *testing.T) {
			t.Parallel()
			move := []edit.Change{replacing("c.fx", 7, 13, "./b.fx"), moving("b.fx")}
			files, s := serving(t,
				planner{name: "mover", fidelity: trust.Resolved, changes: move},
				checker{name: "compiler", fidelity: trust.Resolved, faults: unresolved},
			)
			files.put("c.fx", "import ./a.fx\n")
			got, err := s.Apply(t.Context(), relocation())
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the move")
			assert.Equal(t, files.at("c.fx"), "import ./b.fx\n", "the content of c.fx")
			assert.Equal(t, got.Gate.Caveats, []trust.Caveat{{
				Code: trust.CaveatPartialCheck,
				Note: "the gate did not check the imports that the move rewrites, because a server resolves an " +
					"import on disk, where the moved file is not before the write: c.fx:1",
				Paths: []source.Path{"c.fx"},
			}}, "the caveats of the gate")
		})

		t.Run("refuses an error of a move outside the rewritten imports", func(t *testing.T) {
			t.Parallel()
			move := []edit.Change{replacing("c.fx", 7, 13, "./b.fx"), moving("b.fx")}
			files, s := serving(t,
				planner{name: "mover", fidelity: trust.Resolved, changes: move},
				checker{name: "compiler", fidelity: trust.Resolved, faults: unresolved},
			)
			files.put("c.fx", "import ./a.fx\nuse a\n")
			got, err := s.Apply(t.Context(), relocation())
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, files.at("a.fx"), original, "the content of a.fx")
		})
	})
}
