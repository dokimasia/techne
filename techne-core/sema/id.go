// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import (
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/source"
)

// ID identifies a symbol across processes and across runs.
//
// The form is language:unit#qualified-name:kind, for example
// go:./internal/fsx#Digest:function.
type ID string

// NewID builds the identity of one declaration.
//
// It derives the identity from what the declaration is rather than from
// where it sits, so moving it inside its file leaves the ID unchanged
// and an index that stored it stays valid. Renaming the symbol or its
// unit does change the ID, which is correct: that is a different
// declaration.
func NewID(lang source.Language, unit source.Path, qualified string, kind Kind) ID {
	return ID(fmt.Sprintf("%s:%s#%s:%s", lang, unit, qualified, kind))
}

// Name is the qualified name inside an identity, and the empty string
// for a value that is not one.
//
// Read here rather than by whoever needs it, because the format is this
// package's: an identity taken apart somewhere else is a second copy of
// the format, and the two drift the moment a field is added.
//
// It exists because two engines answering about one declaration need not
// agree on its kind — a parser calls a method in a Scala object a
// function and the language server calls it a method — and an identity
// that differs in that field alone still names the same declaration. A
// caller matching on identity first and name second settles it, and the
// name is the part it cannot get at otherwise.
func (i ID) Name() string {
	_, after, found := strings.Cut(string(i), "#")
	if !found {
		return ""
	}
	if at := strings.LastIndex(after, ":"); at >= 0 {
		return after[:at]
	}
	return after
}
