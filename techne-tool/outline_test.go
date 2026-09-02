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

// parser is the engine a read service would have selected, and is here
// so an answer carries a real name and a real tier rather than ones a
// case wrote down.
type parser struct{ found []sema.Symbol }

func (parser) Name() string                        { return "parser" }
func (parser) Language() source.Language           { return fixture }
func (parser) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }
func (parser) Cost(engine.Role) engine.Cost        { return engine.CostParse }

// Outline is what makes this serve a role, which is what the capability
// report is about. Nothing calls it: a tool is given a service, and the
// service that would have called this lives in another module.
func (p parser) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Items: p.found, Completeness: trust.ScopeTotal}, nil
}

// reads stands in for the read service, which lives in another module.
//
// What is under test here is what a tool does with an answer, not how
// one is assembled, so the answer is stamped by [engine.Publish] and
// withheld by [engine.Unsupported] exactly as a service would do it.
// Building the real service instead would make every case in this file
// depend on selection and merging as well.
type reads struct {
	engine parser
	claims map[source.Path]bool
	// edges and found are what the roles nothing implements yet answer
	// with, so a tool over them is testable before an engine exists.
	edges []sema.Relation
	found []edit.Finding
}

func (r reads) Outline(_ context.Context, req engine.Request) (engine.Answer[sema.Symbol], error) {
	if !r.claims[req.Scope] {
		return engine.Unsupported[sema.Symbol](
			"no engine serves " + string(req.Scope) + " for this role"), nil
	}
	return engine.Publish(
		engine.Result[sema.Symbol]{Items: r.engine.found, Completeness: trust.ScopeTotal},
		r.engine, engine.RoleOutline, req.Preferred), nil
}

func (r reads) Search(
	_ context.Context,
	req engine.Request,
	_ engine.Query,
) (engine.Answer[sema.Symbol], error) {
	if !r.claims[req.Scope] {
		return engine.Unsupported[sema.Symbol](
			"no engine serves " + string(req.Scope) + " for this role"), nil
	}
	return engine.Publish(
		engine.Result[sema.Symbol]{Items: r.engine.found, Completeness: trust.ScopeTotal},
		r.engine, engine.RoleSearch, req.Preferred), nil
}

func (r reads) Resolve(
	_ context.Context,
	req engine.Request,
	_ source.Position,
) (engine.Answer[sema.Symbol], error) {
	if !r.claims[req.Scope] {
		return engine.Unsupported[sema.Symbol](
			"no engine serves " + string(req.Scope) + " for this role"), nil
	}
	return engine.Publish(
		engine.Result[sema.Symbol]{Items: r.engine.found, Completeness: trust.ScopeTotal},
		r.engine, engine.RoleResolve, req.Preferred), nil
}

