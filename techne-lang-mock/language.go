// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"errors"
	"path"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Language is the name of the mock language that [Register] registers.
const Language = "mock"

// Declaration returns the declaration of the mock language named name. The language claims
// the extension .name and the manifest name.manifest, and documentation starts with ;;. A
// test file ends in _test before its extension, a unit is the directory of a file, and a name
// that starts with an upper-case letter is exported.
func Declaration(name string) lang.Declaration {
	return lang.Declaration{
		Language:   source.Language(name),
		Extensions: []string{"." + name},
		Manifests:  []string{name + ".manifest"},
		Comment: lang.CommentStyle{
			Line: documents + " ",
			Doc:  []lang.DocStyle{{Open: documents}},
		},
		Blank:      map[string]bool{"_": true},
		IsTest:     IsTest,
		Namespace:  Namespace,
		Visibility: Visibility,
	}
}

// IsTest reports whether p is a test file: a base name that ends in _test before its
// extension.
func IsTest(p string) bool {
	return strings.HasSuffix(strings.TrimSuffix(p, path.Ext(p)), "_test")
}

// Namespace returns the directory of p, or the empty string for a file at the root.
func Namespace(p string) string {
	held := path.Dir(p)
	if held == "." {
		return ""
	}
	return held
}

// Option sets a value that a mock language claims.
type Option func(*Engine)

// At returns an option that sets the tier of every role to f.
func At(f trust.Fidelity) Option {
	return func(e *Engine) { e.fidelity = f }
}

// Covering returns an option that sets the completeness of every answer to c.
func Covering(c trust.Completeness) Option {
	return func(e *Engine) { e.coverage = c }
}

// Missing returns an option that makes the engine unavailable, with why as the reason.
func Missing(why string) Option {
	return func(e *Engine) { e.missing = why }
}

// Costing returns an option that sets the cost of every role to c.
func Costing(c engine.Cost) Option {
	return func(e *Engine) { e.cost = c }
}

// Registering returns the registration of the mock language named name, with opts applied to
// its engine. A composition root calls the registration as it calls the Register function of a
// language module. The registration returns an error for an empty name. The engine reads the
// filesystem of the workspace, so it does not need a root on disk.
func Registering(
	name string,
	opts ...Option,
) func(lang.Workspace, *lang.Registry, *engine.Catalog) error {
	return func(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
		if name == "" {
			return errors.New("mock: a mock language needs a name")
		}
		e, err := New(w.FS, Declaration(name), opts...)
		if err != nil {
			return err
		}
		return r.Register(c, e.declared, e)
	}
}

// Register registers the mock language named [Language].
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return Registering(Language)(w, r, c)
}
