// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest_test

import (
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp"
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

		t.Run("declares the sections of the indentation for the Asks mode", func(t *testing.T) {
			t.Parallel()
			got := lsptest.Server(lsptest.Asks).Indentation
			assert.NotEqual(t, got, lsp.Indentation{}, "the indentation sections of the Asks mode")
			assert.NotEqual(t, got.Size, got.Spaces, "the width and the spaces sections of the Asks mode")
		})

		t.Run("declares a loading time for the Loading mode", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, lsptest.Server(lsptest.Loading).Loading, 3*lsptest.LoadTime,
				"the loading time of the Loading mode")
		})

		t.Run("declares the diagnosis of DiagnosisPrefix for the Diagnoses mode", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, lsptest.Server(lsptest.Diagnoses).Diagnosis, lsptest.DiagnosisPrefix,
				"the diagnosis of the Diagnoses mode")
		})

		t.Run("declares the diagnosis of DiagnosisPrefix for the DiagnosisStuck mode", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, lsptest.Server(lsptest.DiagnosisStuck).Diagnosis, lsptest.DiagnosisPrefix,
				"the diagnosis of the DiagnosisStuck mode")
		})

		t.Run("declares a loading time of one second for the DiagnosisStuck mode", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, lsptest.Server(lsptest.DiagnosisStuck).Loading, time.Second,
				"the loading time of the DiagnosisStuck mode")
		})

		t.Run("declares an extraction for the Extracts mode", func(t *testing.T) {
			t.Parallel()
			assert.True(t, lsptest.Server(lsptest.Extracts).Extracts.Offered(),
				"Offered of the extraction of the Extracts mode")
		})

		t.Run("declares Imports for the Resolves mode", func(t *testing.T) {
			t.Parallel()
			assert.True(t, lsptest.Server(lsptest.Resolves).Imports, "Imports of the Resolves mode")
		})

		t.Run("declares Imports for the ResolvesLate mode", func(t *testing.T) {
			t.Parallel()
			assert.True(t, lsptest.Server(lsptest.ResolvesLate).Imports, "Imports of the ResolvesLate mode")
		})

		t.Run("declares a Resolving of three times LateStart for the ResolvesLate mode", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, lsptest.Server(lsptest.ResolvesLate).Resolving, 3*lsptest.LateStart,
				"Resolving of the ResolvesLate mode")
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
