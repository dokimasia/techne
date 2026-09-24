// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/tool"
)

// server is an engine of the language fixture at the resolved tier whose program is not on
// PATH.
type server struct{ parser }

func (server) Name() string                        { return "server" }
func (server) Fidelity(engine.Role) trust.Fidelity { return trust.Resolved }
func (server) Cost(engine.Role) engine.Cost        { return engine.CostSession }

func (server) Available(context.Context) error {
	return errors.New("engine: gopls not on PATH")
}

// elsewhere is an engine of the language elsewhere.
type elsewhere struct{ parser }

func (elsewhere) Name() string              { return "elsewhere" }
func (elsewhere) Language() source.Language { return source.Language("elsewhere") }

// catalogued returns the capabilities tool over a catalogue of engines.
func catalogued(t *testing.T, engines ...engine.Engine) tool.Tool {
	t.Helper()
	c := engine.NewCatalog()
	for _, e := range engines {
		assert.NoError(t, c.Add(e), "the error of Add for "+e.Name())
	}
	built, err := tool.Capabilities(c)
	assert.NoError(t, err, "the error of Capabilities")
	return built
}

// capable runs built with input and decodes the capabilities of the output.
func capable(t *testing.T, built tool.Tool, input string) []tool.Capability {
	t.Helper()
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.CapabilitiesOutput
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out.Items
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("Capabilities", func(t *testing.T) {
		t.Parallel()

		t.Run("starts its description with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, catalogued(t).Description(), "PREFER OVER ", "the description")
		})

		t.Run("returns each capability with its tiers as words", func(t *testing.T) {
			t.Parallel()
			got := capable(t, catalogued(t, parser{}), `{}`)
			assert.Equal(t, got, []tool.Capability{{
				Language: "fixture", Role: "outline", Engine: "parser",
				Fidelity: "syntactic", Cost: "parse", Available: true,
			}}, "the capabilities")
		})

		t.Run("returns an engine that cannot run with the reason", func(t *testing.T) {
			t.Parallel()
			got := capable(t, catalogued(t, server{}), `{}`)
			assert.Length(t, got, 1, "the capabilities")
			assert.False(t, got[0].Available, "Available of the server")
			assert.Contains(t, got[0].Unavailable, "PATH", "the reason of the server")
		})

		t.Run("returns the capabilities of the language of the input", func(t *testing.T) {
			t.Parallel()
			got := capable(t, catalogued(t, parser{}, elsewhere{}), `{"language":"fixture"}`)
			assert.Length(t, got, 1, "the capabilities of fixture")
			assert.Equal(t, got[0].Language, "fixture", "the language of the capability")
		})

		t.Run("returns every language without a language", func(t *testing.T) {
			t.Parallel()
			assert.Length(t, capable(t, catalogued(t, parser{}, elsewhere{}), `{}`), 2, "the capabilities")
		})

		t.Run("returns an empty list for an empty catalogue", func(t *testing.T) {
			t.Parallel()
			got := capable(t, catalogued(t), `{}`)
			assert.NotNil(t, got, "the list of capabilities")
			assert.Empty(t, got, "the capabilities")
		})

		t.Run("returns the capabilities in the same order twice", func(t *testing.T) {
			t.Parallel()
			built := catalogued(t, parser{}, elsewhere{})
			assert.Equal(t, capable(t, built, `{}`), capable(t, built, `{}`), "the capabilities of the second call")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("writes one line per engine with its roles", func(t *testing.T) {
			t.Parallel()
			served := func(role string) tool.Capability {
				return tool.Capability{
					Language: "go", Role: role, Engine: "gopls", Fidelity: "resolved", Cost: "session", Available: true,
				}
			}
			got := tool.CapabilitiesOutput{Items: []tool.Capability{served("outline"), served("search")}}.Render()
			assert.Equal(t, strings.Count(got, "gopls"), 1, "the lines of gopls")
			assert.Contains(t, got, "outline search", "the roles of gopls")
		})

		t.Run("writes the reason of an engine that cannot run", func(t *testing.T) {
			t.Parallel()
			got := tool.CapabilitiesOutput{Items: []tool.Capability{{
				Language: "csharp", Role: "outline", Engine: "lsp/csharp", Fidelity: "resolved", Cost: "session",
				Unavailable: "csharp-ls is not on PATH",
			}}}.Render()
			assert.Contains(t, got, "csharp-ls is not on PATH", "the render")
		})

		t.Run("writes nothing is served for no capability", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tool.CapabilitiesOutput{}.Render(), "nothing is served\n", "the render")
		})
	})
}
