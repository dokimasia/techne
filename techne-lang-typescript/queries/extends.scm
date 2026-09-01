;; Everything TypeScript declares, at any depth.
;;
;; The TypeScript grammar inherits JavaScript's, so this covers both: the
;; JavaScript forms a .ts file also uses, and what TypeScript adds. It is
;; a copy rather than a reference because go:embed cannot reach into
;; another module, and the javascript module is a separate one.

;; Types.

(class_declaration name: (type_identifier) @name) @definition.class
(abstract_class_declaration name: (type_identifier) @name) @definition.class
(interface_declaration name: (type_identifier) @name) @definition.interface
(enum_declaration name: (identifier) @name) @definition.enum
(enum_body (property_identifier) @name) @definition.enum_member
(enum_body (enum_assignment name: (property_identifier) @name)) @definition.enum_member
(type_alias_declaration name: (type_identifier) @name) @definition.type
(module name: [(identifier) @name (string) @name]) @definition.module
(internal_module name: [(identifier) @name (string) @name]) @definition.module

;; Callables.

(function_declaration name: (identifier) @name) @definition.function
(generator_function_declaration name: (identifier) @name) @definition.function
(function_signature name: (identifier) @name) @definition.function
(method_definition
  name: (property_identifier) @name
  (#not-eq? @name "constructor")) @definition.method
(method_definition name: (private_property_identifier) @name) @definition.method
(method_definition name: (property_identifier) @name (#eq? @name "constructor")) @definition.constructor
(method_signature name: [(property_identifier) @name (private_property_identifier) @name]) @definition.method
(abstract_method_signature name: (property_identifier) @name) @definition.method

;; Members.

(public_field_definition name: [(property_identifier) @name (private_property_identifier) @name]) @definition.field
(property_signature name: (property_identifier) @name) @definition.field

;; Bindings.

(lexical_declaration "const" (variable_declarator name: (identifier) @name) @definition.constant)
(lexical_declaration "let" (variable_declarator name: (identifier) @name) @definition.variable)
(variable_declaration (variable_declarator name: (identifier) @name) @definition.variable)

;; A constructor parameter carrying a modifier declares a field as well
;; as a parameter: TypeScript calls it a parameter property, and the
;; class gets a member of that name.

(required_parameter
  (accessibility_modifier)
  pattern: (identifier) @name) @definition.field

(required_parameter
  "readonly"
  pattern: (identifier) @name) @definition.field

;; Signatures.

(formal_parameters (required_parameter pattern: (identifier) @name) @definition.parameter)
(formal_parameters (optional_parameter pattern: (identifier) @name) @definition.parameter)

;; TypeScript always wraps a parameter in required_parameter or
;; optional_parameter, so there is no bare-identifier form to match here
;; as there is in JavaScript.
(arrow_function parameter: (identifier) @name) @definition.parameter
(type_parameter name: (type_identifier) @name) @definition.type_parameter

;; Scope and imports. An import is named by the path as written.

(import_statement source: (string (string_fragment) @name)) @definition.import
(import_specifier name: [(identifier) @name (string) @name]) @definition.import
(namespace_import (identifier) @name) @definition.import
(import_clause (identifier) @name) @definition.import
(labeled_statement label: (statement_identifier) @name) @definition.label

;; An object literal declares its properties: TypeScript gives the
;; literal an anonymous type and binds a symbol per key. A generator
;; reading a configuration object needs them.
(pair key: [(property_identifier) @name (string (string_fragment) @name)]) @definition.field
(shorthand_property_identifier) @name @definition.field

;; Destructuring binds names without naming a declarator: `const {a, b}`
;; and `function f({a}, [b])` each declare what they pull apart.
(object_pattern [(shorthand_property_identifier_pattern) @name
                 (pair_pattern value: (identifier) @name)]) @definition.variable
(array_pattern (identifier) @name) @definition.variable
(rest_pattern (identifier) @name) @definition.variable

(required_parameter
  pattern: (object_pattern [(shorthand_property_identifier_pattern) @name
                            (pair_pattern value: (identifier) @name)])) @definition.parameter
(optional_parameter
  pattern: (object_pattern [(shorthand_property_identifier_pattern) @name
                            (pair_pattern value: (identifier) @name)])) @definition.parameter
(required_parameter
  pattern: (array_pattern (identifier) @name)) @definition.parameter
(required_parameter
  pattern: (rest_pattern (identifier) @name)) @definition.parameter
(catch_clause parameter: (identifier) @name) @definition.variable
(for_in_statement left: (identifier) @name) @definition.variable
