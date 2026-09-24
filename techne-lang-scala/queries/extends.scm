;; The declarations of Scala at any depth. The upstream query captures no
;; parameter, import, type parameter or field of a case class.

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

;; An import writes the segments of its path as children of the
;; declaration, each with the path field. A pattern on a repeated field
;; matches the first child with the field only, so the patterns select a
;; segment by its position. Each pattern binds the name that an import
;; brings into scope: the last segment of a path, the package of a wildcard
;; import, each name of a selector group, and the alias of a renamed name.
(import_declaration (identifier) @name .) @definition.import
(import_declaration (identifier) @name . (namespace_wildcard)) @definition.import
(import_declaration (namespace_selectors (identifier) @name)) @definition.import
(import_declaration (namespace_selectors (arrow_renamed_identifier alias: (identifier) @name))) @definition.import
(import_declaration (namespace_selectors (as_renamed_identifier alias: (identifier) @name))) @definition.import
(import_declaration (as_renamed_identifier alias: (identifier) @name)) @definition.import
