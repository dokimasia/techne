// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
)

func TestKind(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("is the wire form of the kind", func(t *testing.T) {
			t.Parallel()
			for kind, want := range map[sema.Kind]string{
				sema.KindUnknown:     "unknown",
				sema.KindModule:      "module",
				sema.KindPackage:     "package",
				sema.KindFile:        "file",
				sema.KindType:        "type",
				sema.KindStruct:      "struct",
				sema.KindEnum:        "enum",
				sema.KindEnumMember:  "enum-member",
				sema.KindInterface:   "interface",
				sema.KindFunction:    "function",
				sema.KindMethod:      "method",
				sema.KindConstructor: "constructor",
				sema.KindField:       "field",
				sema.KindVariable:    "variable",
				sema.KindConstant:    "constant",
			} {
				assert.Equal(t, kind.String(), want,
					"an identity an index stored embeds this string, so it is pinned rather than derived")
			}
		})

		t.Run("falls back to unknown outside the set", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.Kind(200).String(), "unknown",
				"an invented kind must not produce an identity with an empty segment")
		})

		t.Run("differs between kinds", func(t *testing.T) {
			t.Parallel()
			seen := map[string]bool{}
			for _, k := range []sema.Kind{
				sema.KindModule, sema.KindPackage, sema.KindFile, sema.KindType,
				sema.KindStruct, sema.KindEnum, sema.KindEnumMember,
				sema.KindInterface, sema.KindFunction, sema.KindMethod,
				sema.KindConstructor, sema.KindField, sema.KindVariable,
				sema.KindConstant,
			} {
				assert.False(t, seen[k.String()],
					"two kinds sharing a wire form would make two symbols one identity")
				seen[k.String()] = true
			}
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("is unknown", func(t *testing.T) {
			t.Parallel()
			var unset sema.Kind
			assert.Equal(t, unset, sema.KindUnknown,
				"a symbol nobody classified claims no kind")
		})
	})
}
