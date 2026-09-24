;; The declarations of Go source, at any depth.
;;
;; No pattern is anchored to source_file, so a declaration inside a
;; function body matches. A caller that wants the exported surface of a
;; file filters on Kind.Declares and Symbol.Parent. The fields of an
;; anonymous struct in a table-driven test are declarations, and a
;; generator that reads the test needs them.
;;
;; A grouped `var (...)` puts its specs inside a var_spec_list. A grouped
;; `const (...)` or `type (...)` contains its specs directly. A spec can
;; have several name fields, as `const a, b = 1, 2` has. A pattern binds a
;; capture once, so the parser reads those names from the node.

;; Callables.

(
  (comment)* @doc
  .
  (function_declaration
    name: (identifier) @name) @definition.function
  (#strip! @doc "^//\\s*")
  (#set-adjacent! @doc @definition.function)
)

(
  (comment)* @doc
  .
  (method_declaration
    name: (field_identifier) @name) @definition.method
  (#strip! @doc "^//\\s*")
  (#set-adjacent! @doc @definition.method)
)

;; A method belongs to the type of its receiver, which the syntax writes
;; outside the type. The receiver capture names that type through a
;; pointer and through type arguments, and qualifies the method with it.
(method_declaration
  receiver: (parameter_list
    (parameter_declaration
      type: [(type_identifier) @receiver
             (pointer_type (type_identifier) @receiver)
             (generic_type type: (type_identifier) @receiver)
             (pointer_type (generic_type type: (type_identifier) @receiver))]))
  name: (field_identifier) @name) @definition.method

;; Types. The struct and interface patterns match declarations that the
;; general pattern also matches, and the parser keeps the more specific
;; kind.

(type_spec
  name: (type_identifier) @name) @definition.type

(type_spec
  name: (type_identifier) @name
  type: (struct_type)) @definition.struct

(type_spec
  name: (type_identifier) @name
  type: (interface_type)) @definition.interface

(type_alias
  name: (type_identifier) @name) @definition.type

;; Members. The field patterns are not anchored, so they match the fields
;; of a named type and of an anonymous struct.

(field_declaration
  name: (field_identifier) @name) @definition.field

;; An embedded field declares its type: `struct { FileHeader }` declares
;; FileHeader. The negated field !name restricts the pattern to a field
;; without a name, so a named field does not match with its type as the
;; name.
(field_declaration
  !name
  type: [(type_identifier) @name
         (pointer_type (type_identifier) @name)
         (qualified_type name: (type_identifier) @name)]) @definition.field

(method_elem
  name: (field_identifier) @name) @definition.method

;; Bindings.

(const_spec
  name: (identifier) @name) @definition.constant

(var_spec
  name: (identifier) @name) @definition.variable

(short_var_declaration
  left: (expression_list
    (identifier) @name)) @definition.variable

;; Signatures.

(parameter_declaration
  name: (identifier) @name) @definition.parameter

(variadic_parameter_declaration
  name: (identifier) @name) @definition.parameter

(type_parameter_declaration
  name: (identifier) @name) @definition.type_parameter

;; Scope and imports. An import is named by its path. The identifier that
;; an import binds is the name in the package clause of the imported
;; package, and the parser does not read that package.

(import_spec
  path: (interpreted_string_literal) @name) @definition.import

(labeled_statement
  label: (label_name) @name) @definition.label

;; A range clause and a type switch bind names without a var or := of
;; their own.
(range_clause
  left: (expression_list
    (identifier) @name)) @definition.variable

(type_switch_statement
  (expression_list
    (identifier) @name)) @definition.variable
