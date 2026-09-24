// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestServer(t *testing.T) {
	t.Parallel()

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a declaration that Server.Valid accepts", func(t *testing.T) {
			t.Parallel()
			for _, mode := range modes {
				assert.NoError(t, lsptest.Server(mode).Valid(), "Valid of the mode "+string(mode))
			}
		})

		t.Run("declares no tier for format", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, lsptest.Server(lsptest.Default).Fidelity(engine.RoleFormat), trust.None,
				"the tier of RoleFormat")
		})

		t.Run("declares settings for the Asks mode", func(t *testing.T) {
			t.Parallel()
			assert.NotEmpty(t, lsptest.Server(lsptest.Asks).Settings, "the settings of the Asks mode")
		})

		t.Run("declares a loading time for the Loading mode", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, lsptest.Server(lsptest.Loading).Loading, 3*lsptest.LoadTime,
				"the loading time of the Loading mode")
		})

		t.Run("declares an extraction for the Extracts mode", func(t *testing.T) {
			t.Parallel()
			assert.True(t, lsptest.Server(lsptest.Extracts).Extracts.Offered(),
				"Offered of the extraction of the Extracts mode")
		})
	})

	t.Run("Engine", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an engine for the language of the declaration", func(t *testing.T) {
			t.Parallel()
			e := lsptest.Engine(t, t.TempDir(), lsptest.Server(lsptest.Default))
			assert.Equal(t, e.Language(), lsptest.Language, "the language of the engine")
		})
	})

	t.Run("Parsing", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an engine for the language of the declaration", func(t *testing.T) {
			t.Parallel()
			e := lsptest.Parsing(t, t.TempDir(), lsptest.Server(lsptest.Default))
			assert.Equal(t, e.Language(), lsptest.Language, "the language of the engine")
		})
	})
}
