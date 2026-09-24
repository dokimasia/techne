// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestSession(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("starts no server", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "starts")
			lsptest.Engine(t, lsptest.Workspace(t, sample()),
				lsptest.Server(lsptest.Default, lsptest.RecordStarts(log)))
			assert.Equal(t, starts(t, log), 0, "the number of servers started without a question")
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("starts one server for five questions", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "starts")
			e := lsptest.Engine(t, lsptest.Workspace(t, sample()),
				lsptest.Server(lsptest.Default, lsptest.RecordStarts(log)))
			for range 5 {
				_, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
				assert.NoError(t, err, "each Resolve")
			}
			assert.Equal(t, starts(t, log), 1, "the number of servers started")
		})

		t.Run("declines every question for a server that is not installed", func(t *testing.T) {
			t.Parallel()
			e := lsptest.Engine(t, lsptest.Workspace(t, sample()), missing())
			for range 3 {
				_, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
				assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve without a server")
			}
		})

		t.Run("declines when the server exits during the handshake", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Dies, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve")
		})

		t.Run("returns the stderr of a server that exits during the handshake", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Dies, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.HasError(t, err, "Resolve with a server that exits")
			assert.Contains(t, err.Error(), lsptest.Dying, "the error of Resolve")
		})

		t.Run("returns when the context ends before the handshake", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Silent, sample())
			assert.CompletesWithin(t, 5*time.Second, func(ctx context.Context) error {
				short, stop := context.WithTimeout(ctx, 300*time.Millisecond)
				defer stop()
				_, err := e.Resolve(short, engine.Request{Scope: "a.fake"}, store())
				assert.HasError(t, err, "Resolve with a server that never answers")
				return nil
			}, "Resolve returns when its context ends")
		})

		t.Run("starts again after the context of a question ends", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			cancelled, cancel := context.WithCancel(t.Context())
			cancel()
			_, err := e.Resolve(cancelled, engine.Request{Scope: "a.fake"}, store())
			assert.HasError(t, err, "Resolve with a cancelled context")

			got, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve with a live context")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations that Store denotes")
		})

		t.Run("stops waiting for a server that never responds to initialize", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Silent, sample())
			began := time.Now()
			_, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve")
			assert.Contains(t, err.Error(), "initialize", "the error of Resolve")
			assert.True(t, time.Since(began) < time.Minute, "Resolve returns within a minute")
		})
	})
}
