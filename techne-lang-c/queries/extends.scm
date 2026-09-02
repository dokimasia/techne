;; Everything C declares, at any depth.
;;
;; Upstream captures a function, a type and what it calls a class, and
;; nothing else: no struct, no union, no enum, no enumerator, no field,
;; no variable, no parameter and no macro.

;; A struct, union or enum specifier is both how one is declared and how
;; one is named afterwards: `struct Store { int size; }` declares it and
;; `struct Store *held` refers to it, and the two differ only by the
;; body. Matching the name alone reports a parameter's type as a second
;; declaration of it, which makes the name ambiguous and leaves nothing
;; able to address either.
(struct_specifier name: (type_identifier) @name body: (field_declaration_list)) @definition.struct
(union_specifier name: (type_identifier) @name body: (field_declaration_list)) @definition.union
(enum_specifier name: (type_identifier) @name body: (enumerator_list)) @definition.enum
(enumerator name: (identifier) @name) @definition.enum_member
(field_declaration declarator: (field_identifier) @name) @definition.field

;; A function returning a pointer wraps its declarator in one
;; pointer_declarator per star, so each depth is its own pattern. Two
;; covers everything short of a pointer to a pointer to a pointer.
(function_definition declarator: (function_declarator declarator: (identifier) @name)) @definition.function
(function_definition
  declarator: (pointer_declarator
    declarator: (function_declarator declarator: (identifier) @name))) @definition.function
(function_definition
  declarator: (pointer_declarator
    declarator: (pointer_declarator
      declarator: (function_declarator declarator: (identifier) @name)))) @definition.function

(declaration declarator: (function_declarator declarator: (identifier) @name)) @definition.function
(declaration
  declarator: (pointer_declarator
    declarator: (function_declarator declarator: (identifier) @name))) @definition.function
(declaration
  declarator: (pointer_declarator
    declarator: (pointer_declarator
      declarator: (function_declarator declarator: (identifier) @name)))) @definition.function
(type_definition declarator: (type_identifier) @name) @definition.type

(declaration declarator: (identifier) @name) @definition.variable
(declaration declarator: (init_declarator declarator: (identifier) @name)) @definition.variable
(parameter_declaration declarator: (identifier) @name) @definition.parameter
(parameter_declaration declarator: (pointer_declarator declarator: (identifier) @name)) @definition.parameter

(preproc_def name: (identifier) @name) @definition.constant
(preproc_function_def name: (identifier) @name) @definition.macro
(preproc_include path: [(string_literal) @name (system_lib_string) @name]) @definition.import
(labeled_statement label: (statement_identifier) @name) @definition.label
