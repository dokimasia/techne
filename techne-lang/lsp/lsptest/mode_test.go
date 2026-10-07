// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// modes are every mode of the scripted server.
var modes = []lsptest.Mode{
	lsptest.Default, lsptest.Silent, lsptest.Slow, lsptest.Dies, lsptest.DiesLate, lsptest.Orphans,
	lsptest.Empty, lsptest.Unicode,
	lsptest.Flat, lsptest.OneLocation, lsptest.Links, lsptest.Unresolved, lsptest.Exports, lsptest.Pointed,
	lsptest.Unnameable, lsptest.Strict, lsptest.Pushes, lsptest.PushesOne, lsptest.Quiet, lsptest.Loads,
	lsptest.Asks,
	lsptest.Uncallable, lsptest.Untyped, lsptest.Cancels, lsptest.Loading, lsptest.Stuck, lsptest.Created,
	lsptest.Burst,
	lsptest.Hangs, lsptest.Ungated,
	lsptest.Thin, lsptest.Echoes, lsptest.Moveless, lsptest.SilentMove, lsptest.Extracts, lsptest.Commands,
	lsptest.Watches, lsptest.Opened, lsptest.Short, lsptest.Scoped, lsptest.Conflicts, lsptest.Unenclosed,
	lsptest.Compiles, lsptest.DiskChecks, lsptest.DiskStuck, lsptest.Diagnoses, lsptest.DiagnosisStuck,
	lsptest.Unbound, lsptest.WorkspaceDiagnostics,
	lsptest.Canonical, lsptest.Receivers, lsptest.Impls, lsptest.Wrapped, lsptest.Minified, lsptest.Nested,
	lsptest.Aims, lsptest.Mutes, lsptest.Exits, lsptest.Uncalled, lsptest.Projects, lsptest.Redeclares,
	lsptest.Contextual, lsptest.Projected, lsptest.Qualified, lsptest.FromUse, lsptest.Shorthand,
	lsptest.Resolves, lsptest.ResolvesLate,
}

func TestMode(t *testing.T) {
	t.Parallel()

	t.Run("Mode", func(t *testing.T) {
		t.Parallel()

		t.Run("names each behaviour with a distinct string", func(t *testing.T) {
			t.Parallel()
			assert.NoDuplicates(t, func() ([]lsptest.Mode, error) { return modes, nil }, "the modes")
		})

		t.Run("has Default as its zero value", func(t *testing.T) {
			t.Parallel()
			var zero lsptest.Mode
			assert.Equal(t, zero, lsptest.Default, "the zero Mode")
		})
	})
}
