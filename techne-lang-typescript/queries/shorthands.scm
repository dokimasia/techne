;; The name of each shorthand property of an object literal, which is the
;; key and the value of the property at once, as file in { file }.
;;
;; The grammar parses the shorthand of a destructuring pattern, which binds
;; a name, as shorthand_property_identifier_pattern, which this leaves out.
;; It is a copy of the query of the javascript module, because go:embed
;; cannot reach into another module.

(shorthand_property_identifier) @name
