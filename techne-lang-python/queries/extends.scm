;; Everything Python declares, at any depth.
;;
;; Nothing here is anchored. A class built inside a function is still a
;; class, and its methods are still methods; a caller wanting only a
;; module's surface filters on Kind.Declares and Symbol.Parent. Python
;; nests freely, so anchoring would decide that question here for every
;; caller and lose the answer for the ones who wanted it.

;; Types and callables. A function inside a class body is a method, and
;; the parser keeps whichever kind says more.

(class_definition
  name: (identifier) @name) @definition.class

(function_definition
  name: (identifier) @name) @definition.function

(class_definition
  body: (block
    (function_definition
      name: (identifier) @name) @definition.method))

(class_definition
  body: (block
    (decorated_definition
      definition: (function_definition
        name: (identifier) @name)) @definition.method))

;; Bindings. Python has no constant, so a binding is a variable, or a
;; field when a class body makes it one.

(assignment
  left: (identifier) @name) @definition.variable

(assignment
  left: (pattern_list
    (identifier) @name)) @definition.variable

(assignment
  left: (tuple_pattern
    (identifier) @name)) @definition.variable

(augmented_assignment
  left: (identifier) @name) @definition.variable

(named_expression
  name: (identifier) @name) @definition.variable

(class_definition
  body: (block
    (expression_statement
      (assignment
        left: (identifier) @name) @definition.field)))

;; A for target, a with alias and an except alias all bind names.

(for_statement
  left: [(identifier) @name
         (pattern_list (identifier) @name)
         (tuple_pattern (identifier) @name)]) @definition.variable

(for_in_clause
  left: [(identifier) @name
         (pattern_list (identifier) @name)
         (tuple_pattern (identifier) @name)]) @definition.variable

(as_pattern
  alias: (as_pattern_target
    (identifier) @name)) @definition.variable

(except_clause
  (identifier)
  (identifier) @name) @definition.variable

;; Signatures.

(parameters
  [(identifier) @name @definition.parameter
   (typed_parameter (identifier) @name) @definition.parameter
   (default_parameter name: (identifier) @name) @definition.parameter
   (typed_default_parameter name: (identifier) @name) @definition.parameter
   (list_splat_pattern (identifier) @name) @definition.parameter
   (dictionary_splat_pattern (identifier) @name) @definition.parameter])

(lambda_parameters
  [(identifier) @name @definition.parameter
   (default_parameter name: (identifier) @name) @definition.parameter])

;; Imports. An import is named by what it brings in.

(import_statement
  name: (dotted_name (identifier) @name)) @definition.import

(import_statement
  name: (aliased_import
    alias: (identifier) @name)) @definition.import

(import_from_statement
  name: (dotted_name (identifier) @name)) @definition.import

(import_from_statement
  name: (aliased_import
    alias: (identifier) @name)) @definition.import

;; Type aliases and type parameters.

(type_alias_statement
  (type) @name) @definition.type
