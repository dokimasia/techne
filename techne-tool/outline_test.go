// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/tool"
)

const fixture = source.Language("fixture")

// parser is an engine of the language fixture that serves outline at the syntactic tier, so
// an answer has the name and the tier of an engine.
type parser struct{ found []sema.Symbol }

func (parser) Name() string                        { return "parser" }
func (parser) Language() source.Language           { return fixture }
func (parser) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }
func (parser) Cost(engine.Role) engine.Cost        { return engine.CostParse }

// Outline returns the declarations of p, which makes parser an engine of the role outline for
// the capabilities tool.
func (p parser) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Items: p.found, Completeness: trust.ScopeTotal}, nil
}

// reads is the read service of the tests. It publishes the results of its parser with
// [engine.Publish], as a service does, for the scopes that it claims, and returns
// [engine.Unsupported] for any other scope. It records each request of Relate and each query
// of Search.
type reads struct {
	engine parser
	claims map[source.Path]bool
	// edges are the relations of Relate, and found the findings of Verify.
	edges []sema.Relation
	found []edit.Finding

	related  []engine.Request
	relating []sema.ID
	searched []engine.Query
}

// unsupported returns the answer of a scope that r does not claim.
func unsupported[T any](scope source.Path) engine.Answer[T] {
	return engine.Unsupported[T]("no engine serves " + string(scope) + " for this role")
}

func (r *reads) Outline(_ context.Context, req engine.Request) (engine.Answer[sema.Symbol], error) {
	if !r.claims[req.Scope] {
		return unsupported[sema.Symbol](req.Scope), nil
	}
	return engine.Publish(
		engine.Result[sema.Symbol]{Items: r.engine.found, Completeness: trust.ScopeTotal},
		r.engine, engine.RoleOutline, req.Preferred), nil
}

func (r *reads) Search(
	_ context.Context,
	req engine.Request,
	q engine.Query,
) (engine.Answer[sema.Symbol], error) {
	r.searched = append(r.searched, q)
	if !r.claims[req.Scope] {
		return unsupported[sema.Symbol](req.Scope), nil
	}
	return engine.Publish(
		engine.Result[sema.Symbol]{Items: r.engine.found, Completeness: trust.ScopeTotal},
		r.engine, engine.RoleSearch, req.Preferred), nil
}

func (r *reads) Resolve(
	_ context.Context,
	req engine.Request,
	_ source.Position,
) (engine.Answer[sema.Symbol], error) {
	if !r.claims[req.Scope] {
		return unsupported[sema.Symbol](req.Scope), nil
	}
	return engine.Publish(
		engine.Result[sema.Symbol]{Items: r.engine.found, Completeness: trust.ScopeTotal},
		r.engine, engine.RoleResolve, req.Preferred), nil
}

func (r *reads) Relate(
	_ context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Answer[sema.Relation], error) {
	r.related, r.relating = append(r.related, req), append(r.relating, of)
	if !r.claims[req.Scope] {
		return unsupported[sema.Relation](req.Scope), nil
	}
	edges := make([]sema.Relation, 0, len(r.edges))
	for _, one := range r.edges {
		one.Kind = kind
		edges = append(edges, one)
	}
	return engine.Publish(
		engine.Result[sema.Relation]{Items: edges, Completeness: trust.ScopeTotal},
		r.engine, engine.RoleRelate, req.Preferred), nil
}

func (r *reads) Verify(
	_ context.Context,
	req engine.Request,
	_ []string,
) (engine.Answer[edit.Finding], error) {
	if !r.claims[req.Scope] {
		return unsupported[edit.Finding](req.Scope), nil
	}
	return engine.Publish(
		engine.Result[edit.Finding]{Items: r.found, Completeness: trust.ScopeTotal},
		r.engine, engine.RoleVerify, req.Preferred), nil
}

// serving returns a read service over found that claims the scope a.fx.
func serving(found ...sema.Symbol) *reads {
	return &reads{
		engine: parser{found: found},
		claims: map[source.Path]bool{"a.fx": true},
	}
}

// outlining returns the outline tool over a read service of found.
func outlining(t *testing.T, found ...sema.Symbol) tool.Tool {
	t.Helper()
	built, err := tool.Outline(serving(found...))
	assert.NoError(t, err, "the error of Outline")
	return built
}

// outlined runs the outline tool over found with input, and decodes the answer.
func outlined(t *testing.T, input string, found ...sema.Symbol) tool.Answer {
	t.Helper()
	result, err := outlining(t, found...).Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.Answer
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the answer")
	return out
}

// function returns an exported function of the language fixture named name in a.fx, with
// the documentation doc.
func function(name, doc string) sema.Symbol {
	return sema.Symbol{
		ID: sema.NewID(fixture, "a", name, sema.KindFunction), Name: name, Kind: sema.KindFunction,
		Language: fixture, Span: source.Span{Path: "a.fx"}, Visibility: sema.Exported, Doc: doc,
	}
}

