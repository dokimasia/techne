// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Engine serves the questions about one language with the language server that the language
// module declares.
//
// The server starts on the first question and runs until [Engine.Close]. An engine that is
// never asked a question starts no process. A running server returns a result in milliseconds:
// gopls returns textDocument/documentSymbol in 1 to 2ms after the first request.
//
// Engine is safe for concurrent use. [Engine.Close] may run while a question is in flight, and
// the question then returns the error of the closed connection.
type Engine struct {
	declared lang.Declaration
	server   Server
	// root is the absolute workspace root with symbolic links resolved. given is the absolute
	// root as the caller named it. A server path under either root maps into the workspace.
	root, given string

	// starting guards held and failed. A question that arrives during the handshake waits
	// for it and does not start a second server.
	starting sync.Mutex
	held     *session
	failed   error

	// showing is locked while a buffer of the server differs from its file on disk. A new
	// question waits for it, because the refresh of every buffer would replace the content the
	// server was shown.
	showing sync.Mutex
}

// New returns an engine over the workspace at root for the language that d declares, served
// by s.
//
// It returns an error when d names no language, when s is not valid, and when root is not a
// directory on disk. A language server opens files itself, so the workspace cannot be an
// [io/fs.FS].
func New(root string, d lang.Declaration, s Server) (*Engine, error) {
	switch {
	case d.Language == "":
		return nil, errors.New("lsp: the declaration names no language")
	case root == "":
		return nil, fmt.Errorf("lsp: %s: the workspace root is empty", d.Language)
	}
	if err := s.Valid(); err != nil {
		return nil, err
	}
	given, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: workspace root: %w", d.Language, err)
	}
	resolved, err := filepath.EvalSymlinks(given)
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: workspace root: %w", d.Language, err)
	}
	if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("lsp: %s: workspace root %s is not a directory", d.Language, given)
	}
	return &Engine{declared: d, server: s, root: resolved, given: given}, nil
}

// Name returns the name of the server, such as gopls, which names the program to install or
// upgrade.
func (e *Engine) Name() string { return e.server.Name }

// Language returns the language of the engine.
func (e *Engine) Language() source.Language { return e.declared.Language }

// Fidelity returns the tier that the language module declared for role, and [trust.None] for
// a role it did not declare.
func (e *Engine) Fidelity(role engine.Role) trust.Fidelity { return e.server.Fidelity(role) }

// Cost returns [engine.CostSession] for every role: the first question includes the start of
// the server, and every later question uses the running server.
func (*Engine) Cost(engine.Role) engine.Cost { return engine.CostSession }

// Available returns the error of [Server.Installed]: nil when the command of the server is on
// PATH, and the reason otherwise. It does not start the server.
func (e *Engine) Available(context.Context) error { return e.server.Installed() }

// Close stops the server and returns the error of its shutdown. It returns nil for an engine
// without a running server. The next question after Close starts a new server, including after
// a start that failed. Close is safe to call concurrently with a question and with itself.
func (e *Engine) Close(ctx context.Context) error {
	e.starting.Lock()
	held := e.held
	e.held, e.failed = nil, nil
	e.starting.Unlock()

	if held == nil {
		return nil
	}
	return held.stop(ctx)
}

// running returns the running server, and starts it for the first question.
//
// The failure of a start or a handshake is kept and returned to every later question until
// [Engine.Close], so a missing server costs one attempt. A failure caused by the caller's own
// context is not kept. The error of a handshake that fails includes the end of the server's
// stderr.
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

	held, err := start(ctx, e.server, e.root)
	if err != nil {
		e.failed = err
		return nil, err
	}
	handshaking, done := context.WithTimeout(ctx, starting)
	defer done()
	if err := e.handshake(handshaking, held); err != nil {
		// The server did not finish initialize, so it has nothing to write out. stop kills
		// it at once under a context that is already done, and waits until its stderr is
		// read to the end.
		now, kill := context.WithCancel(context.WithoutCancel(ctx))
		kill()
		_ = held.stop(now)

		if ctx.Err() != nil {
			return nil, fmt.Errorf("lsp: %s: initialize: %w", e.server.Name, ctx.Err())
		}
		if errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("lsp: %s: no answer to initialize within %s", e.server.Name, starting)
		}
		e.failed = held.withStderr(err)
		return nil, e.failed
	}
	e.held = held
	return held, nil
}

