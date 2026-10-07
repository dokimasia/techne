// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/treesitter"
)

// snake is the pattern of a node kind or a field name as the grammars spell them.
const snake = `^[a-z]+(_[a-z]+)*$`

func TestNode(t *testing.T) {
	t.Parallel()

	kinds := []treesitter.NodeKind{
		treesitter.NodeModifiers, treesitter.NodeModifier,
		treesitter.NodeAccessModifier, treesitter.NodeVisibilityModifier,
		treesitter.NodeFunctionModifiers, treesitter.NodeExternModifier,
		treesitter.NodeAccessibilityModifier, treesitter.NodeOverrideModifier,
		treesitter.NodeStorageClass, treesitter.NodeTypeQualifier,
		treesitter.NodeFunctionSpecifier,
		treesitter.NodeErasedModifier, treesitter.NodeInfixModifier,
		treesitter.NodeInlineModifier, treesitter.NodeOpaqueModifier,
		treesitter.NodeOpenModifier, treesitter.NodeTrackedModifier,
		treesitter.NodeTransparentModifier,
		treesitter.NodeAnnotation, treesitter.NodeMarkerAnnotation,
		treesitter.NodeAttribute, treesitter.NodeDecorator,
		treesitter.NodeAttributeItem, treesitter.NodeInnerAttributeItem,
		treesitter.NodeAttributeList,
		treesitter.NodeDecoratedDefinition, treesitter.NodeExportStatement,
		treesitter.NodeAmbientDeclaration,
		treesitter.NodeExpressionStatement, treesitter.NodeString,
	}

	t.Run("NodeKind", func(t *testing.T) {
		t.Parallel()

		t.Run("is spelled in lower snake case", func(t *testing.T) {
			t.Parallel()
			for _, kind := range kinds {
				assert.Matches(t, string(kind), snake, string(kind))
			}
		})

		t.Run("has a distinct value for each constant", func(t *testing.T) {
			t.Parallel()
			assert.NoDuplicates(t, func() ([]treesitter.NodeKind, error) { return kinds, nil }, "the node kinds")
		})
	})

	t.Run("FieldName", func(t *testing.T) {
		t.Parallel()

		t.Run("is spelled in lower snake case", func(t *testing.T) {
			t.Parallel()
			for _, field := range []treesitter.FieldName{
				treesitter.FieldNameName, treesitter.FieldNameTag, treesitter.FieldNameBody,
			} {
				assert.Matches(t, string(field), snake, string(field))
			}
		})
	})

	t.Run("Modifier", func(t *testing.T) {
		t.Parallel()

		t.Run("is the keyword as the language writes it", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, string(treesitter.ModifierMutable), "mut", "ModifierMutable")
			assert.Equal(t, string(treesitter.ModifierPub), "pub", "ModifierPub")
		})
	})
}
