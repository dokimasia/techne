// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/query"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/tool"
	"go.dokimi.dev/techne/core/trust"
)

const fixture = source.Language("fixture")

// router answers which language claims a path.
type router map[string]source.Language

func (r router) LanguageOf(p source.Path) (source.Language, bool) {
	l, claimed := r[string(p)]
	return l, claimed
}

// Languages is what a directory scope is asked of.
func (r router) Languages() []source.Language {
	out := make([]source.Language, 0, len(r))
	for _, l := range r {
		out = append(out, l)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// parser is an engine that answers with what a case gave it.
type parser struct{ found []sema.Symbol }

func (parser) Name() string                        { return "parser" }
func (parser) Language() source.Language           { return fixture }
func (parser) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }
func (parser) Cost(engine.Role) engine.Cost        { return engine.CostParse }

func (p parser) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{Items: p.found, Completeness: trust.ScopeTotal}, nil
}

func outlineTool(t *testing.T, found ...sema.Symbol) tool.Tool {
	t.Helper()
	c := engine.NewCatalog()
	assert.NoError(t, c.Add(parser{found: found}), "the case needs an engine registered")
	built, err := tool.Outline(query.New(c, router{"a.fx": fixture}))
	assert.NoError(t, err, "the outline tool builds from a service")
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

		t.Run("reports a status a caller can branch on", func(t *testing.T) {
			t.Parallel()
			got := call(t, outlineTool(t, declared), `{"scope":"a.fx"}`)
			assert.Equal(t, got["status"], "ok", "a status reaches a caller as a word, not a number")
		})

		t.Run("tells a caller the language is not served", func(t *testing.T) {
			t.Parallel()
			got := call(t, outlineTool(t, declared), `{"scope":"notes.md"}`)
			assert.Equal(t, got["status"], "unsupported",
				"a capability gap is something a caller routes around")
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
			got := call(t, outlineTool(t, declared), `{"scope":"a.fx","detail":"full","max_tokens":1}`)
			items := got["items"].([]any)
			assert.Length(t, items, 1, "an answer that found something returns something")
			first := items[0].(map[string]any)
			_, documented := first["doc"]
			assert.False(t, documented, "documentation goes before any declaration does")
		})

		t.Run("carries documentation when asked and affordable", func(t *testing.T) {
			t.Parallel()
			got := call(t, outlineTool(t, declared), `{"scope":"a.fx","detail":"full"}`)
			first := got["items"].([]any)[0].(map[string]any)
			assert.Equal(t, first["doc"], "F does a thing.", "full is the level that carries documentation")
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
