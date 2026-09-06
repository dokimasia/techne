// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"os/exec"
	"path"
	"strings"
	"time"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

// Server is what a language module declares about its language server.
//
// Every language declares one whether or not it is installed. A caller
// asking what techne can do is told that C# is served by a server that
// is not on this machine, which is a thing to act on; told nothing, it
// would conclude that C# cannot be served at all.
type Server struct {
	// Name is the server itself, as its own documentation calls it:
	// gopls, rust-analyzer, clangd. It reaches a caller in a capability
	// report and a provenance.
	Name string

	// Command is the argv to run, the first word of which is looked for
	// on the path before the server is declared available.
	Command []string

	// LanguageID is what this language is called in the protocol, which
	// is the protocol's own list rather than techne's: a language module
	// claiming "golang" would open every file under a name no server
	// recognises.
	LanguageID string

	// Dialects are what an extension is called instead, where one
	// language has more than one name in the protocol.
	//
	// TypeScript has two: a .tsx file is typescriptreact, and opened as
	// typescript a server parses the JSX in it as an error. It is the
	// same language and the same project to the server, which is why
	// this is a name per extension rather than a language of its own.
	Dialects map[string]string

	// Serves is the tier this server reaches, per role.
	//
	// Per role because they are not the same. A server binds names
	// through a type checker, so it resolves and relates at
	// [trust.Resolved]; asked to outline one file it does no better than
	// a parser does, at a thousandth of the cost. A server claiming
	// resolved for outline would win the catalogue's sort and make every
	// outline call start a process.
	Serves map[engine.Role]trust.Fidelity

	// Settings are handed to the server at initialise, under the key its
	// own documentation names.
	Settings map[string]any

	// Env is added to the environment the server runs in, on top of what
	// this process has. A server that needs a JDK or a toolchain pointed
	// at is told here rather than by whoever launches techne.
	Env map[string]string

	// Extracts is the code action this server offers for lifting a run
	// of lines into a function, and is empty for a server that offers
	// none.
	Extracts Refactor

	// Loading is how long a question waits for this server to finish
	// reading the workspace before it is answered anyway. Zero takes the
	// package default.
	//
	// Declared per server because they differ by orders of magnitude: a
	// parser-backed server is ready in milliseconds, and one that
	// imports a build system is not ready for minutes. A question that
	// outwaits this is still answered, and the answer says its coverage
	// is partial rather than claiming to have seen everything.
	Loading time.Duration
}

// Refactor names one of a server's code actions, so techne can ask for
// it and take the right one back.
//
// # Why a name and not a kind
//
// The kind does not identify a refactoring. rust-analyzer offers
// extracting a variable, a constant, a static and a function, all four
// under refactor.extract. typescript-language-server offers a method on
// the class beside an inner function that cannot see the receiver, both
// under refactor.extract.function, and the inner one first. csharp-ls
// sets no kind at all. No server marks any of them preferred.
//
// What tells them apart is the title, which is what an editor puts in
// its menu and what a person picks from. techne has no menu, so the
// language module declares which wording it means — a fact about that
// server, established by asking it, rather than a guess made here.
type Refactor struct {
	// Kind is the code action kind to ask for, and narrows what a server
	// computes. A server that sets no kind on its actions is matched on
	// title alone.
	Kind string

	// Titles are the wordings to prefer, best first, matched
	// case-insensitively as substrings. An action matching none of them
	// is taken only when nothing else is offered, because a server
	// wording an action differently in a context nobody probed is still
	// offering the refactoring that was asked for.
	Titles []string
}

// Offered reports whether the server was declared to offer this
// refactoring at all.
func (r Refactor) Offered() bool { return r.Kind != "" || len(r.Titles) > 0 }

// Named is what a file is called in the protocol, which is the
// language's own name unless a dialect claims the extension.
func (s Server) Named(p string) string {
	if held, dialect := s.Dialects[path.Ext(p)]; dialect {
		return held
	}
	return s.LanguageID
}

// Reaches is the tier this server claims for a role, and [trust.None]
// for a role it does not serve.
func (s Server) Reaches(role engine.Role) trust.Fidelity {
	if held, declared := s.Serves[role]; declared {
		return held
	}
	return trust.None
}

// Installed reports whether the server can be run, and says what is
// missing when it cannot.
//
// A missing server is a different problem from a missing capability. The
// first is fixed by installing something and the second is not, and a
// caller told only "no" cannot tell them apart.
func (s Server) Installed() error {
	if len(s.Command) == 0 {
		return &absent{name: s.Name, why: "declares no command to run"}
	}
	if _, err := exec.LookPath(s.Command[0]); err != nil {
		return &absent{
			name: s.Name,
			why:  s.Command[0] + " is not on PATH",
		}
	}
	return nil
}

// absent is why a declared server cannot run.
type absent struct{ name, why string }

func (a *absent) Error() string { return "lsp: " + a.name + ": " + a.why }

// Valid reports what a declaration is missing, or nil.
//
// Checked when a module registers rather than when a call arrives: a
// server declared without a language id opens every file under an empty
// name and is answered about nothing, which is the hardest failure to
// notice in a system whose job includes reporting that it found nothing.
func (s Server) Valid() error {
	switch {
	case strings.TrimSpace(s.Name) == "":
		return &absent{name: "a server", why: "has no name"}
	case len(s.Command) == 0:
		return &absent{name: s.Name, why: "declares no command to run"}
	case strings.TrimSpace(s.LanguageID) == "":
		return &absent{name: s.Name, why: "declares no language id for the protocol"}
	case len(s.Serves) == 0:
		return &absent{name: s.Name, why: "declares no role it serves"}
	}
	for role, held := range s.Serves {
		if held == trust.None {
			return &absent{
				name: s.Name,
				why:  "claims " + role.String() + " and no evidence for it",
			}
		}
	}
	return nil
}
