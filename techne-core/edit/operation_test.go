// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/techne/core/edit"
)

func TestOperation(t *testing.T) {
	t.Parallel()

	t.Run("Family", func(t *testing.T) {
		t.Parallel()

		t.Run("is the part before the dot", func(t *testing.T) {
			t.Parallel()
			if got := edit.RenameSymbol.Family(); got != edit.FamilyRename {
				t.Errorf("RenameSymbol.Family() = %q, want %q", got, edit.FamilyRename)
			}
		})

		t.Run("groups the subjects that share a verb", func(t *testing.T) {
			t.Parallel()
			if edit.RenameSymbol.Family() != edit.RenameFile.Family() {
				t.Error("rename.symbol and rename.file must share a family")
			}
		})

		t.Run("is the whole name when there is no subject", func(t *testing.T) {
			t.Parallel()
			if got := edit.Operation("verify").Family(); got != edit.Family("verify") {
				t.Errorf("Family() of an undotted name = %q, want %q", got, "verify")
			}
		})
	})

	t.Run("Operations", func(t *testing.T) {
		t.Parallel()

		t.Run("names every operation family.subject", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				family, subject, found := strings.Cut(string(op), ".")
				if !found || family == "" || subject == "" {
					t.Errorf("operation %q is not named family.subject", op)
				}
			}
		})

		t.Run("declares each operation once", func(t *testing.T) {
			t.Parallel()
			seen := map[edit.Operation]bool{}
			for _, op := range edit.Operations() {
				if seen[op] {
					t.Errorf("operation %q is declared twice", op)
				}
				seen[op] = true
			}
		})

		t.Run("returns a copy a caller cannot corrupt", func(t *testing.T) {
			t.Parallel()
			// The catalogue is read by every capability report. A caller
			// that sorted the result in place would reorder it for
			// everyone if the slice were shared.
			first := edit.Operations()
			first[0] = edit.Operation("mutated")
			if edit.Operations()[0] == edit.Operation("mutated") {
				t.Error("Operations returns a shared slice; a caller can corrupt the catalogue")
			}
		})
	})
}
