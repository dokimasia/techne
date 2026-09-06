// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Engine answers about Go by type-checking the workspace in this
// process.
//
// # Why it exists beside a language server
//
// gopls answers the same questions and answers them faster once it is
// warm. It is also a program that has to be installed, and a machine
// without it drops Go from every question that needs a type: what
// implements this, what calls this, does this still compile. This engine
// needs nothing but the Go toolchain the workspace is already built
// with, so the answer is there whether or not anything was installed.
//
// The catalogue prefers the server where both can answer. Both claim the
// same tier, so the order between them is the order they were
// registered, and the language module registers the server first.
//
// # What it does not answer
//
// Outlining and searching are a parser's, which does them at a
// thousandth of the cost. Planning a change is not here either: the
// operations techne serves are the ones a server computes, and a rename
// worked out from a type graph by hand would be a second implementation
// of the one thing the write path must not get wrong.
type Engine struct {
	declared lang.Declaration
	root     string

	// loading guards the one cached view. Two questions arriving
	// together would otherwise type-check the workspace twice.
	loading sync.Mutex
	view    *view
}

// New returns an engine over a workspace on disk.
//
// The root is a path rather than an [io/fs.FS] because the loader runs
// the Go toolchain against a directory, and a tree that is nowhere has
// no packages to load.
func New(root string, d lang.Declaration) (*Engine, error) {
	if d.Language == "" {
		return nil, fmt.Errorf("checker: declaration names no language")
	}
	if root == "" {
		return nil, fmt.Errorf("checker: %q has no workspace root", d.Language)
	}

	held, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("checker: %q workspace root: %w", d.Language, err)
	}
	if info, err := os.Stat(held); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("checker: %q workspace root %q is not a directory", d.Language, held)
	}
	return &Engine{declared: d, root: held}, nil
}

// Name identifies this engine in a provenance and a capability report.
//
// The toolchain rather than the package, because that is what a caller
// can act on: told the Go type checker answered, it knows the answer is
// as good as the build is and that no server was involved.
func (*Engine) Name() string { return "go/types" }

// Language is the one language this engine answers about.
func (e *Engine) Language() source.Language { return e.declared.Language }

// Fidelity is [trust.Resolved] for the roles it serves.
//
// It binds names through the same type checker the compiler runs, which
// is the strongest evidence there is about a Go program.
func (*Engine) Fidelity(role engine.Role) trust.Fidelity { return Binding()[role] }

// Cost is [engine.CostSession]: dear once and cheap after.
//
// Type-checking a module is seconds and the result is kept, so a caller
// asking who calls a function and then who calls its caller pays once.
// Priced at what the first call costs, a catalogue would route around
// the only engine that can answer at all on a machine with no server.
func (*Engine) Cost(engine.Role) engine.Cost { return engine.CostSession }

// Available reports whether the Go toolchain can be run.
//
// The loader shells out to go list, so a workspace whose toolchain is
// missing is a question this cannot answer — and saying so before a call
// is what lets a caller install something rather than read a failure.
func (*Engine) Available(context.Context) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("checker: go is not on PATH")
	}
	return nil
}

// Close drops what was type-checked.
//
// A composition root calls it. This holds no process and no handle, so
// closing it frees memory and nothing else.
func (e *Engine) Close(context.Context) error {
	e.forget()
	return nil
}

// Binding is what this engine reaches, per role.
//
// Declared here rather than guessed at each call, and per role because
// they are not the same: it binds names as well as anything can and
// outlines a file no better than a parser does at a thousandth of the
// cost, so it claims nothing for the roles a parser owns.
func Binding() map[engine.Role]trust.Fidelity {
	return map[engine.Role]trust.Fidelity{
		engine.RoleResolve: trust.Resolved,
		engine.RoleRelate:  trust.Resolved,
		engine.RoleVerify:  trust.Resolved,
		engine.RoleCheck:   trust.Resolved,
	}
}

// assert the engine claims what its package comment says it does, and
// serves every role it has an answer behind.
var (
	_ engine.Engine    = (*Engine)(nil)
	_ engine.Available = (*Engine)(nil)
	_ engine.Resolver  = (*Engine)(nil)
	_ engine.Relator   = (*Engine)(nil)
	_ engine.Verifier  = (*Engine)(nil)
	_ engine.Checker   = (*Engine)(nil)
)
