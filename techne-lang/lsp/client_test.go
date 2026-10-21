// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// spaced is a file of the fake language indented with two spaces.
const spaced = "type Store struct {\n  size int\n}\n"

// secondFile is the file beside a.fake whose width the Asks mode asks for.
const secondFile = "b.fake"

func TestClient(t *testing.T) {
	t.Parallel()

	t.Run("Configuration", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one value per item in the order of the items", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, replies(t, sample())["configuration"], `[{"strict":true},null,{"fake":{"strict":true}}]`,
				"the reply to workspace/configuration")
		})

		t.Run("returns null for every item from a server without settings", func(t *testing.T) {
			t.Parallel()
			unsettled := func(s *lsp.Server) { s.Settings = nil }
			got := replies(t, sample(), unsettled, configuring(nil))
			assert.Equal(t, got["configuration"], `[null,null,null]`, "the reply to workspace/configuration")
		})

		t.Run("returns the section that Configure returns in place of the declared section", func(t *testing.T) {
			t.Parallel()
			got := replies(t, sample(), configuring(map[string]any{"fake": map[string]any{"strict": false}}))
			assert.Equal(t, got["configuration"], `[{"strict":false},null,{"fake":{"strict":false}}]`,
				"the reply to workspace/configuration")
		})

		t.Run("returns a declared section that Configure does not return", func(t *testing.T) {
			t.Parallel()
			got := replies(t, sample(), configuring(map[string]any{"absent": 1}))
			assert.Equal(t, got["configuration"], `[{"strict":true},1,{"absent":1,"fake":{"strict":true}}]`,
				"the reply to workspace/configuration")
		})

		t.Run("returns the settings of Configure for a server without settings of its own", func(t *testing.T) {
			t.Parallel()
			unsettled := func(s *lsp.Server) { s.Settings = nil }
			got := replies(t, sample(), unsettled, configuring(map[string]any{"absent": 1}))
			assert.Equal(t, got["configuration"], `[null,1,{"absent":1}]`, "the reply to workspace/configuration")
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
