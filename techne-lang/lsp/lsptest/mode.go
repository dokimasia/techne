// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest

import "time"

// Mode selects the behaviour of the scripted server. The zero value is [Default].
type Mode string

// The modes of the scripted server. Each mode changes the responses that its documentation
// lists and keeps the [Default] response to every other request.
const (
	// Default responds to every request that package lsp sends with a fixed result about
	// [Content]. It responds to textDocument/diagnostic with three diagnostics: an error, a
	// warning with a numeric code, and one without a severity.
	Default Mode = ""

	// Silent starts and never responds to initialize.
	Silent Mode = "silent"

	// Dies writes [Dying] to stderr and exits with status 3 when it receives initialize.
	Dies Mode = "dies"

	// Empty declares nothing in any file and responds to textDocument/formatting with no edits.
	Empty Mode = "empty"

	// Unicode describes [Emoji]: it declares Störe on line 2 and responds to a definition with
	// the range of that name, counted in UTF-16 code units.
	Unicode Mode = "unicode"

	// Flat responds to textDocument/documentSymbol with SymbolInformation entries instead of a
	// tree, as a server without hierarchical document symbols does. The entry of Get names Store
	// as its container.
	Flat Mode = "flat"

	// OneLocation responds to textDocument/definition with one Location instead of a list.
	OneLocation Mode = "one-location"

	// Links responds to textDocument/definition with a list of LocationLink entries.
	Links Mode = "links"

	// Unresolved responds to textDocument/definition with null.
	Unresolved Mode = "unresolved"

	// Pointed responds to textDocument/definition with the position of the request, and
	// declares the package clause on line 0 of [Content] beside the [Default] declarations.
	Pointed Mode = "pointed"

	// Unnameable responds to textDocument/prepareRename with null.
	Unnameable Mode = "unnameable"

	// Strict describes [Emoji] as the Unicode mode does, and responds to
	// textDocument/prepareRename only for line 2, character 17: the UTF-16 position of Störe.
	Strict Mode = "strict"

	// Pushes advertises no pull diagnostics. It publishes the [Default] diagnostics for each
	// file the client opens, if the client declared textDocument.publishDiagnostics.
	Pushes Mode = "pushes"

	// PushesOne publishes one error for each opened file under a directory with the name two,
	// and an empty list for every other opened file.
	PushesOne Mode = "pushes-one"

	// Asks sends workspace/configuration, workspace/workspaceFolders and
	// workspace/applyEdit during initialize and waits for each reply. It responds to
	// textDocument/diagnostic with one diagnostic per reply, whose message is the request
	// name, an equals sign and the reply.
	Asks Mode = "asks"

	// Uncallable refuses textDocument/prepareCallHierarchy with an error, as a server does
	// for a declaration that cannot be called.
	Uncallable Mode = "uncallable"

	// Loading reports a work-done progress job for [LoadTime] after initialize, and responds
	// to textDocument/references with an empty list until the job ends.
	Loading Mode = "loading"

	// Stuck begins the Loading job and never ends it.
	Stuck Mode = "stuck"

	// Hangs never responds to textDocument/references, as metals did for 5 minutes while it
	// compiled a build.
	Hangs Mode = "hangs"

	// Ungated advertises no pull diagnostics, publishes none, and responds to
	// textDocument/implementation with an empty list. metals behaves this way before it has
	// imported a build.
	Ungated Mode = "ungated"

	// Thin advertises only textDocument/documentSymbol, and textDocument/rename without
	// prepareRename.
	Thin Mode = "thin"

	// Echoes responds to textDocument/diagnostic with three diagnostics about its buffers:
	//
	//   - "first=" and the first line of the buffer for the file.
	//   - "holding=" and the base names of all buffers, sorted and joined with commas.
	//   - "version=" and the version of the buffer for the file.
	Echoes Mode = "echoes"

	// Moveless advertises no workspace/willRenameFiles.
	Moveless Mode = "moveless"

	// SilentMove responds to workspace/willRenameFiles with an empty edit. It advertises no
	// pull diagnostics and publishes none, as metals does for a move.
	SilentMove Mode = "silent-move"

	// Extracts offers "Extract into variable" and "Extract into function" under the kind
	// refactor.extract, and computes the edit of each on codeAction/resolve. The edit appends
	// a function named [Placeholder]. It responds to textDocument/diagnostic with one
	// diagnostic per function in its buffer, whose message is "declares=" and the name.
	Extracts Mode = "extracts"

	// Commands offers the extraction of the Extracts mode as a command. It performs the
	// command by sending the edit to the client in workspace/applyEdit.
	// typescript-language-server offers every refactoring this way.
	Commands Mode = "commands"

	// Watches refuses textDocument/rename after the client changes a buffer, until the
	// client sends workspace/didChangeWatchedFiles. jdtls refuses a rename this way.
	Watches Mode = "watches"

	// Opened computes references from its buffers or else from disk, and renames only in its
	// buffers. It reports the first use of Store in b.fake, and renames that use only while it
	// has a buffer of b.fake. metals renames this way.
	Opened Mode = "opened"

	// Short reports the use of Store in b.fake as the Opened mode does, and never renames it.
	Short Mode = "short"

	// Scoped reports and renames the first use of Store in b.fake only while it has a buffer of
	// b.fake, and its answer to workspace/willRenameFiles renames that use as well.
	// typescript-language-server behaves this way for a file that no tsconfig.json includes.
	Scoped Mode = "scoped"

	// Conflicts refuses textDocument/rename with an error about a conflict.
	Conflicts Mode = "conflicts"

	// Unenclosed responds to textDocument/references with one site on line 0, outside every
	// declaration of [Content].
	Unenclosed Mode = "unenclosed"

	// Compiles responds to textDocument/diagnostic about its buffer for the file: one error
	// at each occurrence of [Broken]. It offers one quick fix for that error, which replaces
	// the word with "declared".
	Compiles Mode = "compiles"

	// Unbound diagnoses as the Compiles mode does, and responds to textDocument/definition
	// with null, as a server does for a name that an error leaves unbound.
	Unbound Mode = "unbound"

	// WorkspaceDiagnostics diagnoses as the Compiles mode does and advertises workspace
	// diagnostics. It also reports an error in each file that uses Store while no file
	// declares it. It diagnoses every file with the [Extension] suffix under the workspace
	// root, from its buffer for the file or else from disk.
	WorkspaceDiagnostics Mode = "workspace-diagnostics"

	// Canonical resolves symbolic links in every URI it receives, so every URI it reports
	// is the real path of a file.
	Canonical Mode = "canonical"

	// Receivers describes [Twins] as gopls does: the types Store and Cache, and the methods
	// (*Store).Get and (*Cache).Get at the top level. It responds to textDocument/definition
	// with the position of the request, and to textDocument/references with no location.
	Receivers Mode = "receivers"

	// Impls describes [Twins] as rust-analyzer describes impl blocks: the types Store and Cache,
	// and each method Get as the child of an Object symbol whose selection range is the name of
	// the type. The Object of the first Get is labelled impl Getter for Store<T>, and the Object
	// of the second impl Getter for &Cache. It responds to textDocument/definition with the
	// position of the request, and to textDocument/references with no location.
	Impls Mode = "impls"

	// Wrapped describes [Content] as csharp-ls and typescript-language-server nest symbols: a
	// File symbol around the file, a namespace Shop that contains Store, a constructor Store of
	// Store and the method Get, and an anonymous <function> that contains After. It responds to
	// textDocument/definition with the position of the request.
	Wrapped Mode = "wrapped"

	// Minified describes a [Bundle] from its buffer: a function for each func keyword, and the
	// declaration of F0 as the definition of every position. It reports each occurrence of F0
	// before a parenthesis in the files with the [Extension] suffix as a reference.
	Minified Mode = "minified"
)

// LoadTime is how long the Loading mode takes to end its progress job.
const LoadTime = 2 * time.Second

// Dying is the line that the Dies mode writes to stderr before it exits.
const Dying = "lsptest: the scripted server exits during initialize"
