// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"context"
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// running stands for an engine that holds a process: it records being
// closed and can refuse to close.
type running struct {
	fake
	closed *int
	refuse error
}

func (h running) Close(context.Context) error {
	*h.closed++
	return h.refuse
}

func TestClose(t *testing.T) {
	t.Parallel()

	held := source.Language("held")

	t.Run("Close", func(t *testing.T) {
		t.Parallel()

		t.Run("stops every engine that holds something", func(t *testing.T) {
			t.Parallel()
			// A language server is a process. One left running per
			// language per run is a leak nobody sees until the machine is
			// out of memory.
			first, second := 0, 0
			c := engine.NewCatalog()
			mustAdd(t, c, running{name: "one", lang: held, closed: &first})
			mustAdd(t, c, running{name: "two", lang: held, closed: &second})

			assert.NoError(t, c.Close(t.Context()), "closing a catalogue of live engines succeeds")
			assert.Equal(t, first, 1, "the first was stopped")
			assert.Equal(t, second, 1, "and so was the second")
		})

		t.Run("leaves alone an engine that holds nothing", func(t *testing.T) {
			t.Parallel()
			// An in-process parser has nothing outside the process.
			// Making it implement the port would mean every adapter
			// carrying a method that returns nil.
			c := engine.NewCatalog()
			mustAdd(t, c, fake{name: "parser", lang: held})
			assert.NoError(t, c.Close(t.Context()), "an engine running nothing is not asked")
		})

		t.Run("asks the rest after one refuses", func(t *testing.T) {
			t.Parallel()
			// Stopping at the first failure would leave a process running
			// for every engine after it, which is the leak this exists to
			// prevent happening on the way out.
			broken := errors.New("engine: will not stop")
			first, second := 0, 0
			c := engine.NewCatalog()
			mustAdd(t, c, running{name: "one", lang: held, closed: &first, refuse: broken})
			mustAdd(t, c, running{name: "two", lang: held, closed: &second})

			err := c.Close(t.Context())
			assert.ErrorIs(t, err, broken, "the refusal reaches the caller")
			assert.Equal(t, second, 1, "and the engine after it was still stopped")
		})

		t.Run("reports every refusal rather than the first", func(t *testing.T) {
			t.Parallel()
			one, two := errors.New("engine: one stuck"), errors.New("engine: two stuck")
			first, second := 0, 0
			c := engine.NewCatalog()
			mustAdd(t, c, running{name: "one", lang: held, closed: &first, refuse: one})
			mustAdd(t, c, running{name: "two", lang: held, closed: &second, refuse: two})

			err := c.Close(t.Context())
			assert.ErrorIs(t, err, one, "the first refusal is reported")
			assert.ErrorIs(t, err, two, "and so is the second")
		})

		t.Run("does nothing to an empty catalogue", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, engine.NewCatalog().Close(t.Context()),
				"a binary that registered nothing has nothing to stop")
		})
	})
}

// assert the port is what the catalogue looks for.
var _ engine.Closer = running{fidelity: trust.None}
