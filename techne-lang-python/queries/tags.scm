;; Vendored verbatim from tree-sitter-python v0.25.0.
;; The engine follows this convention rather than asking a module to
;; rewrite its query, so this file is upstream's and not ours to edit.

(module (expression_statement (assignment left: (identifier) @name) @definition.constant))

(class_definition
  name: (identifier) @name) @definition.class

(function_definition
  name: (identifier) @name) @definition.function

(call
  function: [
      (identifier) @name
      (attribute
        attribute: (identifier) @name)
  ]) @reference.call
