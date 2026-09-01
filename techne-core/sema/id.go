// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import (
	"fmt"

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
