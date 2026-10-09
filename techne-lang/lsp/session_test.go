// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/lsp"
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
			assert.Contains(t, err.Error(), "the start goes on", "the error of Resolve")
			assert.Contains(t, err.Error(), lsptest.Waiting, "the error of Resolve")
			assert.InRange(t, time.Since(began), -1<<63, float64(time.Minute-1), "Resolve returns within a minute")
		})

		t.Run("uses a server whose handshake ends after a question stops waiting", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "starts")
			e := lsptest.Engine(t, lsptest.Workspace(t, sample()),
				lsptest.Server(lsptest.Slow, lsptest.RecordStarts(log)))
			impatient(t, e)

			got, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve after the question that stopped waiting")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations that Store denotes")
			assert.Equal(t, starts(t, log), 1, "the number of servers started")
		})

		t.Run("keeps the failure of a handshake that ends after a question stops waiting", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "starts")
			e := lsptest.Engine(t, lsptest.Workspace(t, sample()),
				lsptest.Server(lsptest.DiesLate, lsptest.RecordStarts(log)))
			impatient(t, e)

			for range 2 {
				_, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
				assert.ErrorIs(t, err, engine.ErrDecline, "the error of Resolve")
				assert.Contains(t, err.Error(), lsptest.Dying, "the error of Resolve")
			}
			assert.Equal(t, starts(t, log), 1, "the number of servers started")
		})
	})

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps the last of two reports of a file in the order of the stream", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.DiskChecks, sample()).
				Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "Verify after an interim report and the report of the check")
			assert.Equal(t, mentions(got.Items, "interim"), 0, "the notes of the interim report")
			assert.Equal(t, mentions(got.Items, "checks=1"), 1, "the note of the report of the check")
		})
	})
}

// impatient asks e a question whose context ends a fifth of [lsptest.SlowStart] after it
// starts the server, before a Slow or DiesLate server responds to initialize, and checks that
// the question fails.
func impatient(t *testing.T, e *lsp.Engine) {
	t.Helper()
	short, stop := context.WithTimeout(t.Context(), lsptest.SlowStart/5)
	defer stop()
	_, err := e.Resolve(short, engine.Request{Scope: "a.fake"}, store())
	assert.HasError(t, err, "Resolve before the server responds to initialize")
}