func TestOutline(t *testing.T) {
	t.Parallel()

	documented := function("F", "F does a thing.")

	t.Run("Outline", func(t *testing.T) {
		t.Parallel()

		t.Run("starts its description with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, outlining(t).Description(), "PREFER OVER ", "the description")
		})

		t.Run("returns the declarations of a scope", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, `{"scope":"a.fx"}`, documented)
			assert.Equal(t, names(got.Items), []string{"F"}, "the declarations of a.fx")
		})

		t.Run("returns the evidence of the engine", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, `{"scope":"a.fx"}`, documented)
			assert.Equal(t, got.Provenance.Fidelity, "syntactic", "the fidelity")
			assert.Equal(t, got.Provenance.Completeness, "total", "the completeness")
			assert.Equal(t, got.Provenance.Engine, "parser", "the engine")
		})

		t.Run("supports no negative claim at the syntactic tier", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, `{"scope":"a.fx"}`)
			assert.False(t, got.Provenance.SupportsNegativeClaim, "the negative claim of an empty answer")
		})

		t.Run("encodes no error for a served request", func(t *testing.T) {
			t.Parallel()
			result, err := outlining(t, documented).Execute(t.Context(), json.RawMessage(`{"scope":"a.fx"}`))
			assert.NoError(t, err, "the error of Execute")
			var fields map[string]any
			assert.NoError(t, json.Unmarshal(result.Payload, &fields), "the decoding of the answer")
			assert.NotContains(t, fields, "error", "the fields of the answer")
		})

		t.Run("returns an unsupported failure for a scope that no engine serves", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, `{"scope":"notes.md"}`, documented)
			assert.True(t, got.Failed(), "the failure of the answer")
			assert.Equal(t, got.Error.Code, "unsupported", "the code of the failure")
			assert.NotEmpty(t, got.Error.Reason, "the reason of the failure")
		})

		t.Run("marks a result as failed for a scope that no engine serves", func(t *testing.T) {
			t.Parallel()
			got, err := outlining(t, documented).Execute(t.Context(), json.RawMessage(`{"scope":"notes.md"}`))
			assert.NoError(t, err, "the error of Execute")
			assert.True(t, got.Failed, "Failed of the result")
		})

		t.Run("marks no degraded answer as failed", func(t *testing.T) {
			t.Parallel()
			got, err := outlining(t, documented).Execute(t.Context(),
				json.RawMessage(`{"scope":"a.fx","preferred_fidelity":"resolved"}`))
			assert.NoError(t, err, "the error of Execute")
			assert.False(t, got.Failed, "Failed of the result")
		})

		t.Run("returns the documentation at docs", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, `{"scope":"a.fx","detail":"docs"}`, documented)
			assert.Equal(t, got.Items[0].Doc, "F does a thing.", "the documentation of F")
		})

		t.Run("drops the documentation before the declaration under a budget", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, `{"scope":"a.fx","detail":"source","max_tokens":1}`, documented)
			assert.Length(t, got.Items, 1, "the declarations of a.fx")
			assert.Empty(t, got.Items[0].Doc, "the documentation of F")
		})

		t.Run("narrows the answer to each kind word", func(t *testing.T) {
			t.Parallel()
			var found []sema.Symbol
			for _, kind := range sema.Kinds() {
				one := function(kind.String(), "")
				one.Kind = kind
				one.Span.Start.Offset = len(found) * 10
				one.Span.End.Offset = len(found)*10 + 5
				found = append(found, one)
			}
			for _, kind := range sema.Kinds() {
				got := outlined(t, `{"scope":"a.fx","include":["all"],"kind":"`+kind.String()+`"}`, found...)
				assert.Equal(t, names(got.Items), []string{kind.String()}, "the declarations of kind "+kind.String())
			}
		})

		t.Run("names the language of an answer of one language", func(t *testing.T) {
			t.Parallel()
			got := outlined(t, `{"scope":"a.fx"}`, documented)
			assert.Equal(t, got.Scope.Language, string(fixture), "the language of the scope")
		})

		t.Run("names no language for an answer of two languages", func(t *testing.T) {
			t.Parallel()
			other := function("G", "")
			other.Language, other.Span.Start.Offset, other.Span.End.Offset = "other", 10, 20
			got := outlined(t, `{"scope":"a.fx"}`, documented, other)
			assert.Empty(t, got.Scope.Language, "the language of the scope")
		})

		t.Run("accepts a backslash inside a path", func(t *testing.T) {
			t.Parallel()
			_, err := outlining(t).Execute(t.Context(), json.RawMessage(`{"scope":"dir\\a.fx"}`))
			assert.NoError(t, err, "the error of Execute for dir\\a.fx")
		})

		t.Run("refuses an absolute path of each platform", func(t *testing.T) {
			t.Parallel()
			for _, absolute := range []string{`/etc/passwd`, `C:\\Windows`, `c:/Windows`, `\\\\server\\share`, `//server/share`} {
				_, err := outlining(t).Execute(t.Context(), json.RawMessage(`{"scope":"`+absolute+`"}`))
				assert.HasError(t, err, "the error of Execute for "+absolute)
			}
		})

		t.Run("refuses a path that leaves the workspace", func(t *testing.T) {
			t.Parallel()
			_, err := outlining(t).Execute(t.Context(), json.RawMessage(`{"scope":"a/../../b.fx"}`))
			assert.HasError(t, err, "the error of Execute for a/../../b.fx")
		})
	})
}
