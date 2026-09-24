// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
)

// kindWords pins the wire strings. Stored IDs embed them, so the test lists
// them instead of deriving them from the code under test.
var kindWords = map[sema.Kind]string{
	sema.KindUnknown:        "unknown",
	sema.KindModule:         "module",
	sema.KindPackage:        "package",
	sema.KindFile:           "file",
	sema.KindType:           "type",
	sema.KindStruct:         "struct",
	sema.KindUnion:          "union",
	sema.KindEnum:           "enum",
	sema.KindEnumMember:     "enum-member",
	sema.KindInterface:      "interface",
	sema.KindAnnotation:     "annotation",
	sema.KindFunction:       "function",
	sema.KindMethod:         "method",
	sema.KindConstructor:    "constructor",
	sema.KindProperty:       "property",
	sema.KindMacro:          "macro",
	sema.KindImplementation: "implementation",
	sema.KindField:          "field",
	sema.KindVariable:       "variable",
	sema.KindConstant:       "constant",
	sema.KindParameter:      "parameter",
	sema.KindTypeParameter:  "type-parameter",
	sema.KindImport:         "import",
	sema.KindLabel:          "label",
}

// local are the kinds whose names are bound in one scope only.
var local = map[sema.Kind]bool{
	sema.KindParameter:     true,
	sema.KindTypeParameter: true,
	sema.KindLabel:         true,
	sema.KindImport:        true,
}

func TestKind(t *testing.T) {
	t.Parallel()

	t.Run("Kind", func(t *testing.T) {
		t.Parallel()

		t.Run("is KindUnknown when zero", func(t *testing.T) {
			t.Parallel()
			var zero sema.Kind
			assert.Equal(t, zero, sema.KindUnknown, "zero value")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the pinned string of every kind", func(t *testing.T) {
			t.Parallel()
			for kind, want := range kindWords {
				assert.Equal(t, kind.String(), want, "wire string")
			}
		})

		t.Run("returns unknown for an undeclared value", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.Kind(200).String(), "unknown", "wire string")
		})
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("round-trips every kind", func(t *testing.T) {
			t.Parallel()
			for kind, want := range kindWords {
				encoded, err := json.Marshal(kind)
				assert.NoError(t, err, "marshal")
				assert.Equal(t, string(encoded), `"`+want+`"`, "encoding")
				var decoded sema.Kind
				assert.NoError(t, json.Unmarshal(encoded, &decoded), "unmarshal")
				assert.Equal(t, decoded, kind, "round trip")
			}
		})
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes an unknown string to KindUnknown", func(t *testing.T) {
			t.Parallel()
			got := sema.KindStruct
			assert.NoError(t, json.Unmarshal([]byte(`"trait-alias"`), &got), "unmarshal")
			assert.Equal(t, got, sema.KindUnknown, "decoded kind")
		})

		t.Run("rejects a JSON number", func(t *testing.T) {
			t.Parallel()
			var got sema.Kind
			assert.HasError(t, json.Unmarshal([]byte(`5`), &got), "unmarshal of a number")
		})
	})

	t.Run("Kinds", func(t *testing.T) {
		t.Parallel()

		t.Run("lists every pinned kind except KindUnknown", func(t *testing.T) {
			t.Parallel()
			listed := map[sema.Kind]bool{}
			for _, k := range sema.Kinds() {
				listed[k] = true
			}
			assert.Length(t, listed, len(kindWords)-1, "listed kinds")
			assert.False(t, listed[sema.KindUnknown], "KindUnknown listed")
		})
	})

	t.Run("Declares", func(t *testing.T) {
		t.Parallel()

		t.Run("returns false for kinds bound in one scope", func(t *testing.T) {
			t.Parallel()
			for k := range local {
				assert.False(t, k.Declares(), k.String())
			}
		})

		t.Run("returns false for KindUnknown", func(t *testing.T) {
			t.Parallel()
			assert.False(t, sema.KindUnknown.Declares(), "KindUnknown")
		})

		t.Run("returns true for every other kind", func(t *testing.T) {
			t.Parallel()
			for _, k := range sema.Kinds() {
				if !local[k] {
					assert.True(t, k.Declares(), k.String())
				}
			}
		})
	})
}
