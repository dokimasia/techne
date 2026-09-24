// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

// NodeKind is the kind of a node, as a grammar spells it. The engine reads
// the nodes of these kinds from the tree to find the metadata of a
// declaration, which a tags query does not capture.
type NodeKind string

// The kinds of the nodes that group the modifier keywords of a
// declaration. The set lists grammar node kinds, not every kind named
// modifier: Ruby's if_modifier and rescue_modifier qualify statements, not
// declarations.
const (
	NodeModifiers             NodeKind = "modifiers"               // Java, Scala
	NodeModifier              NodeKind = "modifier"                // C#
	NodeAccessModifier        NodeKind = "access_modifier"         // Scala
	NodeVisibilityModifier    NodeKind = "visibility_modifier"     // Rust
	NodeFunctionModifiers     NodeKind = "function_modifiers"      // Rust
	NodeExternModifier        NodeKind = "extern_modifier"         // Rust
	NodeAccessibilityModifier NodeKind = "accessibility_modifier"  // TypeScript
	NodeOverrideModifier      NodeKind = "override_modifier"       // TypeScript
	NodeStorageClass          NodeKind = "storage_class_specifier" // C
	NodeTypeQualifier         NodeKind = "type_qualifier"          // C
	NodeFunctionSpecifier     NodeKind = "function_specifier"      // C
)

// The kinds of the nodes that Scala gives each of its own keywords, in
// addition to the modifiers node that groups the keywords it shares with
// other languages.
const (
	NodeErasedModifier      NodeKind = "erased_modifier"
	NodeInfixModifier       NodeKind = "infix_modifier"
	NodeInlineModifier      NodeKind = "inline_modifier"
	NodeOpaqueModifier      NodeKind = "opaque_modifier"
	NodeOpenModifier        NodeKind = "open_modifier"
	NodeTrackedModifier     NodeKind = "tracked_modifier"
	NodeTransparentModifier NodeKind = "transparent_modifier"
)

// The kinds of the nodes that contain an annotation of a declaration.
const (
	NodeAnnotation         NodeKind = "annotation"           // Java @Foo(x)
	NodeMarkerAnnotation   NodeKind = "marker_annotation"    // Java @Foo
	NodeDecorator          NodeKind = "decorator"            // Python, TypeScript
	NodeAttributeItem      NodeKind = "attribute_item"       // Rust #[attr]
	NodeInnerAttributeItem NodeKind = "inner_attribute_item" // Rust #![attr]
	NodeAttributeList      NodeKind = "attribute_list"       // C# [Foo]
)

// NodeAttribute is the kind of one annotation in a group, as C# writes
// [Serializable, Obsolete] as an attribute_list with two attribute nodes.
const NodeAttribute NodeKind = "attribute"

// The kinds of the nodes that wrap a declaration together with what is
// written before it. The documentation and the annotations of the
// declaration belong to the wrapper.
const (
	NodeDecoratedDefinition NodeKind = "decorated_definition" // Python
	NodeExportStatement     NodeKind = "export_statement"     // JavaScript, TypeScript
	NodeAmbientDeclaration  NodeKind = "ambient_declaration"  // TypeScript
)

// The kinds of the nodes of a Python docstring: an expression statement,
// first in a body, that contains a string.
const (
	NodeExpressionStatement NodeKind = "expression_statement"
	NodeString              NodeKind = "string"
)

// FieldName is the name of a field of a node, as a grammar spells it.
type FieldName string

const (
	// FieldNameName is the field of a declared name. A node with more than
	// one declares each of them, as Go writes `const a, b = 1, 2`.
	FieldNameName FieldName = "name"
	// FieldNameTag is the field of the struct tag of a Go field.
	FieldNameTag FieldName = "tag"
	// FieldNameBody is the field of the body of a declaration.
	FieldNameBody FieldName = "body"
)

// Modifier is a modifier keyword, as a language writes it. Grammars write
// most modifiers as anonymous tokens, which the engine matches by text. The
// words of the languages do not collide, so one set serves every language.
type Modifier string

// The modifier keywords the engine reads.
const (
	ModifierAbstract     Modifier = "abstract"
	ModifierAsync        Modifier = "async"
	ModifierAuto         Modifier = "auto"
	ModifierCase         Modifier = "case"
	ModifierConst        Modifier = "const"
	ModifierDefault      Modifier = "default"
	ModifierExplicit     Modifier = "explicit"
	ModifierExport       Modifier = "export"
	ModifierExtern       Modifier = "extern"
	ModifierFinal        Modifier = "final"
	ModifierImplicit     Modifier = "implicit"
	ModifierInline       Modifier = "inline"
	ModifierInternal     Modifier = "internal"
	ModifierLazy         Modifier = "lazy"
	ModifierMutable      Modifier = "mut"
	ModifierNative       Modifier = "native"
	ModifierOpaque       Modifier = "opaque"
	ModifierOpen         Modifier = "open"
	ModifierOverride     Modifier = "override"
	ModifierPartial      Modifier = "partial"
	ModifierPrivate      Modifier = "private"
	ModifierProtected    Modifier = "protected"
	ModifierPublic       Modifier = "public"
	ModifierPub          Modifier = "pub"
	ModifierReadonly     Modifier = "readonly"
	ModifierRegister     Modifier = "register"
	ModifierRequired     Modifier = "required"
	ModifierRestrict     Modifier = "restrict"
	ModifierSealed       Modifier = "sealed"
	ModifierStatic       Modifier = "static"
	ModifierStrictfp     Modifier = "strictfp"
	ModifierSynchronized Modifier = "synchronized"
	ModifierTransient    Modifier = "transient"
	ModifierUnsafe       Modifier = "unsafe"
	ModifierVirtual      Modifier = "virtual"
	ModifierVolatile     Modifier = "volatile"
)

