// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"os/exec"
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
