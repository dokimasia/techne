;; Everything Ruby declares, at any depth.
;;
;; Upstream captures a class, a module and a method and stops: no
;; constant, no attribute accessor, no parameter and no require.

(class name: [(constant) @name (scope_resolution name: (constant) @name)]) @definition.class
(singleton_class value: (constant) @name) @definition.class
(module name: [(constant) @name (scope_resolution name: (constant) @name)]) @definition.module
(method name: [(identifier) @name (setter (identifier) @name) (operator) @name]) @definition.method
(singleton_method name: [(identifier) @name (operator) @name]) @definition.method

(assignment left: (constant) @name) @definition.constant
(assignment left: (identifier) @name) @definition.variable
(assignment left: (instance_variable) @name) @definition.field
(assignment left: (class_variable) @name) @definition.field
(operator_assignment left: (identifier) @name) @definition.variable

(method_parameters [(identifier) @name
                    (optional_parameter name: (identifier) @name)
                    (keyword_parameter name: (identifier) @name)
                    (splat_parameter name: (identifier) @name)
                    (hash_splat_parameter name: (identifier) @name)
                    (block_parameter name: (identifier) @name)]) @definition.parameter
(block_parameters (identifier) @name) @definition.parameter

;; attr_reader :name and friends declare readers; the symbols name them.
(call
  method: (identifier) @accessor
  arguments: (argument_list (simple_symbol) @name)
  (#match? @accessor "^attr_(reader|writer|accessor)$")) @definition.property
