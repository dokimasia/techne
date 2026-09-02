// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/tool"
)

func TestWritten(t *testing.T) {
	t.Parallel()

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("names the operation, so one shape reads for every write", func(t *testing.T) {
			t.Parallel()
			// An agent that has read one change reads the next without
			// learning a second answer.
			for _, one := range []tool.Written{
				{Operation: "rename.symbol", Target: "Kind"},
				{Operation: "move.file", Target: "a/b.go"},
				{Operation: "extract.function", Target: "parsed"},
			} {
				assert.HasPrefix(t, one.Render(), one.Operation+" "+one.Target+" — preview",
					"every write says what it did to what, in the same words")
			}
		})

		t.Run("says a change nothing judged is not one that passed", func(t *testing.T) {
			t.Parallel()
			held := tool.Written{Operation: "rename.symbol", Target: "Kind", Applied: true}
			assert.Contains(t, held.Render(), "nothing checked",
				"reporting no gate as a pass is the worst answer available")
		})

		t.Run("asks for a second call only while nothing has been written", func(t *testing.T) {
			t.Parallel()
			preview := tool.Written{
				Operation: "rename.symbol", Target: "Kind",
				Verified: &tool.Gate{Gate: "parse", Engine: "e", Result: "pass"},
			}
			assert.Contains(t, preview.Render(), "dry_run false", "a preview is one call from being real")

			applied := preview
			applied.Applied = true
			assert.NotContains(t, applied.Render(), "dry_run false",
				"and a change already made is not asked for again")
		})
	})

	t.Run("Failed", func(t *testing.T) {
		t.Parallel()

		t.Run("is what a transport reads, without parsing the payload", func(t *testing.T) {
			t.Parallel()
			// A transport that understood a domain shape would change
			// every time one did.
			assert.False(t, tool.Written{Operation: "move.file"}.Failed(),
				"a preview that ran is not a failure")
			assert.True(t, tool.Written{Error: &tool.Failure{Code: "refused"}}.Failed(),
				"and a refusal a model can correct is")
		})
	})
}

// A caller told a file changed acts on it. An applied change must name
// what it wrote and not what it planned, because the two differ where a
// plan's result was already there: a rename to the same name, a comment
// already written. A monkey run caught this as a change that said it
// wrote a file whose bytes never moved.
func TestTouchedAfterApplying(t *testing.T) {
	t.Parallel()

	planned := []edit.Change{
		{Kind: edit.ChangeEdit, Path: "a.fx"},
		{Kind: edit.ChangeEdit, Path: "b.fx"},
	}
	rewritten := []edit.Rewrite{
		{Path: "a.fx", Line: 1, Was: "one", Now: "two"},
		{Path: "b.fx", Line: 1, Was: "same", Now: "same"},
	}

	t.Run("an applied change", func(t *testing.T) {
		t.Parallel()

		t.Run("names the files it wrote and no other", func(t *testing.T) {
			t.Parallel()
			got := tool.Touched(planned, rewritten, map[string]bool{"a.fx": true})

			assert.Length(t, got, 1, "the file whose bytes moved")
			assert.Equal(t, got[0].Path, "a.fx", "and it is the one that was written")
		})

		t.Run("names nothing where it wrote nothing", func(t *testing.T) {
			t.Parallel()
			got := tool.Touched(planned, rewritten, map[string]bool{})

			assert.Empty(t, got, "a plan whose result was already there wrote no file")
		})
	})

	t.Run("a preview", func(t *testing.T) {
		t.Parallel()

		t.Run("names every file the change would touch", func(t *testing.T) {
			t.Parallel()
			// Nothing has been written, so what a caller reads is what
			// would happen rather than what did.
			got := tool.Touched(planned, rewritten, nil)

			assert.Length(t, got, 2, "both files the plan names")
		})
	})
}
