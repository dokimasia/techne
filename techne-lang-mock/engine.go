// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Engine answers every port, over files it really reads.
//
// It claims what it can do rather than what would be convenient. The
// default is resolved and total because within this language both are
// true: a use names a declaration, and the whole workspace is read to
// find it. [At] and [Covering] lower either, which is how a refusal is
// exercised rather than described.
type Engine struct {
	fsys     fs.FS
	declared lang.Declaration
	fidelity trust.Fidelity
	coverage trust.Completeness
	cost     engine.Cost
	missing  string
}

// New returns an engine over a workspace.
func New(fsys fs.FS, d lang.Declaration, opts ...Option) (*Engine, error) {
	if fsys == nil {
		return nil, fmt.Errorf("mock: no filesystem to read from")
	}
	if d.Language == "" {
		return nil, fmt.Errorf("mock: declaration names no language")
	}

	e := &Engine{
		fsys:     fsys,
		declared: d,
		fidelity: trust.Resolved,
		coverage: trust.ScopeTotal,
		cost:     engine.CostAnalyze,
	}
	for _, apply := range opts {
		apply(e)
	}
	return e, nil
}

// Name identifies this engine in a provenance and a capability report.
// It carries the language, because one adapter serves every mock
// language and five instances sharing one name would collide.
func (e *Engine) Name() string { return "mock/" + string(e.declared.Language) }

// Language is the one language this engine answers about.
func (e *Engine) Language() source.Language { return e.declared.Language }

// Fidelity is what this language was registered to claim, for every
// role. A language that binds names binds them whatever is asked.
func (e *Engine) Fidelity(engine.Role) trust.Fidelity { return e.fidelity }

// Cost is what this language was registered to cost.
func (e *Engine) Cost(engine.Role) engine.Cost { return e.cost }

// Available reports the reason this language cannot run, where one was
// given. A language declared missing is reported rather than hidden, so
// a caller learns the difference between a capability nothing has and
// one whose tool is not installed.
func (e *Engine) Available(context.Context) error {
	if e.missing == "" {
		return nil
	}
	return errors.New("mock: " + e.missing)
}

// assert the engine claims what its package comment says it does.
var _ interface {
	engine.Engine
	engine.Available
} = (*Engine)(nil)
