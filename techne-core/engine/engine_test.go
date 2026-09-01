// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

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
			if !errors.Is(wrapped, engine.ErrDecline) {
				t.Error("a wrapped ErrDecline is no longer recognisable")
			}
		})

		t.Run("is not any other error", func(t *testing.T) {
			t.Parallel()
			if errors.Is(errors.New("gopls: exit status 1"), engine.ErrDecline) {
				t.Error("an unrelated error matched ErrDecline")
			}
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
			if _, declared := e.(engine.Available); declared {
				t.Error("outlineOnly declares Available; this case needs an engine without it")
			}
		})

		t.Run("reports why an engine cannot run", func(t *testing.T) {
			t.Parallel()
			var a engine.Available = unavailable{}
			if err := a.Available(context.Background()); err == nil {
				t.Error("an engine that cannot run must say so")
			}
		})
	})
}

// unavailable stands for an engine whose language server is not
// installed.
type unavailable struct{ outlineOnly }

func (unavailable) Available(context.Context) error {
	return errors.New("engine: gopls not on PATH")
}
