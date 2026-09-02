;; Everything Rust declares, at any depth. Nothing is anchored: an item
;; inside a function is still an item, and a caller wanting only a
;; crate's surface filters on Kind.Declares and Symbol.Parent.
;;
;; Upstream calls a struct, an enum, a union and a type alias all
;; definition.class, collapsing four shapes into one, and captures no
;; constant, static, field, variant, parameter or use.

(struct_item name: (type_identifier) @name) @definition.struct
(enum_item name: (type_identifier) @name) @definition.enum
(enum_variant name: (identifier) @name) @definition.enum_member
(union_item name: (type_identifier) @name) @definition.union
(type_item name: (type_identifier) @name) @definition.type
(trait_item name: (type_identifier) @name) @definition.interface

(function_item name: (identifier) @name) @definition.function
;; An impl block attaches behaviour to a type and is named for the type
;; it is for, so `impl Encoder for Http` and `impl Http` both group under
;; Http. Capturing it is what puts an associated type and a method
;; inside it rather than beside the file's own declarations.
(impl_item type: (type_identifier) @name) @definition.implementation
(impl_item type: (generic_type type: (type_identifier) @name)) @definition.implementation

(impl_item body: (declaration_list (function_item name: (identifier) @name) @definition.method))
(trait_item body: (declaration_list (function_signature_item name: (identifier) @name) @definition.method))
(trait_item body: (declaration_list (function_item name: (identifier) @name) @definition.method))

(const_item name: (identifier) @name) @definition.constant
(static_item name: (identifier) @name) @definition.variable
(field_declaration name: (field_identifier) @name) @definition.field
(mod_item name: (identifier) @name) @definition.module
(macro_definition name: (identifier) @name) @definition.macro

(let_declaration pattern: (identifier) @name) @definition.variable
(parameter pattern: (identifier) @name) @definition.parameter
(type_parameter name: (type_identifier) @name) @definition.type_parameter
(lifetime_parameter name: (lifetime (identifier) @name)) @definition.type_parameter
(const_parameter name: (identifier) @name) @definition.type_parameter
(use_declaration argument: (identifier) @name) @definition.import
(use_declaration argument: (scoped_identifier name: (identifier) @name)) @definition.import
(extern_crate_declaration name: (identifier) @name) @definition.import

;; A use tree brings in one name per leaf, and the leaves nest: a group
;; `use foo::{A, B::C}` imports A and C. Each leaf is matched wherever it
;; sits rather than only at the top of the declaration.
(use_list [(identifier) @name
           (scoped_identifier name: (identifier) @name)]) @definition.import
(use_as_clause alias: (identifier) @name) @definition.import
