// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/lsp"
)

// counting builds an engine whose server records a line each time it
// starts, and returns the engine and a function reading how many times
// that was.
//
// It is the only way to see the warm-server claim from outside: an
// engine that started a process per call answers exactly as one that
// keeps a process does, only slower.
func counting(t *testing.T) (*lsp.Engine, func() int) {
	t.Helper()
	dir := workspace(t, map[string]string{"a.fake": content})
	log := filepath.Join(t.TempDir(), "starts")

	server := pretending(modeDefault)
	server.Env[noting] = log

	e, err := lsp.New(dir, declared(), server)
	assert.NoError(t, err, "an engine builds from a declaration and a server")
	stopping(t, e)

	return e, func() int {
		held, err := os.ReadFile(log)
		if os.IsNotExist(err) {
			return 0
		}
		assert.NoError(t, err, "the case can count the starts")
		return strings.Count(string(held), "started")
	}
}

func TestSession(t *testing.T) {
	t.Parallel()

	t.Run("start", func(t *testing.T) {
		t.Parallel()

		t.Run("does not happen until something is asked", func(t *testing.T) {
			t.Parallel()
			// A binary serving ten languages must not start ten servers
			// to answer about one.
			_, started := counting(t)
			assert.Equal(t, started(), 0, "an engine nobody asked anything started nothing")
		})

		t.Run("happens once however many questions follow", func(t *testing.T) {
			t.Parallel()
			e, started := counting(t)
			for range 5 {
				_, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
				assert.NoError(t, err, "each question is answered")
			}
			assert.Equal(t, started(), 1,
				"one server answers all five, which is what makes the second question cheap")
		})

		t.Run("is not retried after it failed", func(t *testing.T) {
			t.Parallel()
			// Retrying a server that is not installed, once per call,
			// turns one clear refusal into a stall.
			e, err := lsp.New(workspace(t, map[string]string{"a.fake": content}),
				declared(), missing())
			assert.NoError(t, err, "a declaration for a server that is not here is still valid")

			for range 3 {
				_, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
				assert.HasError(t, err, "every call refuses")
				assert.ErrorIs(t, err, engine.ErrDecline, "and declines rather than breaking")
			}
		})

		t.Run("reports a server that dies during the handshake", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeDies, map[string]string{"a.fake": content})
			_, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.HasError(t, err, "a server that exits mid-handshake is a failed start")
			assert.ErrorIs(t, err, engine.ErrDecline,
				"declared so, because another engine may still answer")
		})
	})

	t.Run("stop", func(t *testing.T) {
		t.Parallel()

		t.Run("does nothing for an engine that started nothing", func(t *testing.T) {
			t.Parallel()
			e, err := lsp.New(workspace(t, map[string]string{"a.fake": content}),
				declared(), pretending(modeDefault))
			assert.NoError(t, err, "an engine builds")
			assert.NoError(t, e.Close(t.Context()), "closing one that never ran is not a fault")
		})

		t.Run("can be asked twice", func(t *testing.T) {
			t.Parallel()
			// A composition root closes, and so does whatever else holds
			// the engine. The second must not wait on a process already
			// reaped, which never returns.
			e := serving(t, modeDefault, map[string]string{"a.fake": content})
			_, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "the server started")

			assert.NoError(t, e.Close(t.Context()), "the first close stops it")
			assert.NoError(t, e.Close(t.Context()), "and the second does nothing")
		})

		t.Run("starts again after a close", func(t *testing.T) {
			t.Parallel()
			e, started := counting(t)
			_, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "the first question starts a server")
			assert.NoError(t, e.Close(t.Context()), "which is then stopped")

			_, err = e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "and the next question is still answered")
			assert.Equal(t, started(), 2, "by a second server, because the first was stopped")
		})
	})

	t.Run("a server that never answers", func(t *testing.T) {
		t.Parallel()

		t.Run("gives up when the caller's context does", func(t *testing.T) {
			t.Parallel()
			// A start with no deadline is a tool that hangs rather than
			// one that fails, and a hang is the hardest failure to
			// attribute.
			e := serving(t, modeSilent, map[string]string{"a.fake": content})

			assert.CompletesWithin(t, 5*time.Second, func(ctx context.Context) error {
				held, stop := context.WithTimeout(ctx, 300*time.Millisecond)
				defer stop()
				_, err := e.Outline(held, engine.Request{Scope: "a.fake"})
				assert.HasError(t, err, "the call ends with the context rather than waiting on")
				return nil
			}, "a server that will not answer does not hold the caller")
		})
	})
}

// missing is a declaration for a server that is not on this machine.
func missing() lsp.Server {
	held := pretending(modeDefault)
	held.Command = []string{"techne-no-such-language-server"}
	return held
}
