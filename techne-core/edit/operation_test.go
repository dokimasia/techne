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

	t.Run("Operations", func(t *testing.T) {
		t.Parallel()

		t.Run("names every operation as verb.subject", func(t *testing.T) {
			t.Parallel()
			for _, op := range edit.Operations() {
				verb, subject, found := strings.Cut(string(op), ".")
				assert.True(t, found, "the dot of "+string(op))
				assert.NotEmpty(t, verb, "the verb of "+string(op))
				assert.NotEmpty(t, subject, "the subject of "+string(op))
			}
		})

		t.Run("lists each operation once", func(t *testing.T) {
			t.Parallel()
			assert.NoDuplicates(t, func() ([]edit.Operation, error) { return edit.Operations(), nil },
				"the operations")
		})

		t.Run("returns a new slice on each call", func(t *testing.T) {
			t.Parallel()
			first := edit.Operations()
			first[0] = edit.Operation("mutated")
			assert.NotEqual(t, edit.Operations()[0], edit.Operation("mutated"), "the first operation of a second call")
		})
	})
}
