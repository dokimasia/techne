;; Everything Java declares, at any depth.
;;
;; Nothing here is anchored. A class declared inside a method is still a
;; class, and a caller wanting only a file's surface filters on
;; Kind.Declares and Symbol.Parent.
;;
;; Upstream captures a class, an interface and a method and stops. It has
;; no enum, no record, no constructor, no field, no constant, no
;; parameter and no import.

;; Types.

(class_declaration
  name: (identifier) @name) @definition.class

(record_declaration
  name: (identifier) @name) @definition.class

(interface_declaration
  name: (identifier) @name) @definition.interface

(annotation_type_declaration
  name: (identifier) @name) @definition.annotation

(enum_declaration
  name: (identifier) @name) @definition.enum

(enum_constant
  name: (identifier) @name) @definition.enum_member

;; Callables.

(method_declaration
  name: (identifier) @name) @definition.method

(constructor_declaration
  name: (identifier) @name) @definition.constructor

(compact_constructor_declaration
  name: (identifier) @name) @definition.constructor

(annotation_type_element_declaration
  name: (identifier) @name) @definition.method

;; Members. A field declared static final is also matched as a constant,
;; and the parser keeps whichever kind says more.

(field_declaration
  declarator: (variable_declarator
    name: (identifier) @name)) @definition.field

(field_declaration
  (modifiers) @mods
  declarator: (variable_declarator
    name: (identifier) @name)
  (#match? @mods "static")
  (#match? @mods "final")) @definition.constant

(constant_declaration
  declarator: (variable_declarator
    name: (identifier) @name)) @definition.constant

;; Bindings inside a body.

(local_variable_declaration
  declarator: (variable_declarator
    name: (identifier) @name)) @definition.variable

;; Signatures.

(formal_parameter
  name: (identifier) @name) @definition.parameter

(spread_parameter
  (variable_declarator
    name: (identifier) @name)) @definition.parameter

(catch_formal_parameter
  name: (identifier) @name) @definition.parameter

(type_parameter
  (type_identifier) @name) @definition.type_parameter

;; A receiver parameter has no pattern here because it declares no new
;; name: `void g(A this, int y)` writes the type of this, and this is
;; already bound. The grammar gives the node no name field for the same
;; reason.

;; Scope and imports. An import is named by the path as written.

(package_declaration
  [(identifier) @name
   (scoped_identifier) @name]) @definition.package

(import_declaration
  [(identifier) @name
   (scoped_identifier) @name]) @definition.import

(labeled_statement
  (identifier) @name) @definition.label

;; `o instanceof String s` binds s, and a record pattern binds each
;; component it names. javac calls these binding and deconstruction
;; patterns; both declare names the rest of the method can use.
(instanceof_expression
  name: (identifier) @name) @definition.variable

(type_pattern
  (identifier) @name) @definition.variable

;; A module declaration in module-info.java.
(module_declaration
  [(identifier) @name (scoped_identifier) @name]) @definition.module

;; A lambda binds its parameters, written bare when there is one and in
;; an inferred_parameters list when there are several.
(lambda_expression
  parameters: (identifier) @name) @definition.parameter
(lambda_expression
  parameters: (inferred_parameters (identifier) @name)) @definition.parameter

;; A for-each and a try-with-resources each declare their binding.
(enhanced_for_statement
  name: (identifier) @name) @definition.variable
(resource
  name: (identifier) @name) @definition.variable

;; A field of an interface or an annotation type is implicitly static
;; final. The grammar gives it its own node, constant_declaration, so no
;; modifier has to be read to know it is one.
(constant_declaration
  declarator: (variable_declarator name: (identifier) @name)) @definition.constant
