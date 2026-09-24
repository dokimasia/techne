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

// Router maps a path to the languages that a request is about. It is [engine.Router], so
// the read path and the write path route a path by one rule.
type Router = engine.Router

// Service serves the read roles over one catalogue. It is safe for concurrent use after
// the composition root has registered every engine.
type Service struct {
	catalog *engine.Catalog
	router  Router
}

// New returns a service over the engines of c and the languages of r.
func New(c *engine.Catalog, r Router) *Service {
	return &Service{catalog: c, router: r}
}

// Outline returns the declarations of the files in the scope of req.
func (s *Service) Outline(ctx context.Context, req engine.Request) (engine.Answer[sema.Symbol], error) {
	return ask(ctx, s, req, engine.RoleOutline,
		func(e engine.Engine) (engine.Result[sema.Symbol], error) {
			return e.(engine.Outliner).Outline(ctx, req)
		})
}

// Search returns the declarations in the scope of req that match q. The matches of each
// language keep the order of its engine, and the languages follow the order of the router.
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

// Resolve returns the declarations that the name at a position denotes. Two or more items
// mean that the name is ambiguous. The tier of the answer states whether a type checker
// bound the name or a parser matched it.
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

// Relate returns the relations of kind from the declaration with the ID of, in the
// direction that kind names.
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

// Verify returns the findings of the toolchain of each language of the scope of req.
// suites names the checks in the words of the language. A scope of two languages returns
// the findings of both.
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

// ask runs one read role through [engine.AskEach] and merges the answers. It returns an
// unsupported answer with the reasons of the engines that declined when every answer is
// skipped. It returns an unsupported answer about the scope when no engine answered or
// declined.
func ask[T any](
	ctx context.Context,
	s *Service,
	req engine.Request,
	role engine.Role,
	call func(engine.Engine) (engine.Result[T], error),
) (engine.Answer[T], error) {
	answered, declined, err := engine.AskEach(ctx, s.catalog, s.router, req, role, call)
	switch {
	case err != nil:
		return engine.Answer[T]{}, err
	case !spoke(answered) && len(declined) > 0:
		return engine.Unsupported[T](declined.Reason()), nil
	case len(answered) == 0:
		return engine.Unsupported[T](fmt.Sprintf("no engine serves %q for this role", req.Scope)), nil
	}
	return merge(answered, req.Preferred, declined), nil
}
