// Copyright Dokimasia B.V. 2026
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
	"go.dokimi.dev/techne/core/sema"
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

	// Indentation names the sections of workspace/configuration under which the server requests
	// the indentation of a file, which it applies to the code of a code action. For a file of
	// the workspace, the client returns the indentation of the file under them.
	Indentation Indentation

	// Name is the name of the server, such as gopls or rust-analyzer. A provenance and a
	// capability report name the server by it.
	Name string

	// LanguageID is the LSP 3.17 language identifier of the files of the language: one of the
	// Identity constants.
	LanguageID string

	// Unchecked names what the diagnostics of the server leave out and the compiler of the
	// language checks, such as the lifetimes and borrows that rust-analyzer does not check, or
	// is empty for a server that checks what the compiler checks. Each check of the server
	// then has a [trust.CaveatPartialCheck] caveat.
	Unchecked string

	// DiskCheck is the prefix of the work-done progress token of the check that the server runs
	// over the files on disk after textDocument/didSave, such as rust-analyzer/flycheck/ for the
	// cargo check of rust-analyzer, or empty for a server without one. The server publishes the
	// findings of that check, and the findings describe the files on disk.
	//
	// The engine sends textDocument/didSave for a file whose content on disk changed since the
	// last check. A verification waits for a check that began after the last save to end, and
	// its findings contain those of the check. It then has no [trust.CaveatPartialCheck] caveat.
	DiskCheck string

	// Diagnosis is the start of the title of the work-done progress job in which the server
	// diagnoses a change of its buffers and files, such as diagnosing for gopls with the setting
	// verboseWorkDoneProgress, or empty for a server without one. gopls begins the job in its
	// handler of each notification that changes a file, and ends it after it has published the
	// diagnostics of the change from every build that contains the file.
	//
	// A question does not wait for a diagnosis when it waits for the server to settle. A
	// verification and a check send the server a request after their buffers, and then wait up
	// to [Server.Loading] for every diagnosis to end. A server that handles its messages in
	// order, as gopls does, begins the diagnosis of each buffer before it replies to the request,
	// so the reports that they read describe the buffers that they sent.
	Diagnosis string

	// Extracts is the code action that extracts a function, or the zero value for a server
	// without one.
	Extracts Refactor

	// Command is the program and its arguments. [Server.Installed] looks the program up on
	// PATH.
	Command []string

	// Unrelated are the relations whose request the server answers with other relations, which
	// the engine declines, so another engine of the language answers them. gopls answers the
	// type hierarchy of a Go type with the interfaces that the type implements, which are not
	// the types that it embeds.
	Unrelated []sema.RelationKind

	// Loading is how long a question waits for the server to settle. Zero waits 10 seconds. A
	// question that waits the whole time returns a partial answer.
	Loading time.Duration

	// Answering is how long a question waits for the reply of the server, from the moment the
	// server runs. Zero waits one minute. A question that waits the whole time returns
	// [engine.ErrDecline], so the next engine serves it, and the server receives a
	// $/cancelRequest for the request without a reply.
	Answering time.Duration

	// Checking is how long a check waits for a server without pull diagnostics to publish the
	// diagnostics of the changed files, from the moment the server settles. Zero waits 30
	// seconds. A check that waits the whole time declines, so a weaker engine checks the write.
	Checking time.Duration

	// Resolving is how long after its start the server can resolve an import to no file and later
	// to the imported file, with no work-done progress job in between. metals does so while it
	// compiles the build that it imported. Until the server has run that long, [Engine.importers]
	// asks again at such an import once a second. Zero asks once.
	Resolving time.Duration

	// Scoped reports that the server reads only the files that it has open and what they load,
	// as typescript-language-server does for a file that no tsconfig.json or jsconfig.json
	// includes, and tsc --lsp does for a project of a tsconfig.json none of whose files is open.
	// Before it plans a rename or a move, and before it relates the uses of a declaration that
	// other files can use, the engine opens the files of the workspace that write the name, so
	// the server finds the uses in them. For a rename and a relation, a file writes the name on a
	// line that does not start with //, which starts a line comment of JavaScript and TypeScript,
	// the languages of a scoped server.
	Scoped bool

	// Quiet reports that the server publishes no report for a change after which a kind of the
	// diagnostics of a file is still empty, as typescript-language-server 6.0.0 does, so a
	// change to clean content gets no report. The engine replaces a buffer of such a server
	// with textDocument/didClose and textDocument/didOpen, and the server publishes a report of
	// each kind for the file that it opens.
	Quiet bool

	// Tsserver reports that the server forwards the requests of tsserver in the command
	// typescript.tsserverRequest of workspace/executeCommand, as typescript-language-server
	// does. For a [Server.Scoped] server, [Engine.preload] takes these steps:
	//
	//  1. Open one file of each project of a tsconfig.json or a jsconfig.json, so that tsserver
	//     loads the project.
	//  2. Request the project of every other file from tsserver with projectInfo.
	//  3. Open the files that no loaded project contains and that can refer to the declaration.
	//
	// A file of no project can refer to a declaration of a module only when its imports lead to
	// the module or to a loaded project.
	Tsserver bool

	// Related reports that the server returns, among the references of a declaration, the
	// references of the declarations that it redeclares. typescript-language-server returns the
	// uses of getCurrentDirectory of every interface that an interface extends among the uses
	// of the getCurrentDirectory that the interface declares. The engine requests the
	// definition at each site of a use and of a call, and keeps a site whose definitions include
	// the declaration, and a site without a definition.
	Related bool

	// Contextual reports that the server returns, among the implementations of a declaration,
	// the expressions whose contextual type is the declaration or a subtype of it, and the
	// interfaces that extend it. tsserver returns the array literal of `const nodes:
	// Statement[] = []` among the implementations of Node, and it returns 479 sites for Node in
	// the TypeScript repository. The engine keeps an implementation at the name of a
	// declaration that can implement, such as a class, and leaves out the others.
	Contextual bool

	// Imports reports that the definition at the end of the name of an import returns the file
	// that the import imports, as the definition at "./store" in import { Store } from "./store"
	// returns store.ts. The engine then serves [sema.ImportedBy] by the rule of
	// [Engine.importers]. csharp-ls returns one declaration of the namespace of a using
	// directive, which other files declare as well, so a server of C# does not set it. The type
	// checker of Go relates the importers of a package by its import path, so gopls does not set
	// it either.
	Imports bool
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

// Indentation names the sections of workspace/configuration under which a server requests the
// indentation of the file that an item scopes. An editor returns the indentation of the file
// that it shows under them. An empty field does not name a section.
type Indentation struct {
	// Options is the section of the formatting options of LSP 3.17 for the file, tabSize and
	// insertSpaces, which typescript-language-server requests under formattingOptions.
	Options string

	// Size is the section of the width of one level of indentation, and Spaces the section of
	// whether the file indents with spaces, which jdtls requests under java.format.tabSize and
	// java.format.insertSpaces.
	Size, Spaces string
}

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
