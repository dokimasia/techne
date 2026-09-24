// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"path"
	"slices"
	"strings"
	"time"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
)

// roles are the roles whose port an [Engine] implements.
var roles = []engine.Role{
	engine.RoleResolve, engine.RoleRelate, engine.RolePlan,
	engine.RoleFormat, engine.RoleCheck, engine.RoleVerify,
}

// Server declares the language server of one language. A language module declares one whether
// or not the program is installed, and [Server.Installed] reports whether it is.
type Server struct {
	// Name is the name of the server, such as gopls or rust-analyzer. A provenance and a
	// capability report name the server by it.
	Name string

	// Command is the program and its arguments. [Server.Installed] looks the program up on
	// PATH.
	Command []string

	// LanguageID is the LSP 3.17 language identifier of the files of the language: one of the
	// Identity constants.
	LanguageID string

	// Dialects maps a file extension to the language identifier of a dialect that the same
	// server serves, such as ".tsx" to typescriptreact.
	Dialects map[string]string

	// Serves maps each role that the server serves to the tier of its answers. A role without
	// an entry is not served. [Binding] returns the map of a server with a type checker.
	Serves map[engine.Role]trust.Fidelity

	// Settings are sent as the initializationOptions of initialize and in reply to
	// workspace/configuration.
	Settings map[string]any

	// Env are the environment variables that the server runs with, added to the environment of
	// techne.
	Env map[string]string

	// Extracts is the code action that extracts a function, or the zero value for a server
	// without one.
	Extracts Refactor

	// Loading is how long a question waits for the server to settle. Zero waits 10 seconds. A
	// question that waits the whole time returns a partial answer.
	Loading time.Duration

	// Unchecked names what the diagnostics of the server leave out and the compiler of the
	// language checks, such as the lifetimes and borrows that rust-analyzer does not check, or
	// is empty for a server that checks what the compiler checks. Each check of the server
	// then has a [trust.CaveatPartialCheck] caveat.
	Unchecked string

	// Answering is how long a question waits for the server to answer, from the moment the
	// server runs. Zero waits one minute. A question that waits the whole time returns
	// [engine.ErrDecline], so the next engine answers, and the server receives a
	// $/cancelRequest for the request it did not answer.
	Answering time.Duration

	// Scoped reports that the server loads only the files that it has open and the files that
	// they import, as typescript-language-server does for a file that no tsconfig.json or
	// jsconfig.json includes. Most JavaScript repositories have neither. Before it plans a
	// rename or a move, the engine opens the files of the workspace that write the name, so
	// the server finds the uses in them.
	Scoped bool
}

// Refactor names the code action of a server that performs one refactoring.
//
// A kind does not identify one action. rust-analyzer offers four extractions under
// refactor.extract, typescript-language-server offers two under refactor.extract.function,
// and csharp-ls sets no kind. The title of the action does, so a language module declares
// the titles it established for its server.
type Refactor struct {
	// Kind is the code action kind to request. An action without a kind matches every kind.
	Kind string

	// Titles are the wordings of the action, best first. A title matches a wording that it
	// contains, ignoring case. An action that matches no wording is taken only when no action
	// matches one.
	Titles []string
}

// Offered reports whether r names an action: a kind, a title, or both.
func (r Refactor) Offered() bool { return r.Kind != "" || len(r.Titles) > 0 }

// Named returns the language identifier of the file at p: the identifier of its dialect when
// Dialects maps its extension, and LanguageID otherwise.
func (s Server) Named(p string) string {
	if dialect, declared := s.Dialects[path.Ext(p)]; declared {
		return dialect
	}
	return s.LanguageID
}

// Fidelity returns the tier that s declares for role, and [trust.None] for a role it does not
// serve.
func (s Server) Fidelity(role engine.Role) trust.Fidelity {
	if fidelity, declared := s.Serves[role]; declared {
		return fidelity
	}
	return trust.None
}

// Installed returns nil when the program of Command is on PATH, and an error that names the
// program otherwise.
func (s Server) Installed() error {
	if len(s.Command) == 0 {
		return fmt.Errorf("lsp: %s: the declaration has no command", s.Name)
	}
	if _, err := exec.LookPath(s.Command[0]); err != nil {
		return fmt.Errorf("lsp: %s: %s is not on PATH", s.Name, s.Command[0])
	}
	return nil
}

// Valid returns an error in these cases:
//
//   - s lacks a name, a command, a language identifier or a role.
//   - s declares a role that an [Engine] does not serve.
//   - s declares a role at [trust.None].
func (s Server) Valid() error {
	switch {
	case strings.TrimSpace(s.Name) == "":
		return errors.New("lsp: the server declaration has no name")
	case len(s.Command) == 0:
		return fmt.Errorf("lsp: %s: the declaration has no command", s.Name)
	case strings.TrimSpace(s.LanguageID) == "":
		return fmt.Errorf("lsp: %s: the declaration has no language identifier", s.Name)
	case len(s.Serves) == 0:
		return fmt.Errorf("lsp: %s: the declaration serves no role", s.Name)
	}
	for _, role := range slices.Sorted(maps.Keys(s.Serves)) {
		switch {
		case !slices.Contains(roles, role):
			return fmt.Errorf("lsp: %s: the declaration claims %s, which the engine does not serve", s.Name, role)
		case s.Serves[role] == trust.None:
			return fmt.Errorf("lsp: %s: the declaration claims %s at no tier", s.Name, role)
		}
	}
	return nil
}
