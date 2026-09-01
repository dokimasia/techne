// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package edit_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
)

func TestOperation(t *testing.T) {
	t.Parallel()

	t.Run("Family", func(t *testing.T) {
		t.Parallel()

		t.Run("is the part before the dot", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, edit.RenameSymbol.Family(), edit.FamilyRename,
				"an operation named family.subject belongs to the family before the dot")
		})

		t.Run("groups the subjects that share a verb", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, edit.RenameSymbol.Family(), edit.RenameFile.Family(),
				"a family can be advertised or refused as a unit")
		})

		t.Run("is the whole name when there is no subject", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, edit.Operation("verify").Family(), edit.Family("verify"),
				"a verb needing no subject is its own family")
		})
	})

	t.Run("Operations", func(t *testing.T) {
		t.Parallel()

		t.Run("names every operation family.subject", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				family, subject, found := strings.Cut(string(op), ".")
				assert.True(t, found && family != "" && subject != "",
					"every operation is named family.subject, so a family groups its subjects")
			}
		})

		t.Run("declares each operation once", func(t *testing.T) {
			t.Parallel()
			seen := map[edit.Operation]bool{}
			for _, op := range edit.Operations() {
				assert.False(t, seen[op], "an operation declared twice would be reported twice")
				seen[op] = true
			}
		})

		t.Run("returns a copy a caller cannot corrupt", func(t *testing.T) {
			t.Parallel()
			first := edit.Operations()
			first[0] = edit.Operation("mutated")
			assert.NotEqual(t, edit.Operations()[0], edit.Operation("mutated"),
				"every capability report reads the catalogue, so a caller must not be able to reorder it")
		})
	})
}