func (r reads) Relate(
	_ context.Context,
	req engine.Request,
	_ sema.ID,
	kind sema.RelationKind,
) (engine.Answer[sema.Relation], error) {
	if !r.claims[req.Scope] {
		return engine.Unsupported[sema.Relation](
			"no engine serves " + string(req.Scope) + " for this role"), nil
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

func (r reads) Verify(
	_ context.Context,
	req engine.Request,
	_ []string,
) (engine.Answer[edit.Finding], error) {
	if !r.claims[req.Scope] {
		return engine.Unsupported[edit.Finding](
			"no engine serves " + string(req.Scope) + " for this role"), nil
	}
	return engine.Publish(
		engine.Result[edit.Finding]{Items: r.found, Completeness: trust.ScopeTotal},
		r.engine, engine.RoleVerify, req.Preferred), nil
}

// serving is a read service over the declarations a case gave it.
func serving(found ...sema.Symbol) reads {
	return reads{
		engine: parser{found: found},
		claims: map[source.Path]bool{"a.fx": true},
	}
}

func outlineTool(t *testing.T, found ...sema.Symbol) tool.Tool {
	t.Helper()
	built, err := tool.Outline(serving(found...))
	assert.NoError(t, err, "the outline tool builds from a read service")
	return built
}

func call(t *testing.T, built tool.Tool, input string) map[string]any {
	t.Helper()
	got, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "a well-formed call reaches the service")
	var decoded map[string]any
	assert.NoError(t, json.Unmarshal(got.Payload, &decoded), "the result is JSON a caller can read")
	return decoded
}

func TestOutline(t *testing.T) {
	t.Parallel()

	declared := sema.Symbol{
		ID: "fixture:./a#F:function", Name: "F", Kind: sema.KindFunction,
		Language: fixture, Span: source.Span{Path: "a.fx"}, Visibility: sema.Exported,
		Doc: "F does a thing.",
	}

	t.Run("Outline", func(t *testing.T) {
		t.Parallel()

		t.Run("names the built-in it replaces", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, outlineTool(t).Description(), "PREFER OVER ",
				"an agent reaches for read unless told why not to")
		})

		t.Run("answers what a scope declares", func(t *testing.T) {
			t.Parallel()
			got := call(t, outlineTool(t, declared), `{"scope":"a.fx"}`)
			items, ok := got["items"].([]any)
			assert.True(t, ok, "an answer carries its items")
			assert.Length(t, items, 1, "the engine found one declaration")
		})

		t.Run("states the evidence behind the answer", func(t *testing.T) {
			t.Parallel()
			got := call(t, outlineTool(t, declared), `{"scope":"a.fx"}`)
			provenance, ok := got["provenance"].(map[string]any)
			assert.True(t, ok, "an answer states what stands behind it")
			assert.Equal(t, provenance["fidelity"], "syntactic", "a caller reads the tier as a word")
			assert.Equal(t, provenance["completeness"], "total", "a caller reads the coverage as a word")
			assert.Equal(t, provenance["engine"], "parser", "a caller can tell which engine answered")
		})

		t.Run("says whether an empty answer proves absence", func(t *testing.T) {
			t.Parallel()
			// An agent should not have to know that this means resolved
			// binding together with total coverage.
			got := call(t, outlineTool(t), `{"scope":"a.fx"}`)
			provenance := got["provenance"].(map[string]any)
			assert.Equal(t, provenance["supportsNegativeClaim"], false,
				"a parser's empty answer means none were found, never that there are none")
		})

		t.Run("carries no error when an engine answered", func(t *testing.T) {
			t.Parallel()
			// An answer that ran states its evidence in the provenance.
			// A status beside it would restate one of those fields.
			got := call(t, outlineTool(t, declared), `{"scope":"a.fx"}`)
			_, failed := got["error"]
			assert.False(t, failed, "an answer that ran says what it found and how, and no more")
		})

		t.Run("tells a caller the language is not served", func(t *testing.T) {
			t.Parallel()
			got := call(t, outlineTool(t, declared), `{"scope":"notes.md"}`)
			failure := got["error"].(map[string]any)
			assert.Equal(t, failure["code"], "unsupported",
				"a capability gap is something a caller routes around")
			assert.NotEmpty(t, failure["reason"],
				"a caller told only no cannot tell a gap from a mistake it could correct")
		})

		t.Run("marks an unsupported language as a failure without the transport reading it", func(t *testing.T) {
			t.Parallel()
			// A transport that had to parse the payload to learn this
			// would change every time the payload did.
			got, err := outlineTool(t, declared).Execute(t.Context(), json.RawMessage(`{"scope":"notes.md"}`))
			assert.NoError(t, err, "a capability gap is not a fault")
			assert.True(t, got.Failed, "a model can correct a request for a language nothing serves")
		})

		t.Run("does not mark a degraded answer as a failure", func(t *testing.T) {
			t.Parallel()
			// Weaker evidence than asked for is still worth reading.
			got, err := outlineTool(t, declared).Execute(t.Context(),
				json.RawMessage(`{"scope":"a.fx","preferred_fidelity":"resolved"}`))
			assert.NoError(t, err, "a weaker engine still answered")
			assert.False(t, got.Failed, "a degraded answer carries items a caller can use")
		})

		t.Run("drops documentation before it drops a declaration", func(t *testing.T) {
			t.Parallel()
			got := call(t, outlineTool(t, declared), `{"scope":"a.fx","detail":"source","max_tokens":1}`)
			items := got["items"].([]any)
			assert.Length(t, items, 1, "an answer that found something returns something")
			first := items[0].(map[string]any)
			_, documented := first["doc"]
			assert.False(t, documented, "documentation goes before any declaration does")
		})

		t.Run("carries documentation when asked and affordable", func(t *testing.T) {
			t.Parallel()
			got := call(t, outlineTool(t, declared), `{"scope":"a.fx","detail":"docs"}`)
			first := got["items"].([]any)[0].(map[string]any)
			assert.Equal(t, first["doc"], "F does a thing.", "docs is the level that carries documentation")
		})

		t.Run("refuses an absolute path", func(t *testing.T) {
			t.Parallel()
			// An absolute path leaks the machine's layout into an
			// agent's context and makes the answer useless elsewhere.
			_, err := outlineTool(t).Execute(t.Context(), json.RawMessage(`{"scope":"/etc/passwd"}`))
			assert.HasError(t, err, "a path outside the workspace is refused rather than answered")
		})
	})
}
