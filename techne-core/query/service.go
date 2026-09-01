// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package query

import (
	"context"
	"errors"
	"fmt"
	"path"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Router reports which languages exist and which one claims a path.
//
// It is a port because the registry that knows lives in the language
// module's world, and core names no language. A registry satisfies it.
type Router interface {
	// LanguageOf reports which language claims a path, by its
	// extension. A directory carries none.
	LanguageOf(p source.Path) (source.Language, bool)

	// Languages are every registered language, in a fixed order. A
	// scope no single language claims is asked of all of them, and the
	// order is what keeps a merged answer from wandering.
	Languages() []source.Language
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
	var answered []engine.Answer[T]
	for _, language := range s.languages(req) {
		for _, e := range s.catalog.For(ctx, language, role) {
			result, err := call(e)
			switch {
			case errors.Is(err, engine.ErrDecline):
				continue
			case err != nil:
				return engine.Answer[T]{}, err
			}
			answered = append(answered, engine.Publish(result, e, role, req.Preferred))
			break
		}
	}

	if len(answered) == 0 {
		return unsupported[T](fmt.Sprintf(
			"no engine serves %q for this role", req.Scope)), nil
	}
	return merge(answered, req.Preferred), nil
}

// languages resolves which languages a request is about.
//
// A caller that named one is believed: it may know about a path the
// router does not. A path carrying an extension is a file, and belongs
// to whichever language claims that extension, or to none. A path
// carrying no extension is a directory, which holds whatever it holds:
// a package in Go, mixed sources anywhere else. Every language is asked
// and the answers merge.
//
// A directory named with a dot is read as a file and answers for no
// language. Nothing here can stat the scope, and the alternative is
// asking every language about every unclaimed file.
func (s *Service) languages(req engine.Request) []source.Language {
	if req.Language != "" {
		return []source.Language{req.Language}
	}
	if claimed, ok := s.router.LanguageOf(req.Scope); ok {
		return []source.Language{claimed}
	}
	// path.Ext(".") is ".", so the workspace root would otherwise read
	// as a file carrying an extension no language claims. It is the
	// scope a request that names none is given, which would leave every
	// such request answered by nothing.
	if req.Scope != root && path.Ext(string(req.Scope)) != "" {
		return nil
	}
	return s.router.Languages()
}

// root is the whole workspace, and what a request naming no scope asks
// about.
const root source.Path = "."

// unsupported is the answer when nothing can be asked.
//
// It carries no payload, so an empty item list is never read as evidence
// of absence, and it says why: a caller that is only told no cannot tell
// a capability gap from a mistake it could correct.
func unsupported[T any](reason string) engine.Answer[T] {
	return engine.Answer[T]{
		Status: trust.Unsupported,
		Provenance: trust.Provenance{
			Caveats: []trust.Caveat{{Code: trust.CaveatUnsupported, Note: reason}},
		},
	}
}
