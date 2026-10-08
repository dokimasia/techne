// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestWatching(t *testing.T) {
	t.Parallel()

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("reads a file that changed on disk without a buffer", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Content, "b.fake": lsptest.Content})
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Watching))
			before := verifyA(t, e)
			assert.Equal(t, mentions(before, lsptest.Broken), 0, "the findings of a.fake before b.fake changes")

			rewrite(t, root, "b.fake", lsptest.Faulty)
			assert.Equal(t, mentions(verifyA(t, e), lsptest.Broken), 1, "the findings of a.fake after b.fake changes")
		})

		t.Run("reports the creation, the change and the deletion of watched files", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{
				"a.fake": lsptest.Content, "b.fake": lsptest.Content, "d.fake": lsptest.Content,
			})
			log := filepath.Join(t.TempDir(), "requests")
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Watching, lsptest.RecordRequests(log)))
			verifyA(t, e)

			rewrite(t, root, "b.fake", lsptest.Faulty)
			rewrite(t, root, "c.fake", lsptest.Content)
			assert.NoError(t, os.Remove(filepath.Join(root, "d.fake")), "the test removes d.fake")
			verifyA(t, e)
			assert.Equal(t, watchedIn(t, log), []string{
				lsptest.Watched + " changed b.fake", lsptest.Watched + " created c.fake",
				lsptest.Watched + " deleted d.fake",
			}, "the events that the server received")
		})

		t.Run("reports only the kinds of events that a watcher asks for", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Content})
			log := filepath.Join(t.TempDir(), "requests")
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Watching, lsptest.RecordRequests(log)))
			verifyA(t, e)

			fresh := "x" + lsptest.Fresh
			rewrite(t, root, fresh, "1")
			verifyA(t, e)
			rewrite(t, root, fresh, "22")
			verifyA(t, e)
			assert.NoError(t, os.Remove(filepath.Join(root, fresh)), "the test removes "+fresh)
			verifyA(t, e)
			assert.Equal(t, watchedIn(t, log), []string{lsptest.Watched + " created " + fresh},
				"the events that the server received")
		})

		t.Run("stops reporting the files of a registration that the server removed", func(t *testing.T) {
			t.Parallel()
			dropped := "x" + lsptest.Dropped
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Content, dropped: "1"})
			log := filepath.Join(t.TempDir(), "requests")
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Watching, lsptest.RecordRequests(log)))
			verifyA(t, e)

			rewrite(t, root, dropped, "22")
			verifyA(t, e)
			rewrite(t, root, dropped, "333")
			verifyA(t, e)
			assert.Equal(t, watchedIn(t, log), []string{lsptest.Watched + " changed " + dropped},
				"the events that the server received")
		})

		t.Run("reports the change of a file with a buffer once", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Content})
			log := filepath.Join(t.TempDir(), "requests")
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Watching, lsptest.RecordRequests(log)))
			verifyA(t, e)

			rewrite(t, root, "a.fake", lsptest.Content+"\n")
			verifyA(t, e)
			assert.Equal(t, watchedIn(t, log), []string{lsptest.Watched + " changed a.fake"},
				"the events that the server received")
		})
	})
}

// verifyA returns the findings of a verify of a.fake.
func verifyA(t *testing.T, e *lsp.Engine) []edit.Finding {
	t.Helper()
	got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
	assert.NoError(t, err, "Verify of a.fake")
	return got.Items
}

// watchedIn returns the lines of the log of [lsptest.RecordRequests] that record an event of
// workspace/didChangeWatchedFiles, in order. A log that the server never wrote has none.
func watchedIn(t *testing.T, log string) []string {
	t.Helper()
	recorded, err := os.ReadFile(log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	assert.NoError(t, err, "the test reads "+log)
	var out []string
	for line := range strings.Lines(string(recorded)) {
		if strings.HasPrefix(line, lsptest.Watched+" ") {
			out = append(out, strings.TrimSuffix(line, "\n"))
		}
	}
	return out
}