// handshake sends initialize and initialized, and waits up to [announcing] for the server to
// report a progress job.
//
// The client capabilities declare the requests and notifications that the roles of this
// package use. No position encoding is declared, so every position is counted in UTF-16 code
// units, the LSP 3.17 default that [document] converts.
func (e *Engine) handshake(ctx context.Context, held *session) error {
	root := uri.File(e.root)
	pid := int32(os.Getpid())
	yes := true

	params := &protocol.InitializeParams{
		ProcessID: &pid,
		// Deprecated by workspace folders. jdtls and metals still read it.
		RootURI: &root, //nolint:staticcheck // servers of both generations are declared
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				// A server that publishes diagnostics checks this before it sends any.
				PublishDiagnostics: &protocol.PublishDiagnosticsClientCapabilities{
					RelatedInformation: &yes,
					VersionSupport:     &yes,
				},
				Synchronization: &protocol.TextDocumentSyncClientCapabilities{
					DidSave: &yes,
				},
				// A tree nests members under their type. The flat form names a container
				// by name only.
				DocumentSymbol: &protocol.DocumentSymbolClientCapabilities{
					HierarchicalDocumentSymbolSupport: &yes,
				},
				// Code action literals contain a kind, data to resolve and a disabled reason.
				// A client without them receives bare commands.
				CodeAction: &protocol.CodeActionClientCapabilities{
					CodeActionLiteralSupport: protocol.ClientCodeActionLiteralOptions{
						CodeActionKind: protocol.ClientCodeActionKindOptions{
							ValueSet: refactorings,
						},
					},
					DataSupport:     &yes,
					DisabledSupport: &yes,
					ResolveSupport: protocol.ClientCodeActionResolveOptions{
						Properties: []string{"edit"},
					},
				},
			},
			// A server reports the load of a workspace only to a client that declares
			// work-done progress.
			Window: &protocol.WindowClientCapabilities{
				WorkDoneProgress: &yes,
			},
			Workspace: &protocol.WorkspaceClientCapabilities{
				WorkspaceFolders: &yes,
				// The client returns the settings of the declaration for
				// workspace/configuration.
				Configuration: &yes,
				DidChangeWatchedFiles: &protocol.DidChangeWatchedFilesClientCapabilities{
					DynamicRegistration: &yes,
				},
				// A workspace edit may create, rename and delete files. A server sends
				// those operations only to a client that declares them.
				WorkspaceEdit: &protocol.WorkspaceEditClientCapabilities{
					DocumentChanges: &yes,
					ResourceOperations: []protocol.ResourceOperationKind{
						protocol.ResourceOperationKindCreate,
						protocol.ResourceOperationKindRename,
						protocol.ResourceOperationKindDelete,
					},
				},
				// jdtls and metals advertise workspace/willRenameFiles only to a client
				// that declares it.
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
			return fmt.Errorf("lsp: %s: settings: %w", e.server.Name, err)
		}
		params.InitializationOptions = protocol.LSPAny(raw)
	}

	answered, err := held.asks.Initialize(ctx, params)
	if err != nil {
		return fmt.Errorf("lsp: %s: initialize: %w", e.server.Name, err)
	}
	if answered != nil {
		held.capable = answered.Capabilities
	}
	if err := held.asks.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		return fmt.Errorf("lsp: %s: initialized: %w", e.server.Name, err)
	}
	held.working.announce(ctx, announcing)
	return nil
}

// stamp is the size and modification time of a file on disk. The zero stamp belongs to
// content that is not on disk.
type stamp struct {
	size     int64
	modified time.Time
}

// stampOf returns the stamp of info.
func stampOf(info os.FileInfo) stamp {
	return stamp{size: info.Size(), modified: info.ModTime()}
}

