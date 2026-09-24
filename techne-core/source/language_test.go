// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Language", func(t *testing.T) {
		t.Parallel()

		t.Run("is empty when zero", func(t *testing.T) {
			t.Parallel()
			var zero source.Language
			assert.Equal(t, zero, source.Language(""), "zero value")
		})

		t.Run("encodes as its value in JSON", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(source.Language("go"))
			assert.NoError(t, err, "marshal")
			assert.Equal(t, string(encoded), `"go"`, "encoding")
		})
	})
}
