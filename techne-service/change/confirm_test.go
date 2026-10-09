// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/service/change"
)

// testSuffix ends the name of a test file of the language fixture, which [verifier] skips for a
// request without tests.
const testSuffix = "_test.fx"

// subset is the caveat of a gate that checks less than the compiler of the language.
var subset = trust.Caveat{Code: trust.CaveatPartialCheck, Note: "the gate does not check borrows"}

// verifier is the verifier of the language fixture over the files of a workspace. Its tier is
// resolved unless the case sets one. It refuses a path without a file, and skips a test file for
// a request without tests. faults returns the errors of each file, and caveats are the caveats
// of each answer. When they are set, a file that contains breaks returns an error, and a file
// that contains skims returns a partial answer.
type verifier struct {
	files    *workspace
	fidelity trust.Fidelity
	faults   func(p source.Path, content string) []diag.Diagnostic
	caveats  []trust.Caveat
	breaks   string
	skims    string
}

func (*verifier) Name() string                          { return "verifier" }
func (*verifier) Language() source.Language             { return fixture }
func (v *verifier) Fidelity(engine.Role) trust.Fidelity { return cmp.Or(v.fidelity, trust.Resolved) }
func (*verifier) Cost(engine.Role) engine.Cost          { return engine.CostSession }

func (v *verifier) Verify(_ context.Context, req engine.Request, _ []string) (engine.Result[edit.Finding], error) {
	switch {
	case !v.files.held():
		return engine.Result[edit.Finding]{}, errors.New("the workspace was verified without its lock")
	case !v.files.has(req.Scope):
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: no file is at %s", engine.ErrRefuse, req.Scope)
	}
	content := v.files.at(req.Scope)
	out := engine.Result[edit.Finding]{Completeness: trust.ScopeTotal, Caveats: v.caveats}
	switch {
	case v.breaks != "" && strings.Contains(content, v.breaks):
		return engine.Result[edit.Finding]{}, errors.New("the verifier is broken")
	case !req.Tests && strings.HasSuffix(string(req.Scope), testSuffix):
		return out, nil
	case v.skims != "" && strings.Contains(content, v.skims):
		out.Completeness = trust.ScopePartial
		out.Caveats = append(slices.Clone(v.caveats), trust.Caveat{
			Code: trust.CaveatIndexWarming, Note: "the verifier read part of " + string(req.Scope),
		})
	}
	if v.faults != nil {
		for _, one := range v.faults(req.Scope, content) {
			out.Items = append(out.Items, edit.Finding{Diagnostic: one})
		}
	}
	return out, nil
}

// confirming returns a service over p, a resolved gate with the caveat subset that finds no
// error, and v over the workspace of the service.
func confirming(t *testing.T, p planner, v *verifier) (*workspace, *change.Service) {
	t.Helper()
	files, s := serving(t, p, checker{name: "server", fidelity: trust.Resolved, caveats: []trust.Caveat{subset}}, v)
	v.files = files
	return files, s
}

// warned returns the faults of a warning on each line that contains mark.
func warned(mark string) func(source.Path, string) []diag.Diagnostic {
	return func(p source.Path, content string) []diag.Diagnostic {
		out := marked(mark)(p, content)
		for i := range out {
			out[i].Severity = diag.SeverityWarning
		}
		return out
	}
}

