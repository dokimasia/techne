;; Everything JavaScript declares, at any depth.
;;
;; Upstream captures class, function, method and one constant form, and
;; nothing else: no variable, no field, no parameter and no import.

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

(import_statement source: (string (string_fragment) @name)) @definition.import
(import_specifier name: (identifier) @name) @definition.import
(namespace_import (identifier) @name) @definition.import
(labeled_statement label: (statement_identifier) @name) @definition.label

;; A constructor is not a method.
(method_definition
  name: (property_identifier) @name
  (#eq? @name "constructor")) @definition.constructor

;; An object literal declares its properties: the binder gives the
;; literal an anonymous type and a symbol per key.
(pair key: [(property_identifier) @name (string (string_fragment) @name)]) @definition.field
(shorthand_property_identifier) @name @definition.field

;; Destructuring binds names without naming a declarator.
(object_pattern [(shorthand_property_identifier_pattern) @name
                 (pair_pattern value: (identifier) @name)]) @definition.variable
(array_pattern (identifier) @name) @definition.variable
(rest_pattern (identifier) @name) @definition.variable
(catch_clause parameter: (identifier) @name) @definition.variable
(for_in_statement left: (identifier) @name) @definition.variable
