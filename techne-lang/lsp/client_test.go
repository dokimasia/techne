// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
)

// A server asks the client things during startup and blocks on the
// replies, so what this package answers is not decorative: an
// unanswered request is a server that never finishes starting, and the
// symptom is a call that hangs rather than one that fails.
//
// The fake asks the three that matter, keeps what came back, and reports
// it as declaration names. These cases read the names.
func TestClient(t *testing.T) {
	t.Parallel()

	// asked runs the mode where the fake interrogates the client, and
	// returns what it said, keyed by the request it answered.
	asked := func(t *testing.T) map[string]string {
		t.Helper()
		got, err := serving(t, "asks", map[string]string{"a.fake": content}).
			Outline(t.Context(), engine.Request{Scope: "a.fake"})
		assert.NoError(t, err, "a server that asks the client questions still answers")

		out := map[string]string{}
		for _, one := range got.Items {
			of, said, split := strings.Cut(one.Name, "=")
			assert.True(t, split, "the fake reports one callback per declaration")
			out[of] = said
		}
		return out
	}

	t.Run("Configuration", func(t *testing.T) {
		t.Parallel()

		t.Run("answers one item per item asked about", func(t *testing.T) {
			t.Parallel()
			// A server matches the answers to its items by position. A
			// single value, or a list of a different length, is read
			// against the wrong section, and the server is configured as
			// something nobody asked for.
			said := asked(t)["configuration"]
			assert.Equal(t, said, `[{"strict":true},null]`,
				"one answer per item, in the item's own position, and the declared "+
					"section is the one that carries a value")
		})

		t.Run("answers a section nothing was declared for with null", func(t *testing.T) {
			t.Parallel()
			// Null says there is no setting. An empty object says the
			// setting exists and is blank, which for an option that
			// defaults to on configures a different server.
			assert.True(t, strings.HasSuffix(asked(t)["configuration"], ",null]"),
				"a section nothing was declared for is not set, rather than set to nothing")
		})

		t.Run("is refused by nobody, so a server finishes starting", func(t *testing.T) {
			t.Parallel()
			assert.False(t, strings.HasPrefix(asked(t)["configuration"], "refused"),
				"the default refusal would leave a configuration-driven server unconfigured")
		})
	})

	t.Run("WorkspaceFolders", func(t *testing.T) {
		t.Parallel()

		t.Run("names the one root rather than none", func(t *testing.T) {
			t.Parallel()
			// A server told no folders are open indexes nothing and
			// answers every workspace-wide question with an empty list,
			// which reads exactly like a correct answer.
			said := asked(t)["folders"]
			assert.False(t, strings.HasPrefix(said, "refused"), "the request is answered")
			assert.True(t, strings.Contains(said, "file://"),
				"and the answer names a folder")
		})
	})

	t.Run("ApplyEdit", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses, so a server cannot write behind the gate", func(t *testing.T) {
			t.Parallel()
			// techne plans a change, gates it and applies it atomically.
			// An edit arriving this way is subject to none of that, and a
			// caller told a change was refused would find it on disk.
			said := asked(t)["edit"]
			assert.True(t, strings.Contains(said, `"applied":false`),
				"the edit is not applied")
			assert.False(t, strings.HasPrefix(said, "refused"),
				"and it is answered rather than errored, because the server asked fairly")
		})
	})

	t.Run("RegisterCapability", func(t *testing.T) {
		t.Parallel()

		t.Run("is accepted, so a server that registers still starts", func(t *testing.T) {
			t.Parallel()
			// The fake registers during initialise and waits for the
			// reply before it answers. Refusing it is an error response,
			// and a server treating one as fatal never gets further.
			got, err := serving(t, "", map[string]string{"a.fake": content}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "a server that registers a capability finishes starting")
			_, found := named(got.Items, "Store")
			assert.True(t, found, "and goes on to answer")
		})
	})
}
