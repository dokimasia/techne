// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/tool"
)

// catalogue is a catalogue without capabilities.
type catalogue struct{}

func (catalogue) Capabilities(context.Context) []engine.Capability { return nil }

// outlineOnly is a read service that serves outline and no other question.
type outlineOnly struct{ over *reads }

func (o outlineOnly) Outline(ctx context.Context, req engine.Request) (engine.Answer[sema.Symbol], error) {
	return o.over.Outline(ctx, req)
}

// TestService builds each tool from a type with only the methods that the tool calls.
func TestService(t *testing.T) {
	t.Parallel()

	t.Run("Outliner", func(t *testing.T) {
		t.Parallel()

		t.Run("builds the outline tool from a type that serves outline alone", func(t *testing.T) {
			t.Parallel()
			var reads tool.Outliner = outlineOnly{serving()}
			_, err := tool.Outline(reads)
			assert.NoError(t, err, "the error of Outline")
		})
	})

	t.Run("Catalogue", func(t *testing.T) {
		t.Parallel()

		t.Run("builds the capabilities tool from a type that lists capabilities alone", func(t *testing.T) {
			t.Parallel()
			var listed tool.Catalogue = catalogue{}
			_, err := tool.Capabilities(listed)
			assert.NoError(t, err, "the error of Capabilities")
		})
	})
}
