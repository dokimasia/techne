// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package service_test

import (
	"context"
	"path"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/service/change"
	"go.dokimi.dev/techne/service/query"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("plans with the engine that outlines the file", func(t *testing.T) {
			t.Parallel()
			catalogue := engine.NewCatalog()
			assert.NoError(t, catalogue.Add(both{language: "fixture"}), "Add of fixture")
			assert.NoError(t, catalogue.Add(both{language: "other"}), "Add of other")

			outlined, err := query.New(catalogue, suffix{}).Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.NoError(t, err, "Outline of a.fx")
			applied, err := change.New(catalogue, suffix{}, workspace{}).Apply(t.Context(), edit.Request{
				Operation: edit.DocumentSymbol,
				Scope:     "a.fx",
				Target:    edit.Target{Kind: edit.TargetSpan, Span: source.Span{Path: "a.fx"}},
				Args:      edit.Args{edit.ArgDoc: "Doc."},
				DryRun:    true,
			})
			assert.NoError(t, err, "Apply to a.fx")
			assert.Equal(t, applied.Provenance.Engine, outlined.Provenance.Engine, "the engine of the plan")
		})
	})
}

// suffix routes .fx to fixture, and asks fixture and other about a directory.
type suffix struct{}

func (suffix) LanguageOf(p source.Path) (source.Language, bool) {
	return "fixture", path.Ext(string(p)) == ".fx"
}

func (suffix) Languages() []source.Language { return []source.Language{"fixture", "other"} }

// both is an engine of one language that outlines and plans, so either path can ask it.
type both struct{ language source.Language }

func (b both) Name() string                      { return "engine/" + string(b.language) }
func (b both) Language() source.Language         { return b.language }
func (both) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }
func (both) Cost(engine.Role) engine.Cost        { return engine.CostParse }

func (both) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Completeness: trust.ScopeTotal}, nil
}

func (both) Plan(
	_ context.Context,
	req engine.Request,
	_ edit.Operation,
	_ edit.Target,
	_ edit.Args,
) (engine.Result[edit.Change], error) {
	written := []edit.TextEdit{{New: "// Doc.\n"}}
	return engine.Result[edit.Change]{
		Items:        []edit.Change{{Kind: edit.ChangeEdit, Path: req.Scope, Edits: written}},
		Completeness: trust.ScopeTotal,
	}, nil
}

// workspace is a directory that contains one file with every path, and takes every change.
type workspace struct{}

func (workspace) Read(source.Path) ([]byte, error)     { return []byte("one\n"), nil }
func (workspace) Write(source.Path, []byte) error      { return nil }
func (workspace) Remove(source.Path) error             { return nil }
func (workspace) Move(source.Path, source.Path) error  { return nil }
func (workspace) Lock(context.Context) (func(), error) { return func() {}, nil }
