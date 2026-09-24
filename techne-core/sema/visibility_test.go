// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
)

func TestVisibility(t *testing.T) {
	t.Parallel()

	words := map[sema.Visibility]string{
		sema.VisibilityUnknown: "unknown",
		sema.Unexported:        "unexported",
		sema.Exported:          "exported",
	}

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		t.Run("is VisibilityUnknown when zero", func(t *testing.T) {
			t.Parallel()
			var zero sema.Visibility
			assert.Equal(t, zero, sema.VisibilityUnknown, "zero value")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the pinned string of every visibility", func(t *testing.T) {
			t.Parallel()
			for v, want := range words {
				assert.Equal(t, v.String(), want, "wire string")
			}
		})

		t.Run("returns unknown for an undeclared value", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.Visibility(200).String(), "unknown", "wire string")
		})
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("round-trips every visibility", func(t *testing.T) {
			t.Parallel()
			for v, want := range words {
				encoded, err := json.Marshal(v)
				assert.NoError(t, err, "marshal")
				assert.Equal(t, string(encoded), `"`+want+`"`, "encoding")
				var decoded sema.Visibility
				assert.NoError(t, json.Unmarshal(encoded, &decoded), "unmarshal")
				assert.Equal(t, decoded, v, "round trip")
			}
		})
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes an unknown string to VisibilityUnknown", func(t *testing.T) {
			t.Parallel()
			got := sema.Exported
			assert.NoError(t, json.Unmarshal([]byte(`"internal"`), &got), "unmarshal")
			assert.Equal(t, got, sema.VisibilityUnknown, "decoded visibility")
		})
	})

	t.Run("Visibilities", func(t *testing.T) {
		t.Parallel()

		t.Run("lists every pinned visibility", func(t *testing.T) {
			t.Parallel()
			assert.Length(t, sema.Visibilities(), len(words), "listed visibilities")
			for _, v := range sema.Visibilities() {
				_, pinned := words[v]
				assert.True(t, pinned, v.String())
			}
		})
	})
}
