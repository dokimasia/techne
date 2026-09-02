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

// Router reports which languages exist and which one claims a path.
//
// It is a port because the registry that knows lives in a language
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

// Languages resolves which languages a request could be about.
//
// A caller that named one is believed: it may know about a path the
// router does not. A path carrying an extension is a file, and belongs
// to whichever language claims that extension, or to none. A path
// carrying no extension is a directory, which holds whatever it holds: a
// package in Go, mixed sources anywhere else. Every language is asked
// and the answers merge.
//
// A directory named with a dot is read as a file and answers for no
// language. Nothing here can stat the scope, and the alternative is
// asking every language about every unclaimed file.
//
// The rule lives here rather than in each service because a read and a
// write that disagreed about which language owns a path would plan a
// change with one engine and gate it with another.
func Languages(r Router, req Request) []source.Language {
	if req.Language != "" {
		return []source.Language{req.Language}
	}
	if claimed, ok := r.LanguageOf(req.Scope); ok {
		return []source.Language{claimed}
	}
	// path.Ext(".") is ".", so the workspace root would otherwise read
	// as a file carrying an extension no language claims. It is the
	// scope a request that names none is given, which would leave every
	// such request answered by nothing.
	if req.Scope != Root && path.Ext(string(req.Scope)) != "" {
		return nil
	}
	return r.Languages()
}

// Root is the whole workspace, and what a request naming no scope asks
// about.
const Root source.Path = "."

// Declined is why the engines that could have answered did not.
//
// A decline is a fact a caller can act on and often the only actionable
// thing in the exchange: a server saying it cannot find the TypeScript
// installation names what to install, where "no engine serves this file"
// names nothing and reads as a language techne cannot refactor. So the
// reasons come back rather than being dropped on the way past.
//
// Empty when an engine answered, and empty when none was asked.
type Declined []string

// Reason joins the declines into one line, and is empty when there were
// none.
func (d Declined) Reason() string { return strings.Join(d, "; ") }

// Ask puts one question to the engines serving a language, strongest
// first, and returns the first answer.
//
// An engine returning [ErrDecline] serves the role and cannot answer
// this request, so the next engine gets a turn. Any other error stops
// the search and reaches the caller: answering from a weaker engine when
// the stronger one is broken hides the breakage for as long as anyone
// believes the answer.
//
// The caller supplies the one line that differs between roles, which is
// which port to call. Everything else about selecting an engine and
// stamping what it returned happens here and nowhere else, so what
// counts as degraded cannot drift between one role and another.
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

// attributed names which engine declined, unless the reason already
// does. An engine's own error carries its name by convention, and one
// opening with the name twice reads as two engines refusing.
func attributed(name, why string) string {
	if strings.Contains(why, name) {
		return why
	}
	return name + ": " + why
}

// reason is a decline without the sentinel that marks it as one.
//
// The sentinel is how a service knows to move on; the words after it are
// what a caller reads. Left in, every reason a caller sees would open
// with the same four words.
func reason(err error) string {
	held := err.Error()
	if _, after, cut := strings.Cut(held, ErrDecline.Error()+": "); cut {
		return after
	}
	return held
}

// AskEach puts one question to every language a request could be about,
// and returns what answered.
//
// A scope no single language claims is asked of all of them, because a
// directory holding several languages is the normal case. A caller that
// wants one answer merges them.
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
	for _, language := range Languages(r, req) {
		answered, ok, why, err := Ask(ctx, c, language, role, req.Preferred, call)
		if err != nil {
			return nil, nil, err
		}
		declined = append(declined, why...)
		if ok {
			out = append(out, answered)
		}
	}
	return out, declined, nil
}

// AskAny puts one question to the languages a request could be about and
// stops at the first that answers.
//
// It is what a question with one right answer wants: a symbol is
// declared in one language, so a second answer would be about a second
// symbol, and asking for it is work whose result is thrown away.
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
		if err != nil || ok {
			return answered, ok, declined, err
		}
		declined = append(declined, why...)
	}
	return Answer[T]{}, false, declined, nil
}

// Unsupported is the answer when nothing could be asked.
//
// It carries no payload, so an empty item list is never read as evidence
// of absence, and it says why: a caller that is only told no cannot tell
// a capability gap it should route around from a mistake it could
// correct.
func Unsupported[T any](reason string) Answer[T] {
	return Answer[T]{
		Status: trust.Unsupported,
		Provenance: trust.Provenance{
			Caveats: []trust.Caveat{{Code: trust.CaveatUnsupported, Note: reason}},
		},
	}
}
