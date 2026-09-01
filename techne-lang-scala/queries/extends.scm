;; Everything Scala declares, at any depth.
;;
;; Upstream is the fullest of the ten and still captures no parameter,
;; no import, no type parameter and no case-class field.

(class_definition name: (identifier) @name) @definition.class
(object_definition name: (identifier) @name) @definition.object
(trait_definition name: (identifier) @name) @definition.interface
(enum_definition name: (identifier) @name) @definition.enum
(simple_enum_case name: (identifier) @name) @definition.enum_member
(full_enum_case name: (identifier) @name) @definition.enum_member
(type_definition name: (type_identifier) @name) @definition.type

(function_definition name: (identifier) @name) @definition.function
(function_declaration name: (identifier) @name) @definition.function
(val_definition pattern: (identifier) @name) @definition.constant
(val_declaration name: (identifier) @name) @definition.constant
(var_definition pattern: (identifier) @name) @definition.variable
(var_declaration name: (identifier) @name) @definition.variable
(given_definition name: (identifier) @name) @definition.constant

(parameter name: (identifier) @name) @definition.parameter
(class_parameter name: (identifier) @name) @definition.field
(type_parameters name: (identifier) @name) @definition.type_parameter
(covariant_type_parameter name: (identifier) @name) @definition.type_parameter
(contravariant_type_parameter name: (identifier) @name) @definition.type_parameter
(package_clause (package_identifier) @name) @definition.package

;; An import names its path in repeated path fields with no node holding
;; them together, so the first segment is what a pattern can bind.
(import_declaration path: (identifier) @name) @definition.import
