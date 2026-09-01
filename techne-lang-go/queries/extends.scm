;; Everything Go declares, at any depth.
;;
;; Nothing here is anchored to source_file. A declaration inside a
;; function body is still a declaration, and a caller that wants only a
;; file's exported surface filters on Kind.Declares and Symbol.Parent.
;; Anchoring instead would decide that question here, for every caller,
;; and lose the answer for the ones who wanted it: an anonymous struct in
;; a table-driven test declares fields, and a generator reading that test
;; needs them.
;;
;; Two facts about the grammar drive the shapes below, both established
;; by parsing rather than assumed. A grouped `var (...)` puts its specs
;; inside a var_spec_list while a grouped `const (...)` and `type (...)`
;; hold theirs directly. And a spec can carry several name fields, as
;; `const a, b = 1, 2` does; the parser reads those off the node, because
;; a pattern binds a capture once.

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

;; Types. The struct and interface patterns match the same declaration
;; the general one does, and the parser keeps whichever kind says more.

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

;; Members. A field declaration reaches here from a named type and from
;; an anonymous struct alike, which is why it is not anchored.

(field_declaration
  name: (field_identifier) @name) @definition.field

;; An embedded field declares its type: `struct { FileHeader }` declares
;; FileHeader. It has no name field, which !name asserts, so this cannot
;; also match a named field and take its type for a name.
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

;; Scope and imports. An import is named by its path: the identifier it
;; binds is the package's own name, which a parser cannot know without
;; reading that package.

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
