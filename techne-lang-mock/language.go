// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"fmt"
	"path"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Language is what a mock language is called when nobody names one.
const Language = "mock"

// Declaration states the facts about one mock language.
//
// The name is the language and the extension both, so registering
// "alpha" claims .alpha and nothing else. Two of them in one workspace
// are two languages that route separately, which is what a merged answer
// and a per-language refusal need in order to be exercised.
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

// IsTest reports whether a path holds tests, by the suffix every
// language here spells differently and this one spells _test.
func IsTest(p string) bool {
	return strings.HasSuffix(strings.TrimSuffix(p, path.Ext(p)), "_test")
}

// Namespace is the directory a file sits in, which is as much of a
// module system as this language has.
func Namespace(p string) string {
	held := path.Dir(p)
	if held == "." {
		return ""
	}
	return held
}

// Option narrows what one mock language claims.
type Option func(*Engine)

// At sets the tier this language answers at.
//
// Resolved is the default and is honest: within this language a use
// names a declaration, and the workspace is read whole. Setting it lower
// is how a workspace holds a language that only parses beside one that
// resolves, so the refusal between them is real.
func At(f trust.Fidelity) Option {
	return func(e *Engine) { e.fidelity = f }
}

// Covering sets how much of a scope this language reports examining.
//
// Partial is what a server still building its index reports, and it is
// what makes an operation that rewrites references refuse on evidence
// that looks strong enough until completeness is read as well.
func Covering(c trust.Completeness) Option {
	return func(e *Engine) { e.coverage = c }
}

// Missing makes the language declare itself unable to run, as one whose
// server is not installed does.
func Missing(why string) Option {
	return func(e *Engine) { e.missing = why }
}

// Costing sets what this language reports answering costs.
func Costing(c engine.Cost) Option {
	return func(e *Engine) { e.cost = c }
}

// Registering returns the registration for a language of this name.
//
// A composition root calls the result exactly as it calls a real
// language module's Register, so a mock language is registered the way
// every other language is rather than through a path of its own.
func Registering(
	name string,
	opts ...Option,
) func(lang.Workspace, *lang.Registry, *engine.Catalog) error {
	return func(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
		if name == "" {
			return fmt.Errorf("mock: a language needs a name")
		}
		// The workspace root is ignored. This language has no server to
		// point at one, which is the point of it: every tier it answers
		// at comes from the option it was built with rather than from
		// anything installed.
		e, err := New(w.FS, Declaration(name), opts...)
		if err != nil {
			return err
		}
		return r.Register(c, e.declared, e)
	}
}

// Register adds one mock language, called mock.
func Register(w lang.Workspace, r *lang.Registry, c *engine.Catalog) error {
	return Registering(Language)(w, r, c)
}
