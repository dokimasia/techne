// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
)

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("ErrDecline", func(t *testing.T) {
		t.Parallel()

		t.Run("is recognisable through a wrapping", func(t *testing.T) {
			t.Parallel()
			// A service tells declining from failing to decide whether
			// to move to the next engine or stop. An adapter that adds
			// context must not break that.
			wrapped := fmt.Errorf("gotypes: %w", engine.ErrDecline)
			assert.ErrorIs(t, wrapped, engine.ErrDecline,
				"a service tells declining from failing, so context added by an adapter must not hide it")
		})

		t.Run("is not any other error", func(t *testing.T) {
			t.Parallel()
			assert.ErrorIsNot(t, errors.New("gopls: exit status 1"), engine.ErrDecline,
				"a real failure stops selection rather than falling through to a weaker engine")
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("is optional, so an in-process engine need not implement it", func(t *testing.T) {
			t.Parallel()
			// An engine with no outside dependency is always available.
			// Requiring the method would make every adapter carry one
			// that always returns nil.
			var e engine.Engine = outlineOnly{}
			_, declared := e.(engine.Available)
			assert.False(t, declared,
				"an engine with nothing outside the process to check carries no Available method")
		})

		t.Run("reports why an engine cannot run", func(t *testing.T) {
			t.Parallel()
			var a engine.Available = unavailable{}
			assert.HasError(t, a.Available(t.Context()),
				"an engine that cannot run says why rather than being silently skipped")
		})
	})
}

// unavailable stands for an engine whose language server is not
// installed.
type unavailable struct{ outlineOnly }

func (unavailable) Available(context.Context) error {
	return errors.New("engine: gopls not on PATH")
}
