// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Engine answers about one language by asking its language server.
//
// # The server is started once and kept
//
// Starting is cheap and the first question is not: gopls answers
// initialise in 26ms and its first document symbol in 90ms, and every
// one after that in one or two. A server started per call would pay the
// first question's price every time, which is what makes a refactoring
// tool feel like a batch job rather than an editor.
//
// So the process is started on the first question and kept until
// [Engine.Close]. A caller that asks nothing starts nothing, which is
// what keeps a binary serving ten languages from starting ten servers to
// answer about one.
//
// # It claims what the server reaches, per role
//
// A server binds names through a type checker and outlines a file no
// better than a parser does, at a thousandth of the speed. Claiming
// resolved for every role would win the catalogue's sort and make an
// outline start a process to do worse. What it claims per role is the
// language module's declaration, not this engine's guess.
type Engine struct {
	declared lang.Declaration
	server   Server
	root     string

	// starting guards the one start. A second caller arriving while the
	// first is in the handshake waits for it rather than starting a
	// second server.
	starting sync.Mutex
	held     *session
	failed   error

	// opening serialises what the server is told about a file. Two
	// questions arriving together would otherwise open one file twice, or
	// send two edits under the same version.
	opening sync.Mutex
	// showing is held while the server is being shown a buffer that is
	// not on disk, which is how a refactoring computed over the result
	// of another one is asked for. A question arriving in that window
	// would otherwise put the file back underneath it.
	showing sync.Mutex
	// opened is what the server was last given, per absolute path.
	opened map[string]sent

	// pushed holds the diagnostics of a server that sends them unasked,
	// and is replaced with the session it belongs to: what the last
	// server said is not true of the next one.
	pushed *published

	// working is what this session's server has said it is still doing,
	// and is replaced with it for the same reason.
	working *working

	// offering is where an edit a server was asked to compute is kept,
	// and is replaced with the session for the same reason.
	offering *asking
}

// New returns an engine over one language's server, rooted at a
// directory.
//
// The root is a path on disk rather than an [io/fs.FS], because a
// language server is a process that opens files itself and cannot be
// handed a filesystem that is not one. An engine over a tree that is not
// on disk is refused here rather than left to fail on the first call.
func New(root string, d lang.Declaration, s Server) (*Engine, error) {
	switch {
	case d.Language == "":
		return nil, fmt.Errorf("lsp: declaration names no language")
	case root == "":
		return nil, fmt.Errorf("lsp: %q has no workspace root", d.Language)
	}
	if err := s.Valid(); err != nil {
		return nil, err
	}

	held, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("lsp: %q workspace root: %w", d.Language, err)
	}
	if info, err := os.Stat(held); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("lsp: %q workspace root %q is not a directory", d.Language, held)
	}
	return &Engine{declared: d, server: s, root: held, opened: map[string]sent{}}, nil
}

// sent is what the server was last given for one file.
//
// The digest is of the text it was given, not of the file: what decides
// whether the server is holding something stale is what it was told, and
// a file rewritten to its old content is not stale.
type sent struct {
	version int32
	digest  [sha256.Size]byte
}

// Name identifies this engine in a provenance and a capability report.
//
// The server's own name, because that is what a caller can act on: told
// gopls answered, it knows what to install, what to upgrade and what to
// read the release notes of.
func (e *Engine) Name() string { return e.server.Name }

// Language is the one language this engine answers about.
func (e *Engine) Language() source.Language { return e.declared.Language }

// Fidelity is what this server reaches for a role, as its language
// module declared.
func (e *Engine) Fidelity(role engine.Role) trust.Fidelity { return e.server.Reaches(role) }

// Cost is [engine.CostSession]: dear once and cheap after.
//
// Pricing it at what the first call costs would have a catalogue prefer
// a parser for every question, including the ones only a server can
// answer.
func (*Engine) Cost(engine.Role) engine.Cost { return engine.CostSession }

// Available reports whether the server can be run.
//
// Only whether it is installed. Whether it will start, and whether it
// will answer, are things a call finds out; what this answers is the
// question a caller can act on before making one.
func (e *Engine) Available(context.Context) error { return e.server.Installed() }

// Close stops the server.
//
// A composition root calls it. An engine that is never asked anything
// never started one, and closing it does nothing.
func (e *Engine) Close(ctx context.Context) error {
	e.starting.Lock()
	defer e.starting.Unlock()

	if e.held == nil {
		return nil
	}
	held := e.held
	e.held, e.failed = nil, nil
	e.pushed, e.working, e.offering = nil, nil, nil

	e.opening.Lock()
	e.opened = map[string]sent{}
	e.opening.Unlock()
	return held.stop(ctx)
}

