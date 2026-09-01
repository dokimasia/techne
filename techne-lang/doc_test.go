// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"context"
	"testing"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang"
)

// TestDoc covers the contract the package comment states across the
// declaration and the registry together.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("a language is a value the caller chooses", func(t *testing.T) {
		t.Parallel()

		t.Run("so one registry can hold a subset of another", func(t *testing.T) {
			t.Parallel()
			// Registration is an explicit call rather than an init with
			// a blank import. A test builds a registry holding one
			// language; a smaller binary ships a subset of the same
			// calls.
			full, cat := lang.NewRegistry(), engine.NewCatalog()
			if err := full.Register(cat, declared(), stub{fixture}); err != nil {
				t.Fatalf("Register: %v", err)
			}

			empty := lang.NewRegistry()
			if got := len(empty.Languages()); got != 0 {
				t.Errorf("a second registry holds %d languages, want 0", got)
			}
			if got := len(full.Languages()); got != 1 {
				t.Errorf("the first registry holds %d languages, want 1", got)
			}
			if _, routed := empty.LanguageOf("a.fx"); routed {
				t.Error("a registry nobody registered into routed a path")
			}
			if got := cat.For(context.Background(), fixture, engine.RoleOutline); len(got) != 1 {
				t.Errorf("the catalogue holds %d engines, want 1", len(got))
			}
		})
	})
}
