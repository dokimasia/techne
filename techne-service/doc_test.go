// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package service_test

import (
	"context"
	"strings"
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

// TestDoc covers the claim the package comment makes: the read path and
// the write path drive the same ports and agree about who serves what.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("the read path and the write path", func(t *testing.T) {
		t.Parallel()

		t.Run("route one scope to one language", func(t *testing.T) {
			t.Parallel()
			// They held a copy of this rule each once, and a read and a
			// write that disagree about who owns a file plan a change
			// with one engine and gate it with another. Both now take it
			// from core, and this is what says they still do.
			catalogue := engine.NewCatalog()
			assert.NoError(t, catalogue.Add(both{language: "fixture"}), "an engine registers")
			assert.NoError(t, catalogue.Add(both{language: "other"}), "and so does a second")

			read := query.New(catalogue, claiming{})
			write := change.New(catalogue, claiming{}, workspace{})

			outlined, err := read.Outline(t.Context(), engine.Request{Scope: "a.fx"})
			assert.NoError(t, err, "outlining a served file succeeds")

			applied, err := write.Apply(t.Context(), edit.Request{
				Operation: edit.DocumentSymbol,
				Scope:     "a.fx",
				Target:    edit.Target{Kind: edit.TargetSpan, Span: source.Span{Path: "a.fx"}},
				Args:      edit.Args{edit.ArgDoc: "Doc."},
				DryRun:    true,
			})
			assert.NoError(t, err, "planning a change to the same file succeeds")

			assert.Equal(t, applied.Provenance.Engine, outlined.Provenance.Engine,
				"one file, one language, one engine, whichever path asked")
		})
	})
}

// claiming routes .fx to one language and knows two.
type claiming struct{}

func (claiming) LanguageOf(p source.Path) (source.Language, bool) {
	return "fixture", strings.HasSuffix(string(p), ".fx")
}

func (claiming) Languages() []source.Language {
	return []source.Language{"fixture", "other"}
}

// both serves the read role and the write role, so one engine answers
// whichever path asks.
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
	return engine.Result[edit.Change]{
		Items: []edit.Change{{
			Kind: edit.ChangeEdit, Path: req.Scope,
			Edits: []edit.TextEdit{{New: "// Doc.\n"}},
		}},
		Completeness: trust.ScopeTotal,
	}, nil
}

// workspace is a file that exists and nothing else.
type workspace struct{}

func (workspace) Read(source.Path) ([]byte, error) { return []byte("one\n"), nil }
func (workspace) Write(source.Path, []byte) error  { return nil }
func (workspace) Remove(source.Path) error         { return nil }
