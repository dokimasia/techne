// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Worded returns the byte index of the first occurrence of name in text that is not part of a
// longer identifier, or -1 for none. An identifier is a run of Unicode letters, digits and
// underscores. An empty name returns -1.
func Worded(text, name string) int {
	if name == "" {
		return -1
	}
	for from := 0; ; {
		at := strings.Index(text[from:], name)
		if at < 0 {
			return -1
		}
		at += from
		before, _ := utf8.DecodeLastRuneInString(text[:at])
		after, _ := utf8.DecodeRuneInString(text[at+len(name):])
		if !identifying(before) && !identifying(after) {
			return at
		}
		from = at + 1
	}
}

// WordAt returns the identifier of text that contains the character at the byte offset or ends
// at it, or the empty string when none does. An offset outside text returns the empty string.
func WordAt(text string, offset int) string {
	if offset < 0 || offset > len(text) {
		return ""
	}
	start, end := offset, offset
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:start])
		if !identifying(r) {
			break
		}
		start -= size
	}
	for end < len(text) {
		r, size := utf8.DecodeRuneInString(text[end:])
		if !identifying(r) {
			break
		}
		end += size
	}
	return text[start:end]
}

// identifying reports whether r is a letter, a digit or an underscore. [utf8.RuneError], which
// the decoders of utf8 return at the end of a text and for a byte that is not UTF-8, is none of
// them.
func identifying(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
