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

	// Silent writes [Waiting] to stderr and never responds to initialize.
	Silent Mode = "silent"

	// Slow responds to initialize [SlowStart] after it receives the request, as ruby-lsp does
	// after it installs the gems of its composed bundle.
	Slow Mode = "slow"

	// Dies writes [Dying] to stderr and exits with status 3 when it receives initialize.
	Dies Mode = "dies"

	// DiesLate writes [Dying] to stderr when it receives initialize, and exits with status 3
	// [SlowStart] later.
	DiesLate Mode = "dies-late"

	// Orphans starts a child process that inherits its stderr and ends [OrphanTime] later, and
	// responds as the Default mode does. ruby-lsp starts bundle install this way, and the child
	// keeps the stderr of the server open after the server ends.
	Orphans Mode = "orphans"

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

	// Exports responds to textDocument/definition on line 0 of [Content] with the name Store,
	// and anywhere else with the start of line 0, which no declaration contains. So
	// typescript-language-server responds for an import with the export { X } of its module,
	// whose definition is the declaration of X.
	Exports Mode = "exports"

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

	// Quiet advertises no pull diagnostics and publishes an error at each occurrence of
	// [Broken], as typescript-language-server 6.0.0 publishes:
	//
	//   - The report of a document that the client opens follows [QuietDelay] after the open,
	//     and lists the diagnostics of the latest buffer.
	//   - The report of a change follows at once, and only when the last or the new report is
	//     not empty.
	//   - A document that the client closes gets an empty report [QuietClose] after the close,
	//     as a busy server publishes it, and before the next message is read. No report of a
	//     buffer that it held follows.
	Quiet Mode = "quiet"

	// Loads advertises no pull diagnostics and publishes the report of an open [QuietDelay]
	// after it, as the Quiet mode does, and
	// responds to textDocument/definition in a document with an empty list until it has sent
	// that report. So typescript-language-server responds while it loads the project of the
	// first file of the project that the client opens.
	Loads Mode = "loads"

	// Asks sends workspace/configuration, workspace/workspaceFolders and
	// workspace/applyEdit during initialize and waits for each reply. It then requests the
	// indentation of a.fake, of b.fake, of no file and of the file that [Outside] names, under
	// the sections of its [lsp.Server.Indentation]. It responds to
	// textDocument/diagnostic with one diagnostic per reply, whose message is the request
	// name, an equals sign and the reply.
	Asks Mode = "asks"

	// Cancels responds to textDocument/definition with the error RequestCancelled of LSP 3.17,
	// as metals does for a request that its build import cancels, and to
	// textDocument/references with the error ContentModified.
	Cancels Mode = "cancels"

	// Uncallable refuses textDocument/prepareCallHierarchy with an error, as a server does
	// for a declaration that cannot be called.
	Uncallable Mode = "uncallable"

	// Untyped refuses textDocument/implementation with an error response of the message
	// [NotAType], as gopls does for a constant.
	Untyped Mode = "untyped"

	// Loading reports a work-done progress job for [LoadTime] after initialize, and responds
	// to textDocument/references with an empty list until the job ends.
	Loading Mode = "loading"

	// Stuck begins the Loading job and never ends it.
	Stuck Mode = "stuck"

	// Created sends window/workDoneProgress/create for the Loading job during initialize, and
	// begins the job [CreateTime] later. It ends the job and responds to
	// textDocument/references as the Loading mode does.
	Created Mode = "created"

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
	// diagnostic per function in its buffer, whose message is "declares=" and the name. It
	// refuses a textDocument/rename to a name that is no identifier, as [InvalidName] states.
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

	// FromUse responds as the Default mode does, and at the name of Store on line 2 of [Content]
	// to textDocument/prepareRename and textDocument/rename with null and to
	// textDocument/references with no location. ruby-lsp responds this way at a constant that a
	// value assigns, and prepares a rename, renames and finds the references at each use of the
	// constant.
	FromUse Mode = "from-use"

	// Qualified reports and renames the use of Store in b.fake as the Opened mode does, and
	// reports the use over the qualifier before it and its name, such as a.Store. jdtls reports
	// the qualified name of a Javadoc link this way, and renames the name alone.
	Qualified Mode = "qualified"

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

	// DiskChecks responds to textDocument/diagnostic as the Compiles mode does, and checks the
	// files on disk as rust-analyzer runs cargo check: after initialized and after each
	// textDocument/didSave. A check waits [DiskDelay] and then reports a work-done progress job
	// with the token [DiskToken]. During the job it reads each file with the [Extension] suffix
	// under the workspace root from disk, and publishes two reports of the file. The interim
	// report has [DiskInterim] notes whose message is "interim". The report of the check has an
	// error at each occurrence of [Unsound] and a note whose message is "checks=" and the number
	// of checks so far.
	DiskChecks Mode = "disk-checks"

	// DiskStuck responds as the DiskChecks mode does, and begins a check on disk after
	// initialized that never ends.
	DiskStuck Mode = "disk-stuck"

	// Unbound diagnoses as the Compiles mode does, and responds to textDocument/definition
	// with null, as a server does for a name that an error leaves unbound.
	Unbound Mode = "unbound"

	// WorkspaceDiagnostics diagnoses as the Compiles mode does and advertises workspace
	// diagnostics. It also reports an error in each file that uses Store while no file
	// declares it, and the error [EmptyFile] in each empty file. It diagnoses every file with
	// the [Extension] suffix under the workspace root, from its buffer for the file or else
	// from disk.
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

	// Nested describes a [Bundle] as the Minified mode does, and reports each function as a
	// class of the same name and range that contains one method.
	Nested Mode = "nested"

	// Aims describes its buffer as gopls does, with a function for each func keyword and no
	// local or parameter, and responds to textDocument/rename with one edit that replaces the
	// word at the position of the request. It responds to textDocument/references with no
	// location. A test reads the edit of a rename to learn where the client aimed it.
	Aims Mode = "aims"

	// Mutes responds as the Default mode does until it receives textDocument/references or
	// textDocument/definition. From that request on it responds to every request with an empty
	// result: no symbols, no locations and no diagnostics. typescript-language-server responds this way after its
	// tsserver exits. A process that is not the first that [RecordStarts] records responds as
	// the Default mode does.
	Mutes Mode = "mutes"

	// Exits exits with status 0 when it receives textDocument/references, before it responds. A
	// process that is not the first that [RecordStarts] records responds as the Default mode
	// does.
	Exits Mode = "exits"

	// Uncalled describes a [Bundle] as the Minified mode does and advertises no call hierarchy,
	// as typescript-language-server advertises none. It reports each occurrence of the word F0
	// in the files with the [Extension] suffix as a reference, a call or not.
	Uncalled Mode = "uncalled"

	// Projects reports and renames uses as the Scoped mode does, and responds to the command
	// typescript.tsserverRequest of workspace/executeCommand with the request projectInfo as
	// tsserver does: with the tsconfig.json in the directory of the file or above it when the
	// client has a buffer of a file under the directory of that tsconfig.json, and with the
	// error No Project otherwise.
	Projects Mode = "projects"

	// Redeclares responds as the Default mode does, and to textDocument/definition on line 8 of
	// [Content] with the name After. tsserver responds this way at a use of a member of an
	// interface that another member redeclares.
	Redeclares Mode = "redeclares"

	// Contextual responds as the Default mode does, and to textDocument/implementation with the
	// name of Store, the use of Store inside After on line 8 of [Content], and the name of the
	// interface of [Getter] on line 9. tsserver responds this way with an expression whose type
	// implements an interface, and with an interface that extends it.
	Contextual Mode = "contextual"

	// Projected responds as the Default mode does, and to textDocument/implementation with the
	// name of each type that the document of the request declares, as tsserver responds with
	// the implementations in the project of the file of the request only. It responds to
	// textDocument/definition at a word Store on a line that starts with // imports with the
	// name of Store in a.fake. At a word Store on any other line it responds with the Store of
	// the first such line of the document, as tsserver responds at a use of an imported name
	// with the binding of the import, and with no location when the document has no such line.
	Projected Mode = "projected"
)

