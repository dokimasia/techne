// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package query

import (
	"context"
	"fmt"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Router reports which languages exist and which one claims a path.
//
// It is re-exported rather than redeclared: a composition root naming
// this service should not have to name the package the port lives in,
// and two declarations of one port are two things to keep in step.
type Router = engine.Router

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

// Search reports the declarations in a scope matching a query.
//
// The order is the engine's own. A language server ranks with more to go
// on than a parser has, and re-ranking here would throw that away.
func (s *Service) Search(
	ctx context.Context,
	req engine.Request,
	q engine.Query,
) (engine.Answer[sema.Symbol], error) {
	return ask(ctx, s, req, engine.RoleSearch,
		func(e engine.Engine) (engine.Result[sema.Symbol], error) {
			return e.(engine.Searcher).Search(ctx, req, q)
		})
}

// Resolve reports what the name at a position denotes.
//
// More than one item means the name is ambiguous and the caller chooses.
// An engine that binds through types returns one; a parser returns every
// declaration that happens to share the name, which is why the tier on
// the answer decides what the count is worth.
func (s *Service) Resolve(
	ctx context.Context,
	req engine.Request,
	at source.Position,
) (engine.Answer[sema.Symbol], error) {
	return ask(ctx, s, req, engine.RoleResolve,
		func(e engine.Engine) (engine.Result[sema.Symbol], error) {
			return e.(engine.Resolver).Resolve(ctx, req, at)
		})
}

// Relate reports how a symbol connects to the rest, in one direction.
//
// The direction asked for is the direction answered. An engine storing
// the other one returns the far end of what it stored rather than the
// near end of what it was asked, and the kind on each edge is the
// caller's word for it either way.
func (s *Service) Relate(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Answer[sema.Relation], error) {
	return ask(ctx, s, req, engine.RoleRelate,
		func(e engine.Engine) (engine.Result[sema.Relation], error) {
			return e.(engine.Relator).Relate(ctx, req, of, kind)
		})
}

// Verify reports what a language's own gate says about a scope.
//
// Suites names what to run in the language's own words. A scope holding
// two languages is verified by both and the findings merge, because a
// caller asking whether a directory builds is asking about the directory.
func (s *Service) Verify(
	ctx context.Context,
	req engine.Request,
	suites []string,
) (engine.Answer[edit.Finding], error) {
	return ask(ctx, s, req, engine.RoleVerify,
		func(e engine.Engine) (engine.Result[edit.Finding], error) {
			return e.(engine.Verifier).Verify(ctx, req, suites)
		})
}

// ask runs one read role through the shared path.
//
// It is generic over the item type so every role takes the same steps.
// Resolving the language, taking the engines strongest first and
// stamping what came back are [engine.AskEach]'s, so a read and a write
// cannot come to different conclusions about who serves a file. What is
// left here is the one thing a read does with several answers.
func ask[T any](
	ctx context.Context,
	s *Service,
	req engine.Request,
	role engine.Role,
	call func(engine.Engine) (engine.Result[T], error),
) (engine.Answer[T], error) {
	answered, err := engine.AskEach(ctx, s.catalog, s.router, req, role, call)
	if err != nil {
		return engine.Answer[T]{}, err
	}
	if len(answered) == 0 {
		return engine.Unsupported[T](fmt.Sprintf(
			"no engine serves %q for this role", req.Scope)), nil
	}
	return merge(answered, req.Preferred), nil
}
