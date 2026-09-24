// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"regexp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/treesitter"
)

// snake matches a node kind or a field name as the grammars spell them.
var snake = regexp.MustCompile(`^[a-z]+(_[a-z]+)*$`)

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
				assert.True(t, snake.MatchString(string(kind)), string(kind))
			}
		})

		t.Run("has a distinct value for each constant", func(t *testing.T) {
			t.Parallel()
			seen := map[treesitter.NodeKind]bool{}
			for _, kind := range kinds {
				assert.False(t, seen[kind], string(kind))
				seen[kind] = true
			}
		})
	})

	t.Run("FieldName", func(t *testing.T) {
		t.Parallel()

		t.Run("is spelled in lower snake case", func(t *testing.T) {
			t.Parallel()
			for _, field := range []treesitter.FieldName{
				treesitter.FieldNameName, treesitter.FieldNameTag, treesitter.FieldNameBody,
			} {
				assert.True(t, snake.MatchString(string(field)), string(field))
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
