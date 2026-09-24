// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
)

// Registry maps extensions to the declared languages. A composition root
// builds one and registers every language module before it serves requests.
// After that, the methods of a Registry are safe for concurrent use.
type Registry struct {
	declared    map[source.Language]Declaration
	byExtension map[string]source.Language
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		declared:    map[source.Language]Declaration{},
		byExtension: map[string]source.Language{},
	}
}

// Register records d and adds engines to cat. It checks d and engines
// before it changes r or cat, and returns an error when d is incomplete,
// when another module declared d.Language or one of d.Extensions, when an
// engine serves another language, or when two engines have one name.
//
// A composition root calls Register once per language module, so the set
// of languages is chosen by the caller: a test registers one language, and
// a smaller binary registers a subset.
func (r *Registry) Register(cat *engine.Catalog, d Declaration, engines ...engine.Engine) error {
	if err := validate(d); err != nil {
		return err
	}
	if _, taken := r.declared[d.Language]; taken {
		return fmt.Errorf("lang: %q is already registered", d.Language)
	}
	for _, suffix := range d.Extensions {
		if owner, taken := r.byExtension[suffix]; taken {
			return fmt.Errorf("lang: %q declares %q, which %q already declares", d.Language, suffix, owner)
		}
	}
	names := map[string]bool{}
	for _, e := range engines {
		if e.Language() != d.Language {
			return fmt.Errorf("lang: %q declares engine %q, which serves %q",
				d.Language, e.Name(), e.Language())
		}
		if names[e.Name()] {
			return fmt.Errorf("lang: %q declares engine %q twice", d.Language, e.Name())
		}
		names[e.Name()] = true
	}

	for _, e := range engines {
		if err := cat.Add(e); err != nil {
			return fmt.Errorf("lang: %q: %w", d.Language, err)
		}
	}
	r.declared[d.Language] = d
	for _, suffix := range d.Extensions {
		r.byExtension[suffix] = d.Language
	}
	return nil
}

// LanguageOf returns the language that declares the extension of p. It
// reports false for a path without an extension and for an extension that
// no language declares.
func (r *Registry) LanguageOf(p source.Path) (source.Language, bool) {
	suffix := path.Ext(string(p))
	if suffix == "" {
		return "", false
	}
	l, ok := r.byExtension[suffix]
	return l, ok
}

// Declaration returns the declaration of l, and reports whether l is
// registered.
func (r *Registry) Declaration(l source.Language) (Declaration, bool) {
	d, ok := r.declared[l]
	return d, ok
}

// Languages returns every registered language, sorted. A service that asks
// every language merges the answers in this order, so identical requests
// return identical answers.
func (r *Registry) Languages() []source.Language {
	out := make([]source.Language, 0, len(r.declared))
	for l := range r.declared {
		out = append(out, l)
	}
	slices.Sort(out)
	return out
}

// validate returns the first rule of [Declaration] that d breaks, or nil.
func validate(d Declaration) error {
	switch {
	case d.Language == "":
		return fmt.Errorf("lang: declaration has no language")
	case len(d.Extensions) == 0:
		return fmt.Errorf("lang: %q declares no extension", d.Language)
	case d.IsTest == nil:
		return fmt.Errorf("lang: %q declares no IsTest", d.Language)
	case d.Namespace == nil:
		return fmt.Errorf("lang: %q declares no Namespace", d.Language)
	case d.Visibility == nil:
		return fmt.Errorf("lang: %q declares no Visibility", d.Language)
	}
	for _, suffix := range d.Extensions {
		if !strings.HasPrefix(suffix, ".") {
			return fmt.Errorf("lang: %q declares extension %q without a leading dot", d.Language, suffix)
		}
	}
	return nil
}