// LoadTime is how long the Loading mode takes to end its progress job.
const LoadTime = 2 * time.Second

// CreateTime is how long the Created mode waits between the create request of its job and the
// begin of the job. It is longer than the 500 ms for which the handshake waits for a first job
// and the 300 ms for which a question waits for the server to go quiet, so a client that does
// not take the create request as the begin of a job asks before the job begins.
const CreateTime = 1500 * time.Millisecond

// QuietDelay is how long the Quiet mode takes to publish the report of a document that the
// client opens, unless [Delaying] sets another delay. It is longer than the 300 ms for which a
// question waits for the server to go quiet, so a question that kept the report of the close
// reads that report.
const QuietDelay = 500 * time.Millisecond

// QuietClose is how long the Quiet mode takes to publish the empty report of a document that
// the client closes. A client that sends the next message without waiting for a reply after
// the close receives the report after that message.
const QuietClose = 200 * time.Millisecond

// DiskDelay is how long the DiskChecks mode waits before it begins a check on disk. It is
// longer than the 300 ms for which a question waits for the server to go quiet, so a question
// that does not wait for the check reads the report of the check before.
const DiskDelay = 400 * time.Millisecond

// DiskInterim is the number of notes of the interim report that the DiskChecks mode publishes
// for each file before the report of a check, as rust-analyzer publishes a file more than once
// in one check. A client that keeps the two reports out of the order of the stream keeps the
// interim report.
const DiskInterim = 2000

// DiskPrefix starts the progress token of each check on disk of the DiskChecks and DiskStuck
// modes, and DiskToken is the token.
const (
	DiskPrefix = "lsptest/disk/"
	DiskToken  = DiskPrefix + "0"
)

// Dying is the line that the Dies and DiesLate modes write to stderr before they exit.
const Dying = "lsptest: the scripted server exits during initialize"

// EmptyFile is the message of the error that the WorkspaceDiagnostics mode reports in an
// empty file, as a compiler reports a Go file without a package clause.
const EmptyFile = "the file is empty, and a file of this language starts with a comment"

// NotAType is the message of the error response of the Untyped mode to
// textDocument/implementation.
const NotAType = "Store is a const, not a type"

// InvalidName starts the message of the error response of the Extracts and Commands modes to a
// textDocument/rename whose new name is no identifier.
const InvalidName = "invalid identifier to rename"

// Waiting is the line that the Silent mode writes to stderr when it starts.
const Waiting = "lsptest: the scripted server waits before it responds to initialize"

// SlowStart is how long the Slow mode waits before it responds to initialize, and the DiesLate
// mode before it exits.
const SlowStart = 1500 * time.Millisecond

// OrphanTime is how long the child of the Orphans mode keeps the stderr of the scripted server
// open.
const OrphanTime = 10 * time.Second