// running returns the server, starting it on the first question.
//
// A start that failed is remembered. Retrying a server that is not
// installed, once per call, would turn one clear refusal into a stall.
func (e *Engine) running(ctx context.Context) (*session, error) {
	e.starting.Lock()
	defer e.starting.Unlock()

	if e.held != nil {
		e.current(ctx, e.held)
		return e.held, nil
	}
	if e.failed != nil {
		return nil, e.failed
	}
	if err := e.server.Installed(); err != nil {
		e.failed = err
		return nil, err
	}

	e.pushed, e.working, e.offering = newPublished(), newWorking(), &asking{}
	held, err := start(ctx, e.server, e.root, answers{
		root: e.root, settings: e.server.Settings,
		pushed: e.pushed, working: e.working, offering: e.offering,
	})
	if err != nil {
		e.failed = err
		return nil, err
	}
	if err := e.handshake(ctx, held); err != nil {
		_ = held.stop(ctx)
		e.failed = err
		return nil, err
	}

	e.held = held
	return held, nil
}

// current re-sends every file the server is holding that has moved on
// since it was given it.
//
// Refreshing the one file a question names is not enough. A server
// answers from every buffer it holds: a rename asks who uses a
// declaration, and the uses are in other files, one of which the last
// change rewrote. Driving two renames through one session produced a
// second rename that found no uses at all and rewrote the declaration
// alone, because the file holding the uses still said what it said
// before the first.
//
// Once per question rather than once per file. Nothing in this process
// writes to the workspace while a question is being answered, and doing
// it per file would cost a read of every open file for every file read.
func (e *Engine) current(ctx context.Context, held *session) {
	e.showing.Lock()
	defer e.showing.Unlock()

	e.opening.Lock()
	holding := slices.Collect(maps.Keys(e.opened))
	e.opening.Unlock()

	for _, full := range holding {
		content, err := os.ReadFile(full)
		if err != nil {
			e.closed(ctx, held, full)
			continue
		}
		_ = e.told(ctx, held, full, content)
	}
}

// closed tells the server a file it was holding has gone.
//
// A move takes one away. A server left holding the buffer keeps
// answering about a file that is not there and keeps reporting
// diagnostics against it, and every question after that re-reads a path
// nothing will ever read.
func (e *Engine) closed(ctx context.Context, held *session, full string) {
	e.opening.Lock()
	_, holding := e.opened[full]
	delete(e.opened, full)
	e.opening.Unlock()

	if !holding {
		return
	}
	_ = held.asks.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(full)},
	})
	_ = held.asks.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{
			URI: uri.File(full), Type: protocol.FileChangeTypeDeleted,
		}},
	})
}

