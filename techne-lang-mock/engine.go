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

// Engine serves every role of a mock language over the files of a workspace, at the tier, the
// completeness and the cost that its options set. Each call reads the files anew, so an
// Engine is safe for concurrent use.
type Engine struct {
	fsys     fs.FS
	declared lang.Declaration
	fidelity trust.Fidelity
	coverage trust.Completeness
	cost     engine.Cost
	missing  string
}

// New returns an engine over the workspace in fsys for the language that d declares, with
// opts applied. The engine claims [trust.Resolved], [trust.ScopeTotal] and
// [engine.CostAnalyze] unless an option sets another value. New returns an error for a nil
// fsys and for a declaration without a language.
func New(fsys fs.FS, d lang.Declaration, opts ...Option) (*Engine, error) {
	if fsys == nil {
		return nil, errors.New("mock: the engine has no filesystem to read")
	}
	if d.Language == "" {
		return nil, errors.New("mock: the declaration names no language")
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

// Name returns mock/ and the name of the language, so two mock languages of one catalogue
// have two engine names.
func (e *Engine) Name() string { return "mock/" + string(e.declared.Language) }

// Language returns the language of the declaration that the engine was built with.
func (e *Engine) Language() source.Language { return e.declared.Language }

// Fidelity returns the tier that [At] set for every role.
func (e *Engine) Fidelity(engine.Role) trust.Fidelity { return e.fidelity }

// Cost returns the cost that [Costing] set for every role.
func (e *Engine) Cost(engine.Role) engine.Cost { return e.cost }

// Available returns an error with the reason that [Missing] set, and nil for an engine without
// that option.
func (e *Engine) Available(context.Context) error {
	if e.missing == "" {
		return nil
	}
	return fmt.Errorf("mock: %s", e.missing)
}

// Engine implements the engine port and reports whether it is available.
var _ interface {
	engine.Engine
	engine.Available
} = (*Engine)(nil)
