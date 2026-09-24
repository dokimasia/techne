// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"context"
	"errors"
	"path"
	"strings"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Router maps paths to languages. The language registry implements it.
type Router interface {
	// LanguageOf returns the language that claims p by its extension.
	LanguageOf(p source.Path) (source.Language, bool)

	// Languages returns every registered language, in a fixed order.
	Languages() []source.Language
}

// Root is the scope of the whole workspace.
const Root source.Path = "."

// Languages returns the languages a request is about. It returns the
// request's Language if one is set. Otherwise it returns the language that
// claims the scope's extension. A scope with an extension that no language
// claims is about no language, and a scope without an extension is treated
// as a directory and is about every language.
//
// The read path and the write path both call Languages, so both route a
// path to the same languages. A directory whose name contains a dot is
// treated as a file.
func Languages(r Router, req Request) []source.Language {
	if req.Language != "" {
		return []source.Language{req.Language}
	}
	if claimed, ok := r.LanguageOf(req.Scope); ok {
		return []source.Language{claimed}
	}
	// path.Ext(".") is ".", so the root is excluded explicitly.
	if req.Scope != Root && path.Ext(string(req.Scope)) != "" {
		return nil
	}
	return r.Languages()
}

// Declined lists why engines could not answer a request, one reason per
// engine that declined or language that failed.
type Declined []string

// Reason joins the reasons into one line. It returns an empty string for no
// reasons.
func (d Declined) Reason() string { return strings.Join(d, "; ") }

// Ask asks the engines that serve role for language, in catalogue order, and
// returns the first answer and true. An engine that returns ErrDecline is
// skipped and its reason recorded in Declined. Any other error stops the
// search and is returned, because an answer from a weaker engine would hide
// a broken stronger one.
//
// call invokes the port of role on one engine. If no engine returns an
// answer, Ask returns false and the reasons of the engines that declined.
func Ask[T any](
	ctx context.Context,
	c *Catalog,
	language source.Language,
	role Role,
	want trust.Fidelity,
	call func(Engine) (Result[T], error),
) (Answer[T], bool, Declined, error) {
	var declined Declined
	for _, e := range c.For(ctx, language, role) {
		result, err := call(e)
		switch {
		case errors.Is(err, ErrDecline):
			declined = append(declined, attributed(e.Name(), reason(err)))
			continue
		case err != nil:
			return Answer[T]{}, false, declined, err
		}
		return Publish(result, e, role, want), true, nil, nil
	}
	return Answer[T]{}, false, declined, nil
}

// AskEach asks every language the request is about and returns every
// answer. A language that fails is recorded in Declined with its error, and
// the other languages are still asked. AskEach returns the first failure
// only when no language answered or ctx is done.
//
// The read path calls AskEach and merges the answers. A merge marks the
// coverage of an answer with a Declined entry as partial.
func AskEach[T any](
	ctx context.Context,
	c *Catalog,
	r Router,
	req Request,
	role Role,
	call func(Engine) (Result[T], error),
) ([]Answer[T], Declined, error) {
	var out []Answer[T]
	var declined Declined
	var failed error
	for _, language := range Languages(r, req) {
		answered, ok, why, err := Ask(ctx, c, language, role, req.Preferred, call)
		declined = append(declined, why...)
		switch {
		case err != nil && ctx.Err() != nil:
			return nil, nil, err
		case err != nil:
			if failed == nil {
				failed = err
			}
			declined = append(declined, string(language)+": "+err.Error())
		case ok:
			out = append(out, answered)
		}
	}
	if len(out) == 0 && failed != nil {
		return nil, nil, failed
	}
	return out, declined, nil
}

// AskAny asks the languages the request is about, in order, and returns the
// first answer that is not skipped, and true. A skipped answer states that
// the scope contains no file of its language, so AskAny asks the next
// language. Any error stops the search and is returned. The answer of the
// first language defines the result, so asking the next language after a
// failure can return an answer about a different declaration.
//
// The write path calls AskAny to plan a change and to check it.
func AskAny[T any](
	ctx context.Context,
	c *Catalog,
	r Router,
	req Request,
	role Role,
	call func(Engine) (Result[T], error),
) (Answer[T], bool, Declined, error) {
	var declined Declined
	for _, language := range Languages(r, req) {
		answered, ok, why, err := Ask(ctx, c, language, role, req.Preferred, call)
		switch {
		case err != nil:
			return Answer[T]{}, false, declined, err
		case ok && !answered.Skipped:
			return answered, true, declined, nil
		}
		declined = append(declined, why...)
	}
	return Answer[T]{}, false, declined, nil
}

// Unsupported returns the answer for a request that nothing could serve: no
// items, status Unsupported, and a caveat with the reason.
func Unsupported[T any](reason string) Answer[T] {
	return Answer[T]{
		Status: trust.Unsupported,
		Provenance: trust.Provenance{
			Caveats: []trust.Caveat{{Code: trust.CaveatUnsupported, Note: reason}},
		},
	}
}

// attributed prefixes a decline reason with the engine name unless the
// reason already starts with it.
func attributed(name, why string) string {
	if strings.HasPrefix(why, name) {
		return why
	}
	return name + ": " + why
}

// reason returns the text of a decline without the ErrDecline prefix.
func reason(err error) string {
	text := err.Error()
	if _, after, ok := strings.Cut(text, ErrDecline.Error()+": "); ok {
		return after
	}
	return text
}
