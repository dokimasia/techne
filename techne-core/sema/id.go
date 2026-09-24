// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema

import (
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/source"
)

// ID identifies a declaration across processes and runs. Its form is
// language:unit#name:kind, as in go:./internal/fsx#Store.Digest:method.
type ID string

// NewID returns the ID of the declaration named qualified, of kind, in unit
// of lang. The qualified name is the name that [Qualify] builds from the
// declarations that contain the declaration. The ID does not depend on the
// position of the declaration, so an edit elsewhere in the file keeps it
// valid.
func NewID(lang source.Language, unit source.Path, qualified string, kind Kind) ID {
	return ID(fmt.Sprintf("%s:%s#%s:%s", lang, unit, qualified, kind))
}

// Qualify returns name qualified by the qualified name of its container: the
// two joined by a dot, or name alone for an empty container.
func Qualify(container, name string) string {
	if container == "" {
		return name
	}
	return container + "." + name
}

// Name returns the qualified name in i, or an empty string if i does not have
// the form that NewID returns. Engines can report one declaration with
// different kinds, so a caller compares the names of IDs whose kinds differ.
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

// Base returns the part of the qualified name in i after its last dot, such
// as Get for Store.Get. It returns a name without a dot whole, and an empty
// string if i does not have the form that NewID returns.
func (i ID) Base() string {
	name := i.Name()
	return name[strings.LastIndex(name, ".")+1:]
}