// sent describes the buffer of the server for one file: the version it was sent under, the
// SHA-256 digest of its content, and the stamp of the file it was read from.
type sent struct {
	version int32
	digest  [sha256.Size]byte
	stamp   stamp
}

// snapshot reads the file at full and returns its content with the stamp of the open file.
func snapshot(full string) ([]byte, stamp, error) {
	file, err := os.Open(full)
	if err != nil {
		return nil, stamp{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, stamp{}, err
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, stamp{}, err
	}
	return content, stampOf(info), nil
}

// current sends the server the content of every file whose buffer is stale, and releases
// every buffer whose file is gone. It reads a file only when its stamp differs from the stamp
// of its buffer.
func (e *Engine) current(ctx context.Context, held *session) {
	e.showing.Lock()
	defer e.showing.Unlock()

	for full, buffer := range held.buffers() {
		info, err := os.Stat(full)
		if err != nil {
			e.release(ctx, held, full)
			continue
		}
		if stampOf(info) == buffer.stamp {
			continue
		}
		content, stamped, err := snapshot(full)
		if err != nil {
			e.release(ctx, held, full)
			continue
		}
		_ = e.told(ctx, held, full, content, stamped)
	}
}

// release tells the server that the file at full is gone: textDocument/didClose for its
// buffer and workspace/didChangeWatchedFiles for the file. It does nothing for a file without
// a buffer.
func (*Engine) release(ctx context.Context, held *session, full string) {
	held.opening.Lock()
	_, open := held.opened[full]
	delete(held.opened, full)
	held.opening.Unlock()

	if !open {
		return
	}
	_ = held.asks.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(full)},
	})
	_ = held.asks.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: uri.File(full), Type: protocol.FileChangeTypeDeleted}},
	})
}

// open reads the file at p, sends it to the server when the buffer of the server differs, and
// returns it. It returns the error of [lang.Readable] for a file that no engine reads.
func (e *Engine) open(ctx context.Context, held *session, p source.Path) (document, error) {
	if err := e.readable(p); err != nil {
		return document{}, err
	}
	return e.load(ctx, held, p)
}

// load is [Engine.open] for a path from [Engine.walk], which [lang.Walk] has checked.
func (e *Engine) load(ctx context.Context, held *session, p source.Path) (document, error) {
	full := e.fullPath(p)
	content, stamped, err := snapshot(full)
	if err != nil {
		return document{}, fmt.Errorf("lsp: read %s: %w", p, err)
	}
	if err := e.told(ctx, held, full, content, stamped); err != nil {
		return document{}, err
	}
	return texted(p, content), nil
}

// read returns the file at p without sending it to the server. It returns the error of
// [lang.Readable] for a file that no engine reads.
func (e *Engine) read(p source.Path) (document, error) {
	if err := e.readable(p); err != nil {
		return document{}, err
	}
	content, err := os.ReadFile(e.fullPath(p))
	if err != nil {
		return document{}, fmt.Errorf("lsp: read %s: %w", p, err)
	}
	return texted(p, content), nil
}

// told sends the server the content of a file on disk as its buffer. When the buffer changes,
// told also sends workspace/didChangeWatchedFiles, because jdtls refuses a rename while its
// model of the disk differs from the disk and a changed buffer does not update that model.
func (e *Engine) told(ctx context.Context, held *session, full string, content []byte, at stamp) error {
	changed, err := e.sync(ctx, held, full, content, at)
	if err != nil || !changed {
		return err
	}
	return held.asks.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: uri.File(full), Type: protocol.FileChangeTypeChanged}},
	})
}

