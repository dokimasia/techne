// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.lsp.dev/protocol"
)

// What a language is called in the protocol, for the languages techne
// serves.
//
// The protocol's own list rather than techne's, and taken from the
// protocol's own constants rather than written out again: a module
// declaring "golang" would open every file under a name no server
// recognises and be answered about nothing, which in a tool whose job
// includes reporting that it found nothing is the hardest failure to
// notice. A language module names one of these rather than a string,
// so the compiler catches what a server never would.
const (
	IdentityC          = string(protocol.LanguageKindC)
	IdentityCSharp     = string(protocol.LanguageKindCSharp)
	IdentityGo         = string(protocol.LanguageKindGo)
	IdentityJava       = string(protocol.LanguageKindJava)
	IdentityJavaScript = string(protocol.LanguageKindJavaScript)
	IdentityPython     = string(protocol.LanguageKindPython)
	IdentityRuby       = string(protocol.LanguageKindRuby)
	IdentityRust       = string(protocol.LanguageKindRust)
	IdentityScala      = string(protocol.LanguageKindScala)
	IdentityTypeScript = string(protocol.LanguageKindTypeScript)
)

// Binding is what a server that runs a type checker reaches, per role.
//
// Resolve, relate, plan and verify, because binding is what answers
// those and nothing else does. A parser sees a name and no declaration
// behind it; a server has type-checked the workspace and knows which one
// the name reaches, in this file or another.
//
// Outline and search are absent, so a parser keeps them:
//
//   - Outlining one file, a server does no better than a parser does at
//     a thousandth of the speed. Claiming the role would make every
//     outline start a process to produce the same answer.
//   - A workspace symbol query is answered from an index the server caps
//     without saying so, which can never support a claim that something
//     is absent. A parser reads every file in the scope and can.
//
// A language whose server is weaker than this declares its own map. This
// is the shape nearly all of them share, and ten copies of it would be
// nine chances for one to drift.
func Binding() map[engine.Role]trust.Fidelity {
	return map[engine.Role]trust.Fidelity{
		engine.RoleResolve: trust.Resolved,
		engine.RoleRelate:  trust.Resolved,
		engine.RolePlan:    trust.Resolved,
		engine.RoleVerify:  trust.Resolved,
		// Gating a change is the same analysis as verifying a file, over
		// content nobody has written yet. A parser serves it too and
		// says only that the result is still the language it was; this
		// says it still means something, which is what a change that
		// renames one thing onto another needs.
		engine.RoleCheck: trust.Resolved,
		// Formatting is not a binding, and it is still the server's:
		// what it returns is the language's own formatter, which no
		// parser can reproduce and nothing else here has.
		engine.RoleFormat: trust.Resolved,
	}
}
