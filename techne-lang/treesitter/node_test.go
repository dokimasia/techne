// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Node kinds, field names and modifier words are matched against what a
// grammar produces, so a constant that does not read exactly as the
// grammar spells it matches nothing and reports nothing. Whether each
// one is right for its grammar is settled by the conformance suite in
// the language modules, which is the only place a grammar exists. What
// is checked here is the shape those constants have to have.

func TestNodeKind(t *testing.T) {
	t.Parallel()

	t.Run("is spelled as the grammars spell it", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []treesitter.NodeKind{
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
		} {
			name := string(kind)
			assert.NotEmpty(t, name, "a node kind matched against nothing reports nothing")
			assert.Equal(t, name, strings.ToLower(name),
				"every grammar here names its nodes in lower-case snake case")
			assert.False(t, strings.ContainsAny(name, " \t-"),
				"every grammar here names its nodes in lower-case snake case")
		}
	})

	t.Run("differs between kinds", func(t *testing.T) {
		t.Parallel()
		// Two constants naming one node kind would leave one of them
		// matching nothing, silently.
		seen := map[treesitter.NodeKind]string{
			treesitter.NodeModifiers: "modifiers", treesitter.NodeModifier: "modifier",
			treesitter.NodeAnnotation: "annotation", treesitter.NodeAttribute: "attribute",
			treesitter.NodeAttributeList: "attribute_list", treesitter.NodeAttributeItem: "attribute_item",
		}
		assert.Length(t, seen, 6,
			"two names for one node kind would leave one of the two matching nothing")
	})
}

func TestFieldName(t *testing.T) {
	t.Parallel()

	t.Run("names a field a grammar declares", func(t *testing.T) {
		t.Parallel()
		for _, field := range []treesitter.FieldName{
			treesitter.FieldNameName, treesitter.FieldNameTag, treesitter.FieldNameBody,
		} {
			assert.NotEmpty(t, string(field),
				"a field name matched against nothing reads no field")
			assert.Equal(t, string(field), strings.ToLower(string(field)),
				"every grammar here names its fields in lower case")
		}
	})
}

func TestModifier(t *testing.T) {
	t.Parallel()

	t.Run("is the keyword as it is written in source", func(t *testing.T) {
		t.Parallel()
		// The word is matched against an anonymous token, which is the
		// keyword exactly as the language spells it. Rust's is "mut" and
		// not "mutable", and Go has none of these at all.
		for _, word := range []treesitter.Modifier{
			treesitter.ModifierPub, treesitter.ModifierMutable,
			treesitter.ModifierStatic, treesitter.ModifierFinal,
			treesitter.ModifierAsync, treesitter.ModifierReadonly,
		} {
			assert.NotEmpty(t, string(word), "a keyword matched against nothing matches nothing")
			assert.Equal(t, string(word), strings.TrimSpace(string(word)),
				"an anonymous token carries no surrounding space, so neither does the word")
		}
		assert.Equal(t, string(treesitter.ModifierMutable), "mut",
			"the word is what Rust writes, not what it means")
		assert.Equal(t, string(treesitter.ModifierPub), "pub",
			"the word is what Rust writes, not what it means")
	})
}
