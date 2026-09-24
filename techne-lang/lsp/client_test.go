// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// replies returns the replies of the client to the requests that the scripted server sends
// during initialize in the Asks mode, by request name.
func replies(t *testing.T) map[string]string {
	t.Helper()
	got, err := serving(t, lsptest.Asks, sample()).Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
	assert.NoError(t, err, "Verify in the Asks mode")
	out := map[string]string{}
	for _, message := range messages(got.Items) {
		name, reply, split := strings.Cut(message, "=")
		assert.True(t, split, "the message "+message+" names a request")
		out[name] = reply
	}
	return out
}

func TestClient(t *testing.T) {
	t.Parallel()

	t.Run("Configuration", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one value per item in the order of the items", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, replies(t)["configuration"], `[{"strict":true},null]`,
				"the reply to workspace/configuration")
		})
	})

	t.Run("WorkspaceFolders", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the workspace root", func(t *testing.T) {
			t.Parallel()
			reply := replies(t)["folders"]
			assert.HasPrefix(t, reply, `[{"uri":"file://`, "the reply to workspace/workspaceFolders")
		})
	})

	t.Run("ApplyEdit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns applied false without an error", func(t *testing.T) {
			t.Parallel()
			reply := replies(t)["edit"]
			assert.Contains(t, reply, `"applied":false`, "the reply to workspace/applyEdit")
			assert.False(t, strings.HasPrefix(reply, "refused"), "the reply to workspace/applyEdit is an error")
		})
	})

	t.Run("RegisterCapability", func(t *testing.T) {
		t.Parallel()

		t.Run("accepts a registration during initialize", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).
				Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "Resolve after the server registered a capability")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declarations that Store denotes")
		})
	})
}
