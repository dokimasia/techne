// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

// TypeScriptSettings returns a new map of the [Server.Settings] of typescript-language-server,
// which the TypeScript and the JavaScript modules both declare. The server sends the tsserver
// preference providePrefixAndSuffixTextForRename as false before a rename.
//
// With the preference on, the default of the server and of an editor, the rename of a
// declaration that an export statement re-exports writes `New as Old` in the export statement,
// and every use that imports Old keeps the old name. With it off, the rename rewrites every
// use. typescript-language-server 6.0.0 merges initializationOptions.preferences into the
// preferences that it sends tsserver.
func TypeScriptSettings() map[string]any {
	return map[string]any{
		"preferences": map[string]any{"providePrefixAndSuffixTextForRename": false},
	}
}

// NativeSettings returns a new map of the [Server.Settings] of tsc --lsp of TypeScript 7,
// which the TypeScript and the JavaScript modules both declare. The server reads
// preferences.useAliasesForRenames under js/ts as false, and renames every use of a
// re-exported declaration, as [TypeScriptSettings] describes.
//
// tsc 7.0.2 requests the sections js/ts, typescript, javascript and editor through
// workspace/configuration, and js/ts takes precedence over the others. It ignores the same map
// as initializationOptions.
func NativeSettings() map[string]any {
	return map[string]any{
		"js/ts": map[string]any{
			"preferences": map[string]any{"useAliasesForRenames": false},
		},
	}
}
