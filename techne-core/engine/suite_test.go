// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
)

// The words of the tests: the name of an engine and the suites it is asked about. unrunNote is
// a pin of the refusal as Unrun words it.
const (
	verifier  = "checker"
	lint      = "lint"
	tests     = "test"
	unrunNote = "checker runs only lint, and not test"
)

func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("Unrun", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for no suite", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, engine.Unrun(verifier, nil, nil), "Unrun of no suite")
		})

		t.Run("returns nil for the suites that the engine runs", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, engine.Unrun(verifier, []string{lint}, []string{lint}), "Unrun of lint")
		})

		t.Run("declines a suite that the engine does not run", func(t *testing.T) {
			t.Parallel()
			err := engine.Unrun(verifier, []string{lint}, []string{lint, tests, tests})
			assert.ErrorIs(t, err, engine.ErrDecline, "Unrun of test")
			assert.Contains(t, err.Error(), unrunNote, "the reason of the decline")
		})

		t.Run("names no suite for an engine that runs none", func(t *testing.T) {
			t.Parallel()
			err := engine.Unrun(verifier, nil, []string{tests})
			assert.Contains(t, err.Error(), "checker runs no suite, and not test", "the reason of the decline")
		})
	})
}
