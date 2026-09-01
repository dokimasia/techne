// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package query

import (
	"context"
	"errors"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Router reports which language claims a path.
//
// It is a port because the registry that knows lives in the language
// module's world, and core names no language. A registry satisfies it.
type Router interface {
	LanguageOf(p source.Path) (source.Language, bool)
}

// Service answers read questions.
//
// It is safe for concurrent use once built: nothing registers an engine
// or a language after a composition root has finished.
type Service struct {
	catalog *engine.Catalog
	router  Router
}

// New returns a service over a catalogue and a router.
func New(c *engine.Catalog, r Router) *Service {
	return &Service{catalog: c, router: r}
}

// Outline reports what the files in a scope declare.
func (s *Service) Outline(ctx context.Context, req engine.Request) (engine.Answer[sema.Symbol], error) {
	return ask(ctx, s, req, engine.RoleOutline,
		func(e engine.Engine) (engine.Result[sema.Symbol], error) {
			return e.(engine.Outliner).Outline(ctx, req)
		})
}

// ask runs one read role through the shared path.
//
// It is generic over the item type so every role takes the same steps.
// The caller supplies the one line that differs: which port to call.
func ask[T any](
	ctx context.Context,
	s *Service,
	req engine.Request,
	role engine.Role,
	call func(engine.Engine) (engine.Result[T], error),
) (engine.Answer[T], error) {
	language, known := s.language(req)
	if !known {
		return unsupported[T](), nil
	}

	for _, e := range s.catalog.For(ctx, language, role) {
		result, err := call(e)
		switch {
		case errors.Is(err, engine.ErrDecline):
			continue
		case err != nil:
			return engine.Answer[T]{}, err
		}
		return engine.Publish(result, e, role, req.Preferred), nil
	}
	return unsupported[T](), nil
}

// language resolves which language a request is about.
//
// A caller that named one is believed: it may know about a path the
// router does not. Otherwise the router decides, and a path no language
// claims resolves to nothing rather than to a guess.
func (s *Service) language(req engine.Request) (source.Language, bool) {
	if req.Language != "" {
		return req.Language, true
	}
	return s.router.LanguageOf(req.Scope)
}

// unsupported is the answer when nothing can be asked. It carries no
// payload, so an empty item list is never read as evidence of absence.
func unsupported[T any]() engine.Answer[T] {
	return engine.Answer[T]{Status: trust.Unsupported}
}
