// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/treesitter"
)

func TestCapture(t *testing.T) {
	t.Parallel()

	t.Run("KindOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the kind of each definition capture", func(t *testing.T) {
			t.Parallel()
			for capture, want := range map[treesitter.Capture]sema.Kind{
				treesitter.DefinitionFunction:      sema.KindFunction,
				treesitter.DefinitionMethod:        sema.KindMethod,
				treesitter.DefinitionConstructor:   sema.KindConstructor,
				treesitter.DefinitionType:          sema.KindType,
				treesitter.DefinitionStruct:        sema.KindStruct,
				treesitter.DefinitionUnion:         sema.KindUnion,
				treesitter.DefinitionClass:         sema.KindStruct,
				treesitter.DefinitionObject:        sema.KindModule,
				treesitter.DefinitionEnum:          sema.KindEnum,
				treesitter.DefinitionEnumMember:    sema.KindEnumMember,
				treesitter.DefinitionInterface:     sema.KindInterface,
				treesitter.DefinitionAnnotation:    sema.KindAnnotation,
				treesitter.DefinitionField:         sema.KindField,
				treesitter.DefinitionProperty:      sema.KindProperty,
				treesitter.DefinitionVariable:      sema.KindVariable,
				treesitter.DefinitionConstant:      sema.KindConstant,
				treesitter.DefinitionParameter:     sema.KindParameter,
				treesitter.DefinitionTypeParameter: sema.KindTypeParameter,
				treesitter.DefinitionImport:        sema.KindImport,
				treesitter.DefinitionLabel:         sema.KindLabel,
				treesitter.DefinitionPackage:       sema.KindPackage,
				treesitter.DefinitionModule:        sema.KindModule,
				treesitter.DefinitionMacro:         sema.KindMacro,
				treesitter.DefinitionImplement:     sema.KindImplementation,
			} {
				got, ok := treesitter.KindOf(capture)
				assert.True(t, ok, string(capture))
				assert.Equal(t, got, want, string(capture))
			}
		})

		t.Run("returns false for a capture without a kind", func(t *testing.T) {
			t.Parallel()
			captures := []treesitter.Capture{treesitter.Name, "definition.widget", "reference.call", "", "definition"}
			for _, c := range captures {
				_, ok := treesitter.KindOf(c)
				assert.False(t, ok, string(c))
			}
		})
	})

	t.Run("Definitions", func(t *testing.T) {
		t.Parallel()

		t.Run("lists only captures with a kind", func(t *testing.T) {
			t.Parallel()
			for _, c := range treesitter.Definitions() {
				_, ok := treesitter.KindOf(c)
				assert.True(t, ok, string(c))
			}
		})

		t.Run("lists every capture with the definition prefix once", func(t *testing.T) {
			t.Parallel()
			seen := map[treesitter.Capture]bool{}
			for _, c := range treesitter.Definitions() {
				assert.HasPrefix(t, string(c), treesitter.DefinitionPrefix, string(c))
				assert.False(t, seen[c], string(c))
				seen[c] = true
			}
		})
	})

	t.Run("MoreSpecific", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			candidate sema.Kind
			current   sema.Kind
			want      bool
		}{
			{
				name:      "returns true for a constant over a field",
				candidate: sema.KindConstant,
				current:   sema.KindField,
				want:      true,
			},
			{
				name:      "returns true for a parameter over a variable",
				candidate: sema.KindParameter,
				current:   sema.KindVariable,
				want:      true,
			},
			{
				name:      "returns true for any kind over KindUnknown",
				candidate: sema.KindImport,
				current:   sema.KindUnknown,
				want:      true,
			},
			{
				name:      "returns false for an import over a function",
				candidate: sema.KindImport,
				current:   sema.KindFunction,
			},
			{name: "returns false for a kind over itself", candidate: sema.KindMethod, current: sema.KindMethod},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, treesitter.MoreSpecific(tt.candidate, tt.current), tt.want, "MoreSpecific")
			})
		}
	})
}