// handshake is the exchange a server will not answer anything before.
//
// # What it does not ask for
//
// No position encoding is announced. A client that offers utf-8 may be
// taken up on it, and every span this package converts is converted from
// UTF-16, which is what the specification falls back to when nothing is
// negotiated. Asking for the faster encoding would silently move every
// column on a line holding anything outside ASCII.
func (e *Engine) handshake(ctx context.Context, held *session) error {
	root := uri.File(e.root)
	pid := int32(os.Getpid())
	yes := true

	params := &protocol.InitializeParams{
		ProcessID: &pid,
		// Deprecated in favour of workspace folders, and sent anyway.
		// The servers this engine is declared for are not one
		// generation: the ones that read only the root would be given a
		// workspace they cannot see, and answer about nothing.
		RootURI: &root, //nolint:staticcheck // a server older than the replacement still reads it
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				// A server that reports diagnostics unasked checks
				// whether the client can receive them before it sends
				// any. Undeclared, the gate reads a file nobody
				// analysed and cannot say whether it is clean.
				PublishDiagnostics: &protocol.PublishDiagnosticsClientCapabilities{
					RelatedInformation: &yes,
					VersionSupport:     &yes,
				},
				// Opening a document is how a server is told what to
				// analyse. A client that does not claim to synchronise
				// is one a server need not analyse anything for.
				Synchronization: &protocol.TextDocumentSyncClientCapabilities{
					DidSave: &yes,
				},
				DocumentSymbol: &protocol.DocumentSymbolClientCapabilities{
					// The flat shape is a list of names whose containers
					// are named by string, so two members called Get
					// cannot be told apart. Asking for the tree is what
					// makes an outline nest.
					HierarchicalDocumentSymbolSupport: &yes,
				},
				// A refactoring is a code action, and a client that
				// declares none of this is answered with bare commands:
				// no kind to select on, no data to resolve, and nothing
				// to tell a refactoring from a quick fix.
				CodeAction: &protocol.CodeActionClientCapabilities{
					CodeActionLiteralSupport: protocol.ClientCodeActionLiteralOptions{
						CodeActionKind: protocol.ClientCodeActionKindOptions{
							ValueSet: refactorings,
						},
					},
					DataSupport:     &yes,
					DisabledSupport: &yes,
					ResolveSupport: protocol.ClientCodeActionResolveOptions{
						// The edit, which most servers compute only when
						// asked for: working one out for every action in
						// a menu nobody opened is what they avoid.
						Properties: []string{"edit"},
					},
				},
			},
			Window: &protocol.WindowClientCapabilities{
				// Without this a server has no reason to report what it
				// is doing, and one still loading a workspace answers
				// every question with nothing while looking finished.
				WorkDoneProgress: &yes,
			},
			Workspace: &protocol.WorkspaceClientCapabilities{
				WorkspaceFolders: &yes,
				// Claimed because it is answered. A server told the
				// client cannot be asked falls back to whatever it
				// defaults to, which for the servers that read most of
				// their behaviour from configuration is a different
				// server.
				Configuration: &yes,
				// A server watches the workspace for files changing
				// outside its own buffers, and techne's write path is
				// one of the things that changes them.
				DidChangeWatchedFiles: &protocol.DidChangeWatchedFilesClientCapabilities{
					DynamicRegistration: &yes,
				},
				// A workspace edit may move, create and delete files as
				// well as rewrite them, and a server that was not told
				// the client can apply those sends only the rewrites.
				WorkspaceEdit: &protocol.WorkspaceEditClientCapabilities{
					DocumentChanges: &yes,
					ResourceOperations: []protocol.ResourceOperationKind{
						protocol.ResourceOperationKindCreate,
						protocol.ResourceOperationKindRename,
						protocol.ResourceOperationKindDelete,
					},
				},
				// Several servers advertise willRenameFiles only to a
				// client that said it sends one. jdtls and metals are
				// two: undeclared, both report no file operations at all
				// and moving a file is refused for a language whose
				// server does it.
				FileOperations: &protocol.FileOperationClientCapabilities{
					WillRename: &yes,
					DidRename:  &yes,
				},
			},
		},
	}
	params.WorkspaceFolders = protocol.NewNullable([]protocol.WorkspaceFolder{
		{URI: root, Name: filepath.Base(e.root)},
	})

	if len(e.server.Settings) > 0 {
		raw, err := json.Marshal(e.server.Settings)
		if err != nil {
			return fmt.Errorf("lsp: %s settings: %w", e.server.Name, err)
		}
		params.InitializationOptions = protocol.LSPAny(raw)
	}

	answered, err := held.asks.Initialize(ctx, params)
	if err != nil {
		return fmt.Errorf("lsp: %s: %w", e.server.Name, err)
	}
	if answered != nil {
		held.capable = answered.Capabilities
	}
	if err := held.asks.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		return err
	}

	// A server that loads a workspace announces it a moment after this,
	// not during it. Asked in that moment it answers with nothing, and
	// nothing looks exactly like a complete answer. So the first
	// question waits for the server to say whether it is busy.
	e.working.announce(ctx, announcing)
	return nil
}

// open tells the server about a file, and tells it again when the file
// has moved on since.
//
// A server answers about the buffer it was given rather than about the
// file, and holds that buffer for the life of the session. techne's own
// write path rewrites files under it: after a rename is applied the
// server is still holding what the file said before, and the next
// question is answered about code that is no longer there. The failure
// is silent — a second rename computed against the old text finds the
// uses the old text had, rewrites the declaration and leaves the rest,
// which is the outcome the whole write path exists to prevent.
//
// So the file is read every time and sent again when it differs. Reading
// it costs nothing beside the round trip that follows, and re-sending
// unchanged content is the version conflict that made this open once.
func (e *Engine) open(ctx context.Context, held *session, p source.Path) error {
	full := e.fullPath(p)
	content, err := os.ReadFile(full)
	if err != nil {
		return fmt.Errorf("lsp: read %s: %w", p, err)
	}
	return e.told(ctx, held, full, content)
}