// modifierNodes contains the kinds of the nodes that group modifier
// keywords.
var modifierNodes = map[NodeKind]bool{
	NodeModifiers:             true,
	NodeModifier:              true,
	NodeAccessModifier:        true,
	NodeVisibilityModifier:    true,
	NodeFunctionModifiers:     true,
	NodeExternModifier:        true,
	NodeAccessibilityModifier: true,
	NodeOverrideModifier:      true,
	NodeStorageClass:          true,
	NodeTypeQualifier:         true,
	NodeFunctionSpecifier:     true,
	NodeErasedModifier:        true,
	NodeInfixModifier:         true,
	NodeInlineModifier:        true,
	NodeOpaqueModifier:        true,
	NodeOpenModifier:          true,
	NodeTrackedModifier:       true,
	NodeTransparentModifier:   true,
}

// annotationNodes contains the kinds of the nodes that contain an
// annotation.
var annotationNodes = map[NodeKind]bool{
	NodeAnnotation:         true,
	NodeMarkerAnnotation:   true,
	NodeDecorator:          true,
	NodeAttributeItem:      true,
	NodeInnerAttributeItem: true,
	NodeAttributeList:      true,
}

// wrapperNodes contains the kinds of the nodes that wrap a declaration.
var wrapperNodes = map[NodeKind]bool{
	NodeDecoratedDefinition: true,
	NodeExportStatement:     true,
	NodeAmbientDeclaration:  true,
}

// modifierWords contains the text of every Modifier, matched against
// anonymous tokens.
var modifierWords = map[string]bool{
	string(ModifierAbstract): true, string(ModifierAsync): true,
	string(ModifierAuto): true, string(ModifierCase): true,
	string(ModifierConst): true, string(ModifierDefault): true,
	string(ModifierExplicit): true, string(ModifierExport): true,
	string(ModifierExtern): true, string(ModifierFinal): true,
	string(ModifierImplicit): true, string(ModifierInline): true,
	string(ModifierInternal): true, string(ModifierLazy): true,
	string(ModifierMutable): true, string(ModifierNative): true,
	string(ModifierOpaque): true, string(ModifierOpen): true,
	string(ModifierOverride): true, string(ModifierPartial): true,
	string(ModifierPrivate): true, string(ModifierProtected): true,
	string(ModifierPublic): true, string(ModifierPub): true,
	string(ModifierReadonly): true, string(ModifierRegister): true,
	string(ModifierRequired): true, string(ModifierRestrict): true,
	string(ModifierSealed): true, string(ModifierStatic): true,
	string(ModifierStrictfp): true, string(ModifierSynchronized): true,
	string(ModifierTransient): true, string(ModifierUnsafe): true,
	string(ModifierVirtual): true, string(ModifierVolatile): true,
}

// The punctuation of annotations, struct tags and signatures, and the depth
// at which a signature finds its body.
const (
	rustAttributeOpen      = "#["
	rustInnerAttributeOpen = "#!["
	attributeOpen          = "["
	attributeClose         = "]"
	annotationPrefix       = "@"
	// annotationBreak ends the name of an annotation and starts its
	// arguments.
	annotationBreak = "( \t\n{"
	// annotationQualifier separates the segments of a qualified annotation
	// name.
	annotationQualifier = ".:"
	// blockOpen are the brackets that open a group. A signature does not
	// climb to a parent that writes one before the declaration.
	blockOpen = "{(["
	// bodyOpen opens the body of an aggregate whose grammar names no body
	// field.
	bodyOpen = '{'
	// bodyDepth is the number of levels below a declaring node at which
	// bodyOf looks for the body.
	bodyDepth = 2
	// signatureTail is the punctuation that trimmed removes from the end of
	// a signature. It excludes > and -, which end a generic such as
	// Box<dyn Error>.
	signatureTail = " \t\n\r{(=:"
	// tagSeparator ends the key of a struct tag pair.
	tagSeparator = ":"
	// tagQuote opens and closes the value of a struct tag pair, and
	// tagEscape escapes a quote inside it.
	tagQuote  = '"'
	tagEscape = '\\'
)
