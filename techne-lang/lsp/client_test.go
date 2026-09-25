// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// spaced is a file of the fake language indented with two spaces.
const spaced = "type Store struct {\n  size int\n}\n"

// secondFile is the file beside a.fake whose width the Asks mode asks for.
const secondFile = "b.fake"

// replies returns the replies of the client to the requests that the scripted server sends
// during initialize in the Asks mode over files, by request name.
func replies(t *testing.T, files map[string]string, options ...lsptest.Option) map[string]string {
	t.Helper()
	got, err := serving(t, lsptest.Asks, files, options...).
		Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
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
			assert.Equal(t, replies(t, sample())["configuration"], `[{"strict":true},null]`,
				"the reply to workspace/configuration")
		})

		t.Run("returns the indentation of a file under each section of the indentation", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, replies(t, sample())["indentation"],
				`[{"tabSize":4,"insertSpaces":false},4,false,null,null]`,
				"the reply for a.fake, a missing b.fake and no file")
		})

		t.Run("returns the width of a file indented with spaces", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, replies(t, map[string]string{"a.fake": spaced})["indentation"],
				`[{"tabSize":2,"insertSpaces":true},2,true,null,null]`,
				"the reply for a.fake, a missing b.fake and no file")
		})

		t.Run("returns the width of the file that an item scopes", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{"a.fake": lsptest.Content, secondFile: spaced}
			assert.Equal(t, replies(t, files)["indentation"],
				`[{"tabSize":4,"insertSpaces":false},4,false,2,null]`, "the reply for a.fake, b.fake and no file")
		})

		t.Run("returns null for a file larger than the files that techne reads", func(t *testing.T) {
			t.Parallel()
			large := strings.Repeat(spaced, lang.Largest/len(spaced)+1)
			files := map[string]string{"a.fake": lsptest.Content, secondFile: large}
			assert.Equal(t, replies(t, files)["indentation"],
				`[{"tabSize":4,"insertSpaces":false},4,false,null,null]`, "the reply for a.fake, b.fake and no file")
		})

		t.Run("returns null for a file outside the workspace", func(t *testing.T) {
			t.Parallel()
			beside := t.TempDir()
			rewrite(t, beside, secondFile, spaced)
			got := replies(t, sample(), lsptest.Outside(filepath.Join(beside, secondFile)))["indentation"]
			assert.HasSuffix(t, got, `,null,null,null]`, "the reply for the file outside the workspace")
		})
	})

	t.Run("WorkspaceFolders", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the workspace root", func(t *testing.T) {
			t.Parallel()
			reply := replies(t, sample())["folders"]
			assert.HasPrefix(t, reply, `[{"uri":"file://`, "the reply to workspace/workspaceFolders")
		})
	})

	t.Run("ApplyEdit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns applied false without an error", func(t *testing.T) {
			t.Parallel()
			reply := replies(t, sample())["edit"]
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