func TestConfirm(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("takes back a change that adds an error on disk", func(t *testing.T) {
			t.Parallel()
			files, s := confirming(t, planner{}, &verifier{faults: marked("// Doc.")})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, got.Reason,
				"the change stops a.fx compiling after the write, and the files already written were put back",
				"the reason of the refusal")
			assert.False(t, got.Applied, "the application of the change")
			assert.Length(t, got.Diagnostics, 1, "the errors of the outcome")
			assert.Equal(t, got.Gate.Engine, "verifier", "the engine of the gate")
			assert.Equal(t, files.at("a.fx"), original, "the content of a.fx")
			assert.Equal(t, files.writes("a.fx"), 2, "the writes of a.fx")
		})

		t.Run("takes back a change to a test file that adds an error on disk", func(t *testing.T) {
			t.Parallel()
			tested := source.Path("a" + testSuffix)
			files, s := confirming(t, planner{changes: comment(tested, "Doc.")}, &verifier{faults: marked("// Doc.")})
			files.put(tested, original)
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, files.at(tested), original, "the content of the test file")
		})

		t.Run("keeps a change that adds no error on disk", func(t *testing.T) {
			t.Parallel()
			files, s := confirming(t, planner{}, &verifier{faults: marked("one")})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, got.Changed, []source.Path{"a.fx"}, "the files written")
			assert.Empty(t, got.Gate.Caveats, "the caveats of the gate")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "the content of a.fx")
		})

		t.Run("keeps a change that adds a warning on disk", func(t *testing.T) {
			t.Parallel()
			files, s := confirming(t, planner{}, &verifier{faults: warned("// Doc.")})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "the content of a.fx")
		})

		t.Run("puts back the files of a move that adds an error on disk", func(t *testing.T) {
			t.Parallel()
			files, s := confirming(t, planner{changes: relocated()}, &verifier{faults: marked("// Doc.")})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, files.at("a.fx"), original, "the content of a.fx")
			assert.False(t, files.has("b.fx"), "the file at b.fx")
		})

		t.Run("counts the errors of a moved file before the move", func(t *testing.T) {
			t.Parallel()
			files, s := confirming(t, planner{changes: []edit.Change{moving("b.fx")}}, &verifier{faults: marked("one")})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the move")
			assert.Empty(t, got.Gate.Caveats, "the caveats of the gate")
			assert.Equal(t, files.at("b.fx"), original, "the content of b.fx")
		})

		t.Run("keeps the caveat of a gate whose check on disk is partial", func(t *testing.T) {
			t.Parallel()
			files, s := confirming(t, planner{}, &verifier{faults: marked("// Doc."), skims: "// Doc."})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, got.Gate.Caveats, []trust.Caveat{subset, {
				Code: trust.CaveatPartialCheck,
				Note: "the check of the files on disk did not confirm the change: the verifier read part of a.fx",
			}}, "the caveats of the gate")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "the content of a.fx")
		})

		t.Run("keeps the caveat of a gate whose verifier checks less than the compiler", func(t *testing.T) {
			t.Parallel()
			shallow := trust.Caveat{Code: trust.CaveatPartialCheck, Note: "the verifier does not check borrows"}
			_, s := confirming(t, planner{}, &verifier{caveats: []trust.Caveat{shallow}})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, got.Gate.Caveats, []trust.Caveat{subset, {
				Code: trust.CaveatPartialCheck,
				Note: "the check of the files on disk did not confirm the change: the verifier does not check borrows",
			}}, "the caveats of the gate")
		})

		t.Run("keeps the caveat of a gate whose verifier checks below the tier resolved", func(t *testing.T) {
			t.Parallel()
			_, s := confirming(t, planner{}, &verifier{fidelity: trust.Syntactic, faults: marked("// Doc.")})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, got.Gate.Caveats, []trust.Caveat{subset, {
				Code: trust.CaveatPartialCheck,
				Note: "the check of the files on disk did not confirm the change: " +
					"verifier checks at the tier syntactic",
			}}, "the caveats of the gate")
		})

		t.Run("keeps the caveat of a gate whose check on disk misses a file", func(t *testing.T) {
			t.Parallel()
			files, s := confirming(t, planner{changes: append(comment("a.fx", "Doc."), comment("b.ot", "Doc.")...)},
				&verifier{})
			files.put("b.ot", original)
			req := asking(false)
			req.Language = ""
			got, err := s.Apply(t.Context(), req)
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, got.Gate.Caveats, []trust.Caveat{subset, {
				Code: trust.CaveatPartialCheck,
				Note: "the check of the files on disk did not confirm the change: no engine verifies b.ot",
			}}, "the caveats of the gate")
		})

		t.Run("keeps a change whose check on disk fails after the write", func(t *testing.T) {
			t.Parallel()
			files, s := confirming(t, planner{}, &verifier{breaks: "// Doc."})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, got.Gate.Caveats, []trust.Caveat{subset, {
				Code: trust.CaveatPartialCheck,
				Note: "the check of the files on disk failed: the verifier is broken",
			}}, "the caveats of the gate")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "the content of a.fx")
		})

		t.Run("returns the error of a check on disk that fails before the write", func(t *testing.T) {
			t.Parallel()
			files, s := confirming(t, planner{}, &verifier{breaks: "one"})
			_, err := s.Apply(t.Context(), asking(false))
			assert.HasError(t, err, "the error of Apply")
			assert.Equal(t, files.writes("a.fx"), 0, "the writes of a.fx")
		})

		t.Run("checks the files on disk under the lock of the workspace", func(t *testing.T) {
			t.Parallel()
			files, s := confirming(t, planner{}, &verifier{})
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.Empty(t, got.Gate.Caveats, "the caveats of the gate")
			assert.Equal(t, files.outside, 0, "the changes made without the lock")
		})

		t.Run("skips the check on disk after a gate that checks what the compiler checks", func(t *testing.T) {
			t.Parallel()
			v := &verifier{faults: marked("// Doc.")}
			files, s := serving(t, planner{}, checker{name: "compiler", fidelity: trust.Resolved}, v)
			v.files = files
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "the content of a.fx")
		})

		t.Run("skips the check on disk after a gate that leaves out the imports of a move", func(t *testing.T) {
			t.Parallel()
			move := []edit.Change{replacing("c.fx", 7, 13, "./b.fx"), moving("b.fx")}
			v := &verifier{faults: unresolved}
			files, s := serving(t,
				planner{name: "mover", fidelity: trust.Resolved, changes: move},
				checker{name: "compiler", fidelity: trust.Resolved, faults: unresolved},
				v,
			)
			v.files = files
			files.put("c.fx", "import ./a.fx\n")
			got, err := s.Apply(t.Context(), relocation())
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the move")
			assert.Equal(t, files.at("c.fx"), "import ./b.fx\n", "the content of c.fx")
		})

		t.Run("skips the check on disk for a dry run", func(t *testing.T) {
			t.Parallel()
			_, s := confirming(t, planner{}, &verifier{breaks: "one"})
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.NotEmpty(t, got.Handle, "the handle of the outcome")
			assert.Equal(t, got.Gate.Caveats, []trust.Caveat{subset}, "the caveats of the gate")
		})
	})
}
