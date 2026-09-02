// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

func TestRequest(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("asks for nothing and writes nothing", func(t *testing.T) {
			t.Parallel()
			// A request that forgot to say what it wants must not read
			// as one asking to write. DryRun false is the dangerous
			// default, so nothing else about the zero value may be
			// serviceable.
			var empty edit.Request
			assert.Empty(t, string(empty.Operation), "an unset request names no operation")
			assert.Equal(t, empty.Target.Kind, edit.TargetUnset, "and points at nothing")
			_, declared := edit.SpecFor(empty.Operation)
			assert.False(t, declared, "so it is refused before a planner is chosen")
		})
	})
}

func TestOutcome(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("reports that nothing was applied", func(t *testing.T) {
			t.Parallel()
			// Applied is the one field a caller has to read, so the
			// value it takes when nobody set it has to be the safe one.
			var nothing edit.Outcome
			assert.False(t, nothing.Applied, "a result nobody filled in did not change the workspace")
			assert.Empty(t, nothing.Changed, "and names no file as written")
			assert.Equal(t, nothing.Status, trust.Unset, "and claims no status")
		})
	})
}

func TestFinding(t *testing.T) {
	t.Parallel()

	t.Run("Fix", func(t *testing.T) {
		t.Parallel()

		t.Run("is absent where there is no one obvious change", func(t *testing.T) {
			t.Parallel()
			// Three payloads to save a round trip that may not be taken
			// is a bad trade, so a diagnostic with several plausible
			// remedies carries none.
			held := edit.Finding{Diagnostic: diag.Diagnostic{Message: "ambiguous"}}
			assert.Empty(t, held.Fix, "a finding with no obvious remedy carries no change")
		})

		t.Run("carries the change in the shape the write path takes", func(t *testing.T) {
			t.Parallel()
			// A fix a caller has to translate before applying is a fix
			// that costs the round trip it was meant to save.
			held := edit.Finding{
				Diagnostic: diag.Diagnostic{Message: "prefer FieldsSeq"},
				Fix: []edit.Change{{
					Kind: edit.ChangeEdit, Path: "a.go",
					Edits: []edit.TextEdit{{New: "strings.FieldsSeq"}},
				}},
			}
			plan := edit.Plan{Operation: edit.DocumentSymbol, Changes: held.Fix}
			assert.Equal(t, plan.Paths()[0], held.Fix[0].Path,
				"a fix is a change, so the write path reads it without translation")
		})
	})
}

func TestRewrite(t *testing.T) {
	t.Parallel()

	t.Run("Line", func(t *testing.T) {
		t.Parallel()

		t.Run("counts from one, as an editor reports it", func(t *testing.T) {
			t.Parallel()
			// Spans count from zero and editors count from one. A
			// preview is read by whoever opens the file, so it counts
			// their way.
			encoded, err := json.Marshal(edit.Rewrite{Path: "a.go", Line: 1, Now: "// Doc."})
			assert.NoError(t, err, "a rewrite is JSON a caller can read")
			assert.Contains(t, string(encoded), `"Line":1`, "the first line of a file is line one")
		})
	})

	t.Run("Was", func(t *testing.T) {
		t.Parallel()

		t.Run("is empty for an insertion", func(t *testing.T) {
			t.Parallel()
			held := edit.Rewrite{Path: "a.go", Line: 1, Now: "// Doc."}
			assert.Empty(t, held.Was, "an insertion replaces nothing, and says so by carrying nothing")
		})
	})
}
