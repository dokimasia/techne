// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"fmt"
	"path"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
)

// Registry holds the declared languages and routes a path to one.
//
// A composition root builds one, registers every language module into
// it, and then serves requests. It is safe for concurrent reads once
// building is finished; nothing registers a language afterwards.
type Registry struct {
	declared map[source.Language]Declaration
	bySuffix map[string]source.Language
}

// NewRegistry returns a registry holding no languages.
func NewRegistry() *Registry {
	return &Registry{
		declared: map[source.Language]Declaration{},
		bySuffix: map[string]source.Language{},
	}
}

// Register records a language and adds its engines to the catalogue.
//
// It refuses an incomplete declaration, a language or an extension
// already claimed, and an engine answering about a different language.
// A refusal changes nothing: the catalogue is only touched once every
// check has passed, so a rejected module leaves no engines behind.
//
// A composition root calls this once per language. Nothing registers
// from an init function: the set of languages has to be a value the
// caller chooses, so that a test can build a registry holding one
// language and a smaller binary can ship a subset.
func (r *Registry) Register(cat *engine.Catalog, d Declaration, engines ...engine.Engine) error {
	if err := validate(d); err != nil {
		return err
	}
	if _, taken := r.declared[d.Language]; taken {
		return fmt.Errorf("lang: %q is already registered", d.Language)
	}
	for _, suffix := range d.Extensions {
		if owner, taken := r.bySuffix[suffix]; taken {
			return fmt.Errorf("lang: %q claims %q, already claimed by %q", d.Language, suffix, owner)
		}
	}
	for _, e := range engines {
		if e.Language() != d.Language {
			return fmt.Errorf("lang: %q declares engine %q, which answers about %q",
				d.Language, e.Name(), e.Language())
		}
	}

	for _, e := range engines {
		if err := cat.Add(e); err != nil {
			return fmt.Errorf("lang: %q: %w", d.Language, err)
		}
	}
	r.declared[d.Language] = d
	for _, suffix := range d.Extensions {
		r.bySuffix[suffix] = d.Language
	}
	return nil
}

// LanguageOf reports which language claims a path, by its extension.
//
// It reports false for a suffix nothing claimed and for a path with no
// extension. Guessing would answer about a language nothing declared.
func (r *Registry) LanguageOf(p source.Path) (source.Language, bool) {
	suffix := path.Ext(string(p))
	if suffix == "" {
		return "", false
	}
	l, claimed := r.bySuffix[suffix]
	return l, claimed
}

// Declaration returns what a language declared about itself.
func (r *Registry) Declaration(l source.Language) (Declaration, bool) {
	d, declared := r.declared[l]
	return d, declared
}

// Languages returns every registered language, so a caller can report
// what is served without knowing what was compiled in.
func (r *Registry) Languages() []source.Language {
	out := make([]source.Language, 0, len(r.declared))
	for l := range r.declared {
		out = append(out, l)
	}
	return out
}

// validate reports the first way a declaration is incomplete.
func validate(d Declaration) error {
	switch {
	case d.Language == "":
		return fmt.Errorf("lang: declaration names no language")
	case len(d.Extensions) == 0:
		return fmt.Errorf("lang: %q declares no extension, so no path routes to it", d.Language)
	case d.IsTest == nil:
		return fmt.Errorf("lang: %q declares no IsTest", d.Language)
	case d.Namespace == nil:
		return fmt.Errorf("lang: %q declares no Namespace", d.Language)
	case d.Visibility == nil:
		return fmt.Errorf("lang: %q declares no Visibility", d.Language)
	}
	for _, suffix := range d.Extensions {
		if !strings.HasPrefix(suffix, ".") {
			return fmt.Errorf("lang: %q declares extension %q, which has no leading dot", d.Language, suffix)
		}
	}
	return nil
}
