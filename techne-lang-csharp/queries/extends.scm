;; Everything C# declares, at any depth.
;;
;; Upstream captures a class, an interface, a method and a namespace and
;; stops: no struct, no record, no enum, no member, no property, no
;; constructor, no field, no parameter and no using.

(class_declaration name: (identifier) @name) @definition.class
(struct_declaration name: (identifier) @name) @definition.struct
(record_declaration name: (identifier) @name) @definition.class
(interface_declaration name: (identifier) @name) @definition.interface
(enum_declaration name: (identifier) @name) @definition.enum
(enum_member_declaration name: (identifier) @name) @definition.enum_member
(delegate_declaration name: (identifier) @name) @definition.type

(method_declaration name: (identifier) @name) @definition.method
(constructor_declaration name: (identifier) @name) @definition.constructor
(destructor_declaration name: (identifier) @name) @definition.method
(property_declaration name: (identifier) @name) @definition.property
(event_declaration name: (identifier) @name) @definition.field
(indexer_declaration) @definition.property

(field_declaration
  (variable_declaration (variable_declarator name: (identifier) @name))) @definition.field

;; C# writes a constant as a const field, and the keyword sits on the
;; declaration rather than on each declarator. Both patterns match and
;; the parser keeps whichever kind says more.
(field_declaration
  (modifier) @mods
  (variable_declaration (variable_declarator name: (identifier) @name))
  (#eq? @mods "const")) @definition.constant

(local_declaration_statement
  (variable_declaration (variable_declarator name: (identifier) @name))) @definition.variable

(parameter name: (identifier) @name) @definition.parameter
(type_parameter (identifier) @name) @definition.type_parameter

;; A namespace and an import are named by their qualified name as
;; written, which is one node however many segments it has.
(namespace_declaration
  name: [(qualified_name) @name (identifier) @name]) @definition.module
(file_scoped_namespace_declaration
  name: [(qualified_name) @name (identifier) @name]) @definition.module
(using_directive
  [(qualified_name) @name (identifier) @name]) @definition.import
