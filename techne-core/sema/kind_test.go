// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
)

// wireForms pins the string every kind is written as. An identity an
// index stored embeds one of these, so they are listed rather than
// derived: a rename that a derivation would follow silently is exactly
// the change that invalidates stored data.
var wireForms = map[sema.Kind]string{
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

func TestKind(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("is the wire form of the kind", func(t *testing.T) {
			t.Parallel()
			for kind, want := range wireForms {
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
			for _, k := range sema.Kinds() {
				assert.False(t, seen[k.String()],
					"two kinds sharing a wire form would make two symbols one identity")
				seen[k.String()] = true
			}
		})
	})

	t.Run("Kinds", func(t *testing.T) {
		t.Parallel()

		t.Run("names every kind that carries a wire form", func(t *testing.T) {
			t.Parallel()
			listed := map[sema.Kind]bool{}
			for _, k := range sema.Kinds() {
				listed[k] = true
			}
			for kind := range wireForms {
				if kind == sema.KindUnknown {
					continue
				}
				assert.True(t, listed[kind],
					"a caller checking a value against the set must not meet a kind the set omits")
			}
		})

		t.Run("leaves out unknown", func(t *testing.T) {
			t.Parallel()
			for _, k := range sema.Kinds() {
				assert.NotEqual(t, k, sema.KindUnknown,
					"unknown is the absence of a classification, not one of them")
			}
		})
	})

	t.Run("Declares", func(t *testing.T) {
		t.Parallel()

		t.Run("is false for a binding that does not leave its scope", func(t *testing.T) {
			t.Parallel()
			for _, k := range []sema.Kind{
				sema.KindParameter, sema.KindTypeParameter,
				sema.KindLabel, sema.KindImport,
			} {
				assert.False(t, k.Declares(),
					"a caller listing what a file offers drops these in one check")
			}
		})

		t.Run("is false for unknown", func(t *testing.T) {
			t.Parallel()
			assert.False(t, sema.KindUnknown.Declares(),
				"a declaration nobody classified is not one a caller can refer to")
		})

		t.Run("is true for every other kind", func(t *testing.T) {
			t.Parallel()
			scoped := map[sema.Kind]bool{
				sema.KindParameter: true, sema.KindTypeParameter: true,
				sema.KindLabel: true, sema.KindImport: true,
			}
			for _, k := range sema.Kinds() {
				if scoped[k] {
					continue
				}
				assert.True(t, k.Declares(),
					"a kind naming something other code refers to is reported as declaring")
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
