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

		t.Run("maps a definition capture to the kind it declares", func(t *testing.T) {
			t.Parallel()
			// These names come from the tags queries the grammars ship.
			// The engine follows their convention rather than asking a
			// language module to rewrite its query.
			for capture, want := range map[treesitter.Capture]sema.Kind{
				treesitter.DefinitionFunction:      sema.KindFunction,
				treesitter.DefinitionMethod:        sema.KindMethod,
				treesitter.DefinitionConstructor:   sema.KindConstructor,
				treesitter.DefinitionType:          sema.KindType,
				treesitter.DefinitionStruct:        sema.KindStruct,
				treesitter.DefinitionUnion:         sema.KindUnion,
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
				got, declares := treesitter.KindOf(capture)
				assert.True(t, declares, "a capture the grammars' own tags queries use declares a symbol")
				assert.Equal(t, got, want, "a capture maps to the kind the shared vocabulary carries for it")
			}
		})

		t.Run("maps the shapes one kind carries onto that kind", func(t *testing.T) {
			t.Parallel()
			// A class, a Scala object and a Go struct are one shape here:
			// a named aggregate of fields and methods. Telling them apart
			// would mean a caller searching for that shape had to know
			// which language answered.
			for _, capture := range []treesitter.Capture{
				treesitter.DefinitionClass,
				treesitter.DefinitionObject,
				treesitter.DefinitionStruct,
			} {
				got, declares := treesitter.KindOf(capture)
				assert.True(t, declares, "each of these names an aggregate some grammar declares")
				assert.Equal(t, got, sema.KindStruct,
					"a caller searching for a shape must not have to know which language answered")
			}
		})

		t.Run("rejects the name capture, which declares nothing", func(t *testing.T) {
			t.Parallel()
			// @name is the identifier belonging to a definition, not a
			// definition itself. Treating it as one would emit a symbol
			// per name.
			_, declares := treesitter.KindOf(treesitter.Name)
			assert.False(t, declares, "the identifier belongs to a definition rather than being one")
		})

		t.Run("rejects a capture no grammar declares", func(t *testing.T) {
			t.Parallel()
			// A mistyped capture must not silently map to a kind. An
			// engine that found nothing is the hardest failure to
			// notice in a system whose job includes reporting that it
			// found nothing.
			for _, unknown := range []treesitter.Capture{
				"definition.widget", "reference.call", "", "definition",
			} {
				_, declares := treesitter.KindOf(unknown)
				assert.False(t, declares,
					"a mistyped capture mapping to a kind would produce an engine that silently finds nothing")
			}
		})

		t.Run("maps every declared definition capture", func(t *testing.T) {
			t.Parallel()
			// The list and the table are two places one convention is
			// written. A capture in one and not the other is a silent
			// gap.
			for _, c := range treesitter.Definitions() {
				_, declares := treesitter.KindOf(c)
				assert.True(t, declares, "the list and the table are one convention written twice")
			}
		})
	})

	t.Run("Definitions", func(t *testing.T) {
		t.Parallel()

		t.Run("excludes the name capture", func(t *testing.T) {
			t.Parallel()
			for _, c := range treesitter.Definitions() {
				assert.NotEqual(t, c, treesitter.Name, "the identifier capture declares no symbol")
			}
		})
	})
}