// sync makes content the buffer of the server for the file at full, and reports whether it
// replaced a different buffer. It sends textDocument/didOpen for a file without a
// buffer, textDocument/didChange with the whole content for a buffer with other content, and
// nothing for a buffer with the same content. A replaced buffer drops the diagnostics the
// server published for it. The stamp is the zero stamp for content that is not on disk.
func (e *Engine) sync(
	ctx context.Context,
	held *session,
	full string,
	content []byte,
	at stamp,
) (bool, error) {
	held.opening.Lock()
	defer held.opening.Unlock()

	digest := sha256.Sum256(content)
	was, open := held.opened[full]
	switch {
	case open && was.digest == digest:
		was.stamp = at
		held.opened[full] = was
		return false, nil
	case open:
		if err := held.asks.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
			TextDocument: protocol.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri.File(full)},
				Version:                was.version + 1,
			},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{
				&protocol.TextDocumentContentChangeWholeDocument{Text: string(content)},
			},
		}); err != nil {
			return false, fmt.Errorf("lsp: %s: didChange %s: %w", e.server.Name, full, err)
		}
		held.reports.forget(uri.File(full))
		held.working.touched()
		held.opened[full] = sent{version: was.version + 1, digest: digest, stamp: at}
		return true, nil
	}

	if err := held.asks.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri.File(full),
			LanguageID: protocol.LanguageKind(e.server.Named(full)),
			Version:    1,
			Text:       string(content),
		},
	}); err != nil {
		return false, fmt.Errorf("lsp: %s: didOpen %s: %w", e.server.Name, full, err)
	}
	held.working.touched()
	held.opened[full] = sent{version: 1, digest: digest, stamp: at}
	return false, nil
}

// restore sends the server the content on disk of each path after a question showed it other
// content, and releases the buffer of a path that has no file. It uses a context that ctx
// does not cancel, so a cancelled question does not leave the shown content in the server.
func (e *Engine) restore(ctx context.Context, held *session, paths []source.Path) {
	back := context.WithoutCancel(ctx)
	for _, p := range paths {
		if _, err := e.load(back, held, p); err != nil {
			e.release(back, held, e.fullPath(p))
		}
	}
}

// walk returns the files of the language in the scope of req, from [lang.Walk].
func (e *Engine) walk(req engine.Request) (lang.Files, error) {
	return lang.Walk(os.DirFS(e.root), req.Scope, e.declared.Extensions)
}

// readable returns the error of [lang.Readable] for a path in the workspace. For an absolute
// path outside the workspace, such as a file in a module cache, it returns the error of
// [lang.Large] only, because no .gitignore file of the workspace applies there.
func (e *Engine) readable(p source.Path) error {
	if !filepath.IsAbs(filepath.FromSlash(string(p))) {
		return lang.Readable(os.DirFS(e.root), p)
	}
	info, err := os.Stat(e.fullPath(p))
	if err != nil {
		return fmt.Errorf("lsp: %s: %w", p, err)
	}
	return lang.Large(p, info.Size())
}

// fullPath returns the absolute path of p: p itself for an absolute path, and p under the
// root otherwise.
func (e *Engine) fullPath(p source.Path) string {
	native := filepath.FromSlash(string(p))
	if filepath.IsAbs(native) {
		return native
	}
	return filepath.Join(e.root, native)
}

// pathOf returns the workspace path of a file URI: slash-separated and relative to the root.
// A file under the resolved root and a file under the root as the caller gave it both map
// into the workspace. A file outside the workspace keeps its absolute path, and a URI that
// names no file is returned as a path unchanged.
func (e *Engine) pathOf(u uri.URI) source.Path {
	full := u.FsPath()
	if full == "" {
		return source.Path(u)
	}
	for _, root := range []string{e.root, e.given} {
		relative, err := filepath.Rel(root, full)
		if p := source.Path(filepath.ToSlash(relative)); err == nil && !outside(p) {
			return p
		}
	}
	return source.Path(filepath.ToSlash(full))
}

// The ports that Engine implements. The catalogue selects an engine for a role by its port,
// so this list is every role an Engine can serve. Outline, search and index are absent. The
// tree-sitter engine serves those roles for every language.
var (
	_ engine.Engine    = (*Engine)(nil)
	_ engine.Available = (*Engine)(nil)
	_ engine.Resolver  = (*Engine)(nil)
	_ engine.Relator   = (*Engine)(nil)
	_ engine.Planner   = (*Engine)(nil)
	_ engine.Formatter = (*Engine)(nil)
	_ engine.Checker   = (*Engine)(nil)
	_ engine.Verifier  = (*Engine)(nil)
)
