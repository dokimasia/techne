// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/lsp"
)

// The settings on the wire, as each server reads them. Each is a pin of the name that the
// server reads: typescript-language-server 6.0.0 from initializationOptions.preferences, and
// tsc 7.0.2 from the section js/ts of workspace/configuration.
const (
	typeScriptWire = `{"preferences":{"providePrefixAndSuffixTextForRename":false}}`
	nativeWire     = `{"js/ts":{"preferences":{"useAliasesForRenames":false}}}`
)

func TestTypeScript(t *testing.T) {
	t.Parallel()

	t.Run("TypeScriptSettings", func(t *testing.T) {
		t.Parallel()

		t.Run("turns off the prefix and the suffix of a rename", func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(lsp.TypeScriptSettings())
			assert.NoError(t, err, "Marshal of the settings")
			assert.Equal(t, string(raw), typeScriptWire, "the settings on the wire")
		})

		t.Run("returns a new map on each call", func(t *testing.T) {
			t.Parallel()
			lsp.TypeScriptSettings()["preferences"] = nil
			raw, err := json.Marshal(lsp.TypeScriptSettings())
			assert.NoError(t, err, "Marshal of the settings")
			assert.Equal(t, string(raw), typeScriptWire, "the settings after a change to another map")
		})
	})

	t.Run("NativeSettings", func(t *testing.T) {
		t.Parallel()

		t.Run("turns off the aliases of a rename under js/ts", func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(lsp.NativeSettings())
			assert.NoError(t, err, "Marshal of the settings")
			assert.Equal(t, string(raw), nativeWire, "the settings on the wire")
		})

		t.Run("returns a new map on each call", func(t *testing.T) {
			t.Parallel()
			lsp.NativeSettings()["js/ts"] = nil
			raw, err := json.Marshal(lsp.NativeSettings())
			assert.NoError(t, err, "Marshal of the settings")
			assert.Equal(t, string(raw), nativeWire, "the settings after a change to another map")
		})
	})
}
