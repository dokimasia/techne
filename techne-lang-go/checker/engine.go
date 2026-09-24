// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"context"
	"errors"
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

// Engine type-checks a Go workspace in this process, with the loader of
// golang.org/x/tools/go/packages, and serves the roles that need types: resolve, relate, verify
// and check. It needs the go command, which builds the workspace, and no language server.
//
// An Engine is safe for concurrent use. It keeps one view of the workspace until the stamp of a
// walk of the workspace changes.
type Engine struct {
	declared lang.Declaration
	// root is the absolute path of the workspace. The go command runs in root and reports the
	// files under it in the form of root, also when root contains a symbolic link, because it
	// takes the working directory from PWD.
	root string

	// loading guards view, so two questions that arrive together load the workspace once.
	loading sync.Mutex
	view    *view
}

// New returns an engine over the workspace at root, a directory on disk, for the language that
// d declares. The go command runs in the directory, so the workspace must be on disk.
func New(root string, d lang.Declaration) (*Engine, error) {
	if d.Language == "" {
		return nil, errors.New("checker: the declaration names no language")
	}
	if root == "" {
		return nil, fmt.Errorf("checker: %s has no workspace root", d.Language)
	}
	held, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("checker: %s workspace root: %w", d.Language, err)
	}
	if info, err := os.Stat(held); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("checker: %s workspace root %s is not a directory", d.Language, held)
	}
	return &Engine{declared: d, root: held}, nil
}

// Name returns go/types, the name of the type checker of the Go toolchain.
func (*Engine) Name() string { return "go/types" }

// Language returns the language of the declaration that the engine was built with.
func (e *Engine) Language() source.Language { return e.declared.Language }

// Fidelity returns the tier of role from [Binding], and [trust.None] for a role that the
// engine does not serve.
func (*Engine) Fidelity(role engine.Role) trust.Fidelity { return Binding()[role] }

// Cost returns [engine.CostSession] for every role: the first question loads the workspace,
// and a later question reads the cached view.
func (*Engine) Cost(engine.Role) engine.Cost { return engine.CostSession }

// Available returns an error when the go command is not on PATH.
func (*Engine) Available(context.Context) error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("checker: go is not on PATH")
	}
	return nil
}

// Close drops the cached view, which is the only state that the engine keeps between
// questions.
func (e *Engine) Close(context.Context) error {
	e.forget()
	return nil
}

// Binding returns the tier of each role that the engine serves: [trust.Resolved] for resolve,
// relate, verify and check. It returns no tier for outline, search, plan, format and index,
// which the parser and the language server serve.
func Binding() map[engine.Role]trust.Fidelity {
	return map[engine.Role]trust.Fidelity{
		engine.RoleResolve: trust.Resolved,
		engine.RoleRelate:  trust.Resolved,
		engine.RoleVerify:  trust.Resolved,
		engine.RoleCheck:   trust.Resolved,
	}
}

// Engine implements the port of each role of [Binding], and reports whether the go command is
// available.
var (
	_ engine.Engine    = (*Engine)(nil)
	_ engine.Available = (*Engine)(nil)
	_ engine.Resolver  = (*Engine)(nil)
	_ engine.Relator   = (*Engine)(nil)
	_ engine.Verifier  = (*Engine)(nil)
	_ engine.Checker   = (*Engine)(nil)
)
