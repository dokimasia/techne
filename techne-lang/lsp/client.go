// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// nothing is the JSON null that the client returns for a configuration section without
// settings. An empty object declares the section with no settings, which a server reads as
// every option off.
const nothing = "null"

// The reasons that ApplyEdit returns with applied set to false.
const (
	declined  = "techne applies edits through its own gate"
	collected = "techne took the edit and applies it through its own gate"
)

// answers implements [protocol.Client]: the requests and notifications that a server sends to
// techne.
//
// A server waits for the reply to each of its requests, and some servers do not finish
// starting without one. [protocol.UnimplementedClient] refuses every request with an error.
// answers replies to the requests that a server sends during startup and during the roles of
// this package.
type answers struct {
	protocol.UnimplementedClient

	root     string
	settings map[string]any

	reports  *reports
	working  *working
	offering *asking
}

// RegisterCapability accepts a registration and does not store it, because the roles read the
// capabilities of initialize. A refusal would be an error response, which some servers treat
// as a failed start.
func (answers) RegisterCapability(context.Context, *protocol.RegistrationParams) error { return nil }

// UnregisterCapability accepts the removal of a registration.
func (answers) UnregisterCapability(context.Context, *protocol.UnregistrationParams) error {
	return nil
}

// Configuration returns one value per item, in the order of the items, because a server
// matches the values to its items by position. The value of an item is the settings under its
// section, all settings for an item without a section, and null for a section without
// settings.
func (a answers) Configuration(
	_ context.Context,
	params *protocol.ConfigurationParams,
) ([]protocol.LSPAny, error) {
	out := make([]protocol.LSPAny, 0, len(params.Items))
	for _, item := range params.Items {
		out = append(out, a.setting(item.Section))
	}
	return out, nil
}

// setting returns the settings under section as JSON, or null.
func (a answers) setting(section *string) protocol.LSPAny {
	var value any = a.settings
	if section != nil && *section != "" {
		under, declared := a.settings[*section]
		if !declared {
			return protocol.LSPAny(nothing)
		}
		value = under
	}
	if value == nil {
		return protocol.LSPAny(nothing)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return protocol.LSPAny(nothing)
	}
	return protocol.LSPAny(raw)
}

// WorkspaceFolders returns the workspace root as the only folder, because a server that is
// told that no folder is open indexes nothing.
func (a answers) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	return []protocol.WorkspaceFolder{{URI: uri.File(a.root), Name: filepath.Base(a.root)}}, nil
}

// ApplyEdit returns applied false for every edit, because techne writes a change only through
// its write path, which previews, gates and applies it atomically. An edit that a server offers
// while techne performs a command for it becomes the result of the command, see [asking], and
// its reply gives the reason collected. Every other reply gives the reason declined.
func (a answers) ApplyEdit(
	_ context.Context,
	params *protocol.ApplyWorkspaceEditParams,
) (*protocol.ApplyWorkspaceEditResult, error) {
	reason := declined
	if params != nil && a.offering.offered(&params.Edit) {
		reason = collected
	}
	return &protocol.ApplyWorkspaceEditResult{Applied: false, FailureReason: &reason}, nil
}

// ShowDocument returns success false, because techne has no editor to show a document in.
func (answers) ShowDocument(
	context.Context,
	*protocol.ShowDocumentParams,
) (*protocol.ShowDocumentResult, error) {
	return &protocol.ShowDocumentResult{Success: false}, nil
}

// ShowMessageRequest returns null, the protocol's reply for a prompt that nobody chose an
// action for.
func (answers) ShowMessageRequest(
	context.Context,
	*protocol.ShowMessageRequestParams,
) (*protocol.MessageActionItem, error) {
	return nil, nil //nolint:nilnil // null is the reply for no action chosen
}

// Each refresh request returns nil, because techne does not cache the results of a server.
func (answers) CodeLensRefresh(context.Context) error       { return nil }
func (answers) FoldingRangeRefresh(context.Context) error   { return nil }
func (answers) SemanticTokensRefresh(context.Context) error { return nil }
func (answers) InlineValueRefresh(context.Context) error    { return nil }
func (answers) InlayHintRefresh(context.Context) error      { return nil }
func (answers) DiagnosticRefresh(context.Context) error     { return nil }

// TextDocumentContentRefresh returns nil, because techne does not cache document content.
func (answers) TextDocumentContentRefresh(
	context.Context,
	*protocol.TextDocumentContentRefreshParams,
) error {
	return nil
}

// asking keeps the edits that a server offers through workspace/applyEdit while techne
// performs a command for it. typescript-language-server performs every refactoring this way.
//
// The window is open for one command at a time, so two commands in flight cannot take each
// other's edits. An edit offered while the window is closed is refused. asking is safe for
// concurrent use.
type asking struct {
	// one is locked during one command.
	one sync.Mutex
	// mu guards open and kept, which the goroutine that reads the connection writes.
	mu   sync.Mutex
	open bool
	kept []*protocol.WorkspaceEdit
}

// arm opens the window and returns the function that closes it and returns the edits offered
// while it was open. arm blocks while the window is open for another command.
func (a *asking) arm() func() []*protocol.WorkspaceEdit {
	a.one.Lock()
	a.mu.Lock()
	a.open, a.kept = true, nil
	a.mu.Unlock()

	return func() []*protocol.WorkspaceEdit {
		a.mu.Lock()
		kept := a.kept
		a.open, a.kept = false, nil
		a.mu.Unlock()
		a.one.Unlock()
		return kept
	}
}

// offered keeps edit and reports true while the window is open, and reports false otherwise.
func (a *asking) offered(edit *protocol.WorkspaceEdit) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.open {
		return false
	}
	a.kept = append(a.kept, edit)
	return true
}

// The client callbacks that techne implements.
var _ protocol.Client = answers{}
