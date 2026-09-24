// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package wire

import (
	"encoding/json"
	"fmt"
)

// Names maps the values of one enum to their wire strings. It is immutable
// after New and safe for concurrent use.
type Names[T comparable] struct {
	words    map[T]string
	values   map[string]T
	fallback T
}

// New returns the Names for words. Values outside words encode as the string
// of fallback, and unknown strings decode to fallback.
//
// New panics if fallback has no string or if two values share a string. Both
// are programming errors in the declaring package.
func New[T comparable](fallback T, words map[T]string) Names[T] {
	values := make(map[string]T, len(words))
	for v, word := range words {
		if _, taken := values[word]; taken {
			panic(fmt.Sprintf("wire: %q maps to two values", word))
		}
		values[word] = v
	}
	if _, ok := words[fallback]; !ok {
		panic("wire: fallback has no string")
	}
	return Names[T]{words: words, values: values, fallback: fallback}
}

// String returns the wire string of v, or the fallback's string if v is not
// in the set.
func (n Names[T]) String(v T) string {
	if word, ok := n.words[v]; ok {
		return word
	}
	return n.words[n.fallback]
}

// Parse returns the value for s and true, or the fallback and false if s is
// unknown.
func (n Names[T]) Parse(s string) (T, bool) {
	v, ok := n.values[s]
	if !ok {
		return n.fallback, false
	}
	return v, true
}

// Marshal returns the JSON encoding of the wire string of v.
func (n Names[T]) Marshal(v T) ([]byte, error) {
	return json.Marshal(n.String(v))
}

// Unmarshal decodes a JSON string into *into. Unknown strings decode to the
// fallback. If b is not a JSON string, Unmarshal returns an error and leaves
// *into unchanged.
func (n Names[T]) Unmarshal(b []byte, into *T) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("wire: %w", err)
	}
	*into, _ = n.Parse(s)
	return nil
}
