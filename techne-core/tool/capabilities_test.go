// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/tool"
	"go.dokimi.dev/techne/core/trust"
)

// server stands for an engine whose language server is not installed.
type server struct{ parser }

func (server) Name() string                        { return "server" }
func (server) Fidelity(engine.Role) trust.Fidelity { return trust.Resolved }
func (server) Cost(engine.Role) engine.Cost        { return engine.CostSession }

func (server) Available(context.Context) error {
	return errors.New("engine: gopls not on PATH")
}

// elsewhere serves another language entirely.
type elsewhere struct{ parser }

func (elsewhere) Name() string              { return "elsewhere" }
func (elsewhere) Language() source.Language { return source.Language("elsewhere") }

func capabilitiesTool(t *testing.T, engines ...engine.Engine) tool.Tool {
	t.Helper()
	c := engine.NewCatalog()
	for _, e := range engines {
		assert.NoError(t, c.Add(e), "the case needs this engine registered")
	}
	built, err := tool.Capabilities(c)
	assert.NoError(t, err, "the capabilities tool builds from a catalogue")
	return built
}

func reported(t *testing.T, built tool.Tool, input string) []map[string]any {
	t.Helper()
	got, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "a well-formed call reaches the catalogue")
	var decoded struct {
		Items []map[string]any `json:"items"`
	}
	assert.NoError(t, json.Unmarshal(got.Payload, &decoded), "the result is JSON a caller can read")
	return decoded.Items
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("Capabilities", func(t *testing.T) {
		t.Parallel()

		t.Run("says why an agent should call it", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, capabilitiesTool(t).Description(), "PREFER OVER ",
				"an agent that infers capability from the tool list gets it wrong")
		})

		t.Run("reports what an engine serves, in words", func(t *testing.T) {
			t.Parallel()
			items := reported(t, capabilitiesTool(t, parser{}), `{}`)
			assert.Length(t, items, 1, "one engine serving one role is one capability")
			assert.Equal(t, items[0]["language"], "fixture", "a caller reads which language")
			assert.Equal(t, items[0]["role"], "outline", "a caller reads the role as a word")
			assert.Equal(t, items[0]["fidelity"], "syntactic", "a caller reads the tier as a word")
			assert.Equal(t, items[0]["cost"], "parse", "a caller reads the price as a word")
			assert.Equal(t, items[0]["available"], true, "an in-process engine can always run")
		})

		t.Run("reports an engine that cannot run, with the reason", func(t *testing.T) {
			t.Parallel()
			// A missing server is a different problem from a missing
			// capability, and omitting it would make them look alike.
			items := reported(t, capabilitiesTool(t, server{}), `{}`)
			assert.Length(t, items, 1, "an engine that cannot run is still reported")
			assert.Equal(t, items[0]["available"], false, "the server is not installed")
			assert.Contains(t, items[0]["unavailable"], "PATH", "the reason says what is missing")
		})

		t.Run("narrows to one language when asked", func(t *testing.T) {
			t.Parallel()
			items := reported(t, capabilitiesTool(t, parser{}, elsewhere{}), `{"language":"fixture"}`)
			assert.Length(t, items, 1, "a caller working in one language need not read about others")
			assert.Equal(t, items[0]["language"], "fixture", "the filter keeps the language asked for")
		})

		t.Run("reports every language when none is named", func(t *testing.T) {
			t.Parallel()
			items := reported(t, capabilitiesTool(t, parser{}, elsewhere{}), `{}`)
			assert.Length(t, items, 2, "a caller that names no language is told everything served")
		})

		t.Run("answers an empty catalogue with an empty list", func(t *testing.T) {
			t.Parallel()
			// Serving nothing is a fact a caller can act on, not a
			// fault.
			items := reported(t, capabilitiesTool(t), `{}`)
			assert.Empty(t, items, "a server holding no engine says so rather than failing")
		})

		t.Run("answers in a stable order", func(t *testing.T) {
			t.Parallel()
			built := capabilitiesTool(t, parser{}, elsewhere{})
			first := reported(t, built, `{}`)
			second := reported(t, built, `{}`)
			assert.Equal(t, first, second, "a caller caching the report sees only real changes")
		})
	})
}
