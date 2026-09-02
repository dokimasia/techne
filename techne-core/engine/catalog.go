// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Catalog holds the registered engines and orders them for a question.
//
// It is safe for concurrent reads once building is finished. [Catalog.Add]
// is called from a composition root before any request is served, and
// nothing adds an engine afterwards.
type Catalog struct {
	engines []Engine
	named   map[string]bool
}

// NewCatalog returns an empty catalogue.
func NewCatalog() *Catalog {
	return &Catalog{named: map[string]bool{}}
}

// Add registers an engine.
//
// It reports an error when an engine with the same name is already
// registered: a provenance names the engine that answered, so two
// engines sharing a name make an answer untraceable.
func (c *Catalog) Add(e Engine) error {
	name := e.Name()
	if c.named[name] {
		return fmt.Errorf("engine: %q is already registered", name)
	}
	c.named[name] = true
	c.engines = append(c.engines, e)
	return nil
}

// For returns the engines that can answer this role for this language,
// strongest evidence first and cheapest among equals.
//
// An engine that does not serve the role, serves another language, or
// cannot run is left out. Asking an engine is the caller's job: the
// order here is the order to try, and [ErrDecline] means move to the
// next.
func (c *Catalog) For(ctx context.Context, lang source.Language, role Role) []Engine {
	var usable []Engine
	for _, e := range c.engines {
		if e.Language() != lang || !serves(e, role) {
			continue
		}
		if reason := unavailable(ctx, e); reason != nil {
			continue
		}
		usable = append(usable, e)
	}

	slices.SortStableFunc(usable, func(a, b Engine) int {
		// Strongest evidence first, so the comparison is reversed.
		if byTier := cmp.Compare(b.Fidelity(role), a.Fidelity(role)); byTier != 0 {
			return byTier
		}
		return cmp.Compare(a.Cost(role), b.Cost(role))
	})
	return usable
}

// Capability is what one engine can answer for one language and role.
type Capability struct {
	Language source.Language
	Role     Role
	Engine   string
	Fidelity trust.Fidelity
	Cost     Cost
	// Available reports whether the engine can run now.
	Available bool
	// Unavailable says why it cannot, and is empty when it can.
	Unavailable string
}

// Capabilities reports every language and role an engine serves.
//
// It answers "can you rename this Python symbol" as a value, rather than
// leaving a caller to infer it from which tools exist. An engine that
// cannot run is reported with the reason rather than omitted, because a
// missing server is a different problem from a missing capability.
func (c *Catalog) Capabilities(ctx context.Context) []Capability {
	var out []Capability
	for _, e := range c.engines {
		for _, role := range Roles() {
			if !serves(e, role) {
				continue
			}
			capability := Capability{
				Language:  e.Language(),
				Role:      role,
				Engine:    e.Name(),
				Fidelity:  e.Fidelity(role),
				Cost:      e.Cost(role),
				Available: true,
			}
			if reason := unavailable(ctx, e); reason != nil {
				capability.Available = false
				capability.Unavailable = reason.Error()
			}
			out = append(out, capability)
		}
	}
	return out
}

// unavailable reports why an engine cannot run, or nil. An engine that
// does not implement [Available] has nothing outside the process to
// check and is always usable.
func unavailable(ctx context.Context, e Engine) error {
	gate, declared := e.(Available)
	if !declared {
		return nil
	}
	return gate.Available(ctx)
}

// serves reports whether an engine implements the port for a role.
//
// Selection is by type assertion, so an engine declines a role by not
// having the method. This is the one place the roles and the ports are
// mapped onto each other; a port added without a case here can never be
// selected.
func serves(e Engine, role Role) bool {
	switch role {
	case RoleOutline:
		_, ok := e.(Outliner)
		return ok
	case RoleSearch:
		_, ok := e.(Searcher)
		return ok
	case RoleResolve:
		_, ok := e.(Resolver)
		return ok
	case RoleRelate:
		_, ok := e.(Relator)
		return ok
	case RolePlan:
		_, ok := e.(Planner)
		return ok
	case RoleFormat:
		_, ok := e.(Formatter)
		return ok
	case RoleCheck:
		_, ok := e.(Checker)
		return ok
	case RoleVerify:
		_, ok := e.(Verifier)
		return ok
	case RoleIndex:
		_, ok := e.(Indexer)
		return ok
	case RoleUnset:
		return false
	default:
		return false
	}
}
