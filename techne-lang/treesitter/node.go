// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

// NodeKind is a node kind as a grammar names it.
//
// The values here are the ones this package reads directly off the tree
// rather than through a query. A query names its own nodes and needs
// none of them; documentation and metadata do, because both are
// attached to a declaration rather than captured by it, and a pattern
// per grammar per shape would be unreadable.
type NodeKind string

// Node kinds that group the keywords qualifying a declaration.
//
// The set is taken from the grammars themselves rather than from the
// naming convention they mostly follow. Ruby breaks that convention:
// rescue_modifier, if_modifier and the rest are statement modifiers,
// which qualify an expression rather than a declaration, and reading
// their text would report `value if ready` as a keyword.
const (
	NodeModifiers             NodeKind = "modifiers"               // java, scala
	NodeModifier              NodeKind = "modifier"                // c#
	NodeAccessModifier        NodeKind = "access_modifier"         // scala
	NodeVisibilityModifier    NodeKind = "visibility_modifier"     // rust
	NodeFunctionModifiers     NodeKind = "function_modifiers"      // rust
	NodeExternModifier        NodeKind = "extern_modifier"         // rust
	NodeAccessibilityModifier NodeKind = "accessibility_modifier"  // typescript
	NodeOverrideModifier      NodeKind = "override_modifier"       // typescript
	NodeStorageClass          NodeKind = "storage_class_specifier" // c
	NodeTypeQualifier         NodeKind = "type_qualifier"          // c
	NodeFunctionSpecifier     NodeKind = "function_specifier"      // c
)

// Node kinds Scala gives each of its own keywords, beside the modifiers
// node that groups the ones it shares with the other languages.
const (
	NodeErasedModifier      NodeKind = "erased_modifier"
	NodeInfixModifier       NodeKind = "infix_modifier"
	NodeInlineModifier      NodeKind = "inline_modifier"
	NodeOpaqueModifier      NodeKind = "opaque_modifier"
	NodeOpenModifier        NodeKind = "open_modifier"
	NodeTrackedModifier     NodeKind = "tracked_modifier"
	NodeTransparentModifier NodeKind = "transparent_modifier"
)

// Node kinds that carry metadata written onto a declaration.
const (
	NodeAnnotation         NodeKind = "annotation"           // java, @Foo(...)
	NodeMarkerAnnotation   NodeKind = "marker_annotation"    // java, @Foo
	NodeDecorator          NodeKind = "decorator"            // python, typescript
	NodeAttributeItem      NodeKind = "attribute_item"       // rust, #[...]
	NodeInnerAttributeItem NodeKind = "inner_attribute_item" // rust, #![...]
	NodeAttributeList      NodeKind = "attribute_list"       // c#, [Foo]
)

// NodeAttribute is the one annotation inside a node that groups them.
// C# writes [Serializable, Obsolete] as one attribute_list holding two
// of these.
const NodeAttribute NodeKind = "attribute"

// Node kinds that exist only to hold a declaration together with what is
// written above or before it. Documentation and metadata sit on the
// wrapper rather than on the declaration inside it.
const (
	NodeDecoratedDefinition NodeKind = "decorated_definition" // python
	NodeExportStatement     NodeKind = "export_statement"     // javascript, typescript
	NodeAmbientDeclaration  NodeKind = "ambient_declaration"  // typescript
)

// Node kinds that make up a Python docstring: the first statement in a
// body, which is a bare string expression.
const (
	NodeExpressionStatement NodeKind = "expression_statement" // python
	NodeString              NodeKind = "string"               // python
)

// FieldName is a field a grammar names on one of its nodes.
type FieldName string

const (
	// FieldNameName is the field naming what a node declares. A node
	// carrying several is how Go writes `const a, b = 1, 2`.
	FieldNameName FieldName = "name"
	// FieldNameTag is Go's struct tag, written after the field.
	FieldNameTag FieldName = "tag"
	// FieldNameBody is the block a declaration's documentation sits in
	// where the language writes it inside rather than above.
	FieldNameBody FieldName = "body"
)

// Modifier is a keyword qualifying a declaration.
//
// A grammar spells most of these as anonymous tokens, which carry no
// field and no useful kind, so they are matched by the text they are
// written as. The set is shared across languages because the words do
// not collide: a grammar that has no such keyword produces no anonymous
// token for it, so the word simply never matches there.
type Modifier string

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

// modifierNodes group keywords into a node of their own.
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

// annotationNodes carry metadata written onto a declaration.
var annotationNodes = map[NodeKind]bool{
	NodeAnnotation:         true,
	NodeMarkerAnnotation:   true,
	NodeDecorator:          true,
	NodeAttributeItem:      true,
	NodeInnerAttributeItem: true,
	NodeAttributeList:      true,
}

// wrapperNodes hold a declaration together with what precedes it.
var wrapperNodes = map[NodeKind]bool{
	NodeDecoratedDefinition: true,
	NodeExportStatement:     true,
	NodeAmbientDeclaration:  true,
}

// modifierWords is the set matched against anonymous tokens.
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

// Punctuation each language wraps its metadata in, and what a Go struct
// tag is written with.
const (
	rustAttributeOpen      = "#["
	rustInnerAttributeOpen = "#!["
	attributeOpen          = "["
	attributeClose         = "]"
	annotationPrefix       = "@"
	// annotationBreak ends the name and begins the arguments.
	annotationBreak = "( \t\n{"
	// annotationQualifier separates the segments of a qualified name.
	annotationQualifier = ".:"
	// blockOpen is the punctuation a language groups with. A signature
	// never climbs across one, because a parent reaching this node
	// through one has opened something this node is inside.
	blockOpen = "{(["
	// bodyOpen is the brace a language opens a body with, and is where
	// a declaration whose body the grammar does not name ends.
	bodyOpen = '{'
	// bodyDepth bounds how far below a declaring node its body may sit.
	// Go puts a struct's fields two levels down; past that the search
	// would find the body of something nested inside the declaration.
	bodyDepth = 2
	// signatureTail is the punctuation a declaration opens its body
	// with, left behind when the body is removed.
	signatureTail = " \t\n\r{(=:->"
	// tagSeparator ends a Go struct tag's key.
	tagSeparator = ":"
	// tagQuote opens and closes a struct tag value, and tagEscape is
	// what stops one inside a value from closing it.
	tagQuote  = '"'
	tagEscape = '\\'
)
