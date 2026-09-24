// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.lsp.dev/protocol"
)

// The LSP 3.17 language identifiers of the languages that techne serves, from the constants
// of the protocol binding. A language module sets [Server.LanguageID] to one of them. A server
// analyses a file under the identifier it is opened with, and returns no result for a file
// whose identifier it does not know.
const (
	IdentityC          = string(protocol.LanguageKindC)
	IdentityCSharp     = string(protocol.LanguageKindCSharp)
	IdentityGo         = string(protocol.LanguageKindGo)
	IdentityJava       = string(protocol.LanguageKindJava)
	IdentityJavaScript = string(protocol.LanguageKindJavaScript)
	// IdentityJavaScriptReact is the dialect of .jsx files. typescript-language-server parses
	// JSX only in a file opened under a react identifier.
	IdentityJavaScriptReact = string(protocol.LanguageKindJavaScriptReact)
	// IdentityTypeScriptReact is the dialect of .tsx files.
	IdentityTypeScriptReact = string(protocol.LanguageKindTypeScriptReact)
	IdentityPython          = string(protocol.LanguageKindPython)
	IdentityRuby            = string(protocol.LanguageKindRuby)
	IdentityRust            = string(protocol.LanguageKindRust)
	IdentityScala           = string(protocol.LanguageKindScala)
	IdentityTypeScript      = string(protocol.LanguageKindTypeScript)
)

// Binding returns a new map of the tiers of a server with a type checker: [trust.Resolved] for
// resolve, relate, plan, format, check and verify. A language module whose server does less
// deletes roles from the map, as the C module deletes check.
//
// Format is the formatter of the language, which no parser reproduces. Outline and search are
// absent, because an [Engine] serves neither. The tree-sitter engine outlines a file as well as
// a server does, and a workspace symbol query returns a capped list that cannot show a name
// is absent.
func Binding() map[engine.Role]trust.Fidelity {
	return map[engine.Role]trust.Fidelity{
		engine.RoleResolve: trust.Resolved,
		engine.RoleRelate:  trust.Resolved,
		engine.RolePlan:    trust.Resolved,
		engine.RoleFormat:  trust.Resolved,
		engine.RoleCheck:   trust.Resolved,
		engine.RoleVerify:  trust.Resolved,
	}
}
