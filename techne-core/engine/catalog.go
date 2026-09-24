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

// Catalog is the set of registered engines. Add is not safe for concurrent
// use. Build the catalog before serving requests, after which every other
// method is safe for concurrent use.
type Catalog struct {
	engines []Engine
	named   map[named]bool
}

// named identifies an engine within one language.
type named struct {
	language source.Language
	engine   string
}

// NewCatalog returns an empty Catalog.
func NewCatalog() *Catalog {
	return &Catalog{named: map[named]bool{}}
}

// Add registers an engine. It returns an error if an engine with the same
// name already serves the same language, because a provenance identifies the
// engine that answered by name. One name can serve more than one language,
// as typescript-language-server does for TypeScript and JavaScript.
func (c *Catalog) Add(e Engine) error {
	key := named{language: e.Language(), engine: e.Name()}
	if c.named[key] {
		return fmt.Errorf("engine: %q already serves %q", key.engine, key.language)
	}
	c.named[key] = true
	c.engines = append(c.engines, e)
	return nil
}

// For returns the engines that serve role for lang and can run now, highest
// fidelity first and cheapest first among equals. Engines of equal fidelity
// and cost keep their registration order. Callers try them in order and move
// to the next one when an engine returns ErrDecline.
func (c *Catalog) For(ctx context.Context, lang source.Language, role Role) []Engine {
	var usable []Engine
	for _, e := range c.engines {
		if e.Language() == lang && offers(e, role) && unavailable(ctx, e) == nil {
			usable = append(usable, e)
		}
	}
	slices.SortStableFunc(usable, func(a, b Engine) int {
		return cmp.Or(
			cmp.Compare(b.Fidelity(role), a.Fidelity(role)),
			cmp.Compare(a.Cost(role), b.Cost(role)),
		)
	})
	return usable
}

// Capability is one role that one engine serves for one language.
type Capability struct {
	Language source.Language
	Role     Role
	Engine   string
	Fidelity trust.Fidelity
	Cost     Cost
	// Available reports whether the engine can run now.
	Available bool
	// Unavailable is the reason the engine cannot run, or empty.
	Unavailable string
}

// Capabilities returns one Capability for every role that each engine
// offers, in registration order. It includes the engines that cannot run, with
// Available false and the error text in Unavailable, so a caller can tell a
// missing server from a missing capability.
func (c *Catalog) Capabilities(ctx context.Context) []Capability {
	var out []Capability
	for _, e := range c.engines {
		why := unavailable(ctx, e)
		for _, role := range Roles() {
			if !offers(e, role) {
				continue
			}
			capability := Capability{
				Language:  e.Language(),
				Role:      role,
				Engine:    e.Name(),
				Fidelity:  e.Fidelity(role),
				Cost:      e.Cost(role),
				Available: why == nil,
			}
			if why != nil {
				capability.Unavailable = why.Error()
			}
			out = append(out, capability)
		}
	}
	return out
}

// offers reports whether e serves role: it implements the role's port and
// declares a fidelity above None for it. An engine declines a role of a port
// it implements by declaring None.
func offers(e Engine, role Role) bool {
	return serves(e, role) && e.Fidelity(role) != trust.None
}

// unavailable returns why e cannot run, or nil. An engine that does not
// implement Available can always run.
func unavailable(ctx context.Context, e Engine) error {
	if a, ok := e.(Available); ok {
		return a.Available(ctx)
	}
	return nil
}

// serves reports whether e implements the port of role.
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
