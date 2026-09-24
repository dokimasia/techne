// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
)

func TestRelation(t *testing.T) {
	t.Parallel()

	words := map[sema.RelationKind]string{
		sema.RelationUnknown: "unknown",
		sema.Calls:           "calls",
		sema.CalledBy:        "called-by",
		sema.Implements:      "implements",
		sema.ImplementedBy:   "implemented-by",
		sema.References:      "references",
		sema.ReferencedBy:    "referenced-by",
		sema.Imports:         "imports",
		sema.ImportedBy:      "imported-by",
		sema.Embeds:          "embeds",
		sema.EmbeddedBy:      "embedded-by",
	}

	t.Run("RelationKind", func(t *testing.T) {
		t.Parallel()

		t.Run("is RelationUnknown when zero", func(t *testing.T) {
			t.Parallel()
			var zero sema.RelationKind
			assert.Equal(t, zero, sema.RelationUnknown, "zero value")
		})
	})

	t.Run("Inverse", func(t *testing.T) {
		t.Parallel()

		t.Run("pairs Calls with CalledBy", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.Calls.Inverse(), sema.CalledBy, "inverse of Calls")
			assert.Equal(t, sema.CalledBy.Inverse(), sema.Calls, "inverse of CalledBy")
		})

		t.Run("returns the original kind when applied twice", func(t *testing.T) {
			t.Parallel()
			for _, k := range sema.RelationKinds() {
				assert.Equal(t, k.Inverse().Inverse(), k, k.String())
			}
		})

		t.Run("never returns the kind itself", func(t *testing.T) {
			t.Parallel()
			for _, k := range sema.RelationKinds() {
				assert.NotEqual(t, k.Inverse(), k, k.String())
			}
		})

		t.Run("returns RelationUnknown for RelationUnknown", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.RelationUnknown.Inverse(), sema.RelationUnknown, "inverse")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the pinned string of every kind", func(t *testing.T) {
			t.Parallel()
			for k, want := range words {
				assert.Equal(t, k.String(), want, "wire string")
			}
		})

		t.Run("returns unknown for an undeclared value", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.RelationKind(200).String(), "unknown", "wire string")
		})
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("round-trips every kind", func(t *testing.T) {
			t.Parallel()
			for k, want := range words {
				encoded, err := json.Marshal(k)
				assert.NoError(t, err, "marshal")
				assert.Equal(t, string(encoded), `"`+want+`"`, "encoding")
				var decoded sema.RelationKind
				assert.NoError(t, json.Unmarshal(encoded, &decoded), "unmarshal")
				assert.Equal(t, decoded, k, "round trip")
			}
		})

		t.Run("encodes an undeclared value as unknown", func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(sema.RelationKind(200))
			assert.NoError(t, err, "marshal")
			assert.Equal(t, string(encoded), `"unknown"`, "encoding")
		})
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes an unknown string to RelationUnknown", func(t *testing.T) {
			t.Parallel()
			got := sema.Calls
			assert.NoError(t, json.Unmarshal([]byte(`"overrides"`), &got), "unmarshal")
			assert.Equal(t, got, sema.RelationUnknown, "decoded kind")
		})
	})
}