// told tells the server what a file on disk holds, both as the buffer it
// is keeping and as a file that changed underneath it.
//
// Both, because they are different things to a server. A server that
// keeps its own model of the workspace checks it against the filesystem
// before it refactors and refuses while the two differ: jdtls answers a
// rename over a file techne's write path rewrote with "out of sync with
// file system", and updating the buffer does not settle it.
func (e *Engine) told(ctx context.Context, held *session, full string, content []byte) error {
	changed, err := e.sync(ctx, held, full, content)
	if err != nil || !changed {
		return err
	}
	return held.asks.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{
			URI: uri.File(full), Type: protocol.FileChangeTypeChanged,
		}},
	})
}

// sync tells the server what a file holds, whether or not that is what
// is on disk.
//
// An unsaved buffer is what an editor gives a server while someone is
// still typing, and it is how a refactoring computed over the result of
// another one is asked for: the extraction is not written yet, and the
// server has to see it to be able to rename what it made.
// It reports whether the server was holding something else, which is
// what tells a caller the file moved on rather than being seen for the
// first time.
func (e *Engine) sync(
	ctx context.Context,
	held *session,
	full string,
	content []byte,
) (bool, error) {
	e.opening.Lock()
	defer e.opening.Unlock()

	digest := sha256.Sum256(content)
	was, already := e.opened[full]
	switch {
	case already && was.digest == digest:
		return false, nil
	case already:
		// Whole-document synchronisation. A server that asked for
		// incremental sync accepts a full replacement too: the range is
		// what is optional, not the text.
		if err := held.asks.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
			TextDocument: protocol.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri.File(full)},
				Version:                was.version + 1,
			},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{
				&protocol.TextDocumentContentChangeWholeDocument{Text: string(content)},
			},
		}); err != nil {
			return false, err
		}
		e.opened[full] = sent{version: was.version + 1, digest: digest}
		return true, nil
	}

	if err := held.asks.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri.File(full),
			LanguageID: protocol.LanguageKind(e.server.LanguageID),
			Version:    1,
			Text:       string(content),
		},
	}); err != nil {
		return false, err
	}
	e.opened[full] = sent{version: 1, digest: digest}
	return false, nil
}

// fullPath is a path where it is on disk.
//
// A path outside the workspace keeps the absolute form [Engine.pathOf]
// gave it. Joining that onto the root builds a name with the root
// twice — an existing file reported as missing, which is what a server
// answering about a sibling module produces on every call.
func (e *Engine) fullPath(p source.Path) string {
	held := filepath.FromSlash(string(p))
	if filepath.IsAbs(held) {
		return held
	}
	return filepath.Join(e.root, held)
}

// pathOf is a URI as a path relative to the workspace, which is how
// every path techne reports is written.
//
// A URI naming something outside the workspace keeps its own path. A
// server answers about what it reads, and what it reads includes a
// standard library and a module cache; reported as a relative path those
// would climb out of the root and read as workspace files.
//
// Climbing out is a path segment of two dots. A file called ..config is
// a name that begins with them and is inside.
func (e *Engine) pathOf(held uri.URI) source.Path {
	full := held.FsPath()
	if full == "" {
		return source.Path(held)
	}
	relative, err := filepath.Rel(e.root, full)
	if err != nil || outside(source.Path(filepath.ToSlash(relative))) {
		return source.Path(filepath.ToSlash(full))
	}
	return source.Path(filepath.ToSlash(relative))
}

// assert the engine claims what its package comment says it does, and
// serves every role it has a request behind.
//
// A role is declined by lacking a method rather than by returning an
// error, so this list is the whole of what a catalogue can select this
// engine for. [engine.Checker] and [engine.Indexer] are absent on
// purpose: gating content the workspace does not hold has no request in
// the protocol, and an index of a server's answers would be a second
// copy of what the server already keeps and invalidates better.
var (
	_ engine.Engine    = (*Engine)(nil)
	_ engine.Available = (*Engine)(nil)
	_ engine.Outliner  = (*Engine)(nil)
	_ engine.Searcher  = (*Engine)(nil)
	_ engine.Resolver  = (*Engine)(nil)
	_ engine.Relator   = (*Engine)(nil)
	_ engine.Planner   = (*Engine)(nil)
	_ engine.Formatter = (*Engine)(nil)
	_ engine.Verifier  = (*Engine)(nil)
)
