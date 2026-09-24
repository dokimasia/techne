;; The declarations of JavaScript at any depth. The upstream query captures
;; a class, a function, a method and one form of constant, and no variable,
;; field, parameter or import.

(class_declaration name: (identifier) @name) @definition.class
(class name: (identifier) @name) @definition.class
(function_declaration name: (identifier) @name) @definition.function
(generator_function_declaration name: (identifier) @name) @definition.function
(method_definition
  name: (property_identifier) @name
  (#not-eq? @name "constructor")) @definition.method
(field_definition property: (property_identifier) @name) @definition.field

(lexical_declaration "const" (variable_declarator name: (identifier) @name) @definition.constant)
(lexical_declaration "let" (variable_declarator name: (identifier) @name) @definition.variable)
(variable_declaration (variable_declarator name: (identifier) @name) @definition.variable)

(formal_parameters (identifier) @name @definition.parameter)
(formal_parameters (rest_pattern (identifier) @name) @definition.parameter)
(formal_parameters (assignment_pattern left: (identifier) @name) @definition.parameter)
(arrow_function parameter: (identifier) @name) @definition.parameter

;; An import declares its source and each name it binds: the default
;; binding, each named binding and the namespace binding. The patterns are
;; those of the TypeScript module.
(import_statement source: (string (string_fragment) @name)) @definition.import
(import_clause (identifier) @name) @definition.import
(import_specifier name: [(identifier) @name (string) @name]) @definition.import
(namespace_import (identifier) @name) @definition.import
(labeled_statement label: (statement_identifier) @name) @definition.label

;; A method named constructor is a constructor.
(method_definition
  name: (property_identifier) @name
  (#eq? @name "constructor")) @definition.constructor

;; An object literal declares its properties, because the TypeScript binder
;; gives the literal an anonymous type with a symbol for each key.
(pair key: [(property_identifier) @name (string (string_fragment) @name)]) @definition.field
(shorthand_property_identifier) @name @definition.field

;; A destructuring pattern declares each name it binds.
(object_pattern [(shorthand_property_identifier_pattern) @name
                 (pair_pattern value: (identifier) @name)]) @definition.variable
(array_pattern (identifier) @name) @definition.variable
(rest_pattern (identifier) @name) @definition.variable
(catch_clause parameter: (identifier) @name) @definition.variable
(for_in_statement left: (identifier) @name) @definition.variable
