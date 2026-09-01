// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
)

func TestKind(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("is the wire form of the kind", func(t *testing.T) {
			t.Parallel()
			// These strings are embedded in identities an index stores,
			// so the test pins them rather than deriving them.
			for kind, want := range map[sema.Kind]string{
				sema.KindUnknown:     "unknown",
				sema.KindModule:      "module",
				sema.KindPackage:     "package",
				sema.KindFile:        "file",
				sema.KindType:        "type",
				sema.KindStruct:      "struct",
				sema.KindEnum:        "enum",
				sema.KindEnumMember:  "enum-member",
				sema.KindConstructor: "constructor",
				sema.KindInterface:   "interface",
				sema.KindFunction:    "function",
				sema.KindMethod:      "method",
				sema.KindField:       "field",
				sema.KindVariable:    "variable",
				sema.KindConstant:    "constant",
			} {
				if got := kind.String(); got != want {
					t.Errorf("Kind(%d).String() = %q, want %q", kind, got, want)
				}
			}
		})

		t.Run("falls back to unknown outside the set", func(t *testing.T) {
			t.Parallel()
			// An engine that invents a kind must not produce an identity
			// containing an empty segment, which would collide with
			// every other malformed one.
			if got := sema.Kind(200).String(); got != "unknown" {
				t.Errorf("Kind(200).String() = %q, want %q", got, "unknown")
			}
		})

		t.Run("differs between kinds", func(t *testing.T) {
			t.Parallel()
			seen := map[string]sema.Kind{}
			for _, k := range []sema.Kind{
				sema.KindModule, sema.KindPackage, sema.KindFile, sema.KindType,
				sema.KindStruct, sema.KindEnum, sema.KindEnumMember,
				sema.KindInterface, sema.KindFunction, sema.KindMethod,
				sema.KindConstructor, sema.KindField, sema.KindVariable,
				sema.KindConstant,
			} {
				if prior, ok := seen[k.String()]; ok {
					t.Errorf("kinds %d and %d share the wire form %q", prior, k, k.String())
				}
				seen[k.String()] = k
			}
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("is unknown", func(t *testing.T) {
			t.Parallel()
			var unset sema.Kind
			if unset != sema.KindUnknown {
				t.Errorf("zero Kind = %d, want KindUnknown", unset)
			}
		})
	})
}
