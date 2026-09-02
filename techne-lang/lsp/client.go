// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"encoding/json"
	"path/filepath"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// nothing is the protocol's way of saying a setting is not set.
//
// Different from an empty object, which says the setting exists and is
// blank. For an option that defaults to on, the two configure different
// servers.
const nothing = "null"

// declined is why techne never lets a server write.
const declined = "techne applies edits through its own gate"

// answers is what techne says when a server asks it something.
//
// A language server is not only asked questions. It registers
// capabilities, asks what it is configured with, asks which folders are
// open, and offers to apply edits — and every one of those is a request
// whose sender waits for a reply. A server left waiting stops serving,
// which in a tool whose job includes reporting that it found nothing is
// the hardest failure to notice.
//
// [protocol.UnimplementedClient] refuses every request it was not given
// an answer for. That is the right default and the wrong answer for the
// few a server will not finish starting without, which are answered
// here.
type answers struct {
	protocol.UnimplementedClient

	root     string
	settings map[string]any

	// pushed is where diagnostics a server sends unasked are kept. A
	// server that has no pull request is not a server with no
	// diagnostics, and dropping what it sends would report every such
	// language as clean.
	pushed *published
}

// PublishDiagnostics keeps what a server reported about a file.
//
// It replaces rather than accumulates, because a publish is the whole of
// what the server currently says about that file: a file that was fixed
// is republished with an empty list, and appending would report the
// problem forever.
func (a answers) PublishDiagnostics(
	_ context.Context,
	params *protocol.PublishDiagnosticsParams,
) error {
	a.pushed.keep(params.URI, params.Diagnostics)
	return nil
}

// RegisterCapability accepts what a server registers, and stores none of
// it.
//
// A server registers to be told about things a client watches: files
// changing on disk, configuration changing. techne re-reads what it
// needs when it asks, so there is nothing to hold — but refusing the
// registration is an error response, and a server that treats one as
// fatal never finishes starting.
func (answers) RegisterCapability(context.Context, *protocol.RegistrationParams) error { return nil }

// UnregisterCapability accepts the withdrawal of what was never stored.
func (answers) UnregisterCapability(context.Context, *protocol.UnregistrationParams) error {
	return nil
}

// Configuration answers with what the language module declared.
//
// One answer per item and in the item's own position, because a server
// matches them by index rather than by section. A section nothing was
// declared for is answered null.
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

// setting is the declared settings under one section.
//
// A server asking for no section is handed the whole declaration, which
// is the same thing it was given at initialise.
func (a answers) setting(section *string) protocol.LSPAny {
	held := any(a.settings)
	if section != nil && *section != "" {
		under, declared := a.settings[*section]
		if !declared {
			return protocol.LSPAny(nothing)
		}
		held = under
	}
	if held == nil {
		return protocol.LSPAny(nothing)
	}

	raw, err := json.Marshal(held)
	if err != nil {
		return protocol.LSPAny(nothing)
	}
	return protocol.LSPAny(raw)
}

// WorkspaceFolders is the one root this engine was built over.
//
// One folder, not none. A server told there are no folders open indexes
// nothing and answers every workspace-wide question with an empty list,
// which reads exactly like a correct answer.
func (a answers) WorkspaceFolders(context.Context) ([]protocol.WorkspaceFolder, error) {
	return []protocol.WorkspaceFolder{{
		URI:  uri.File(a.root),
		Name: filepath.Base(a.root),
	}}, nil
}

// WorkDoneProgressCreate accepts a progress token.
//
// The progress reported against it is a notification this client drops.
// Refusing the token instead makes a server that reports progress on
// every request fail every request.
func (answers) WorkDoneProgressCreate(
	context.Context,
	*protocol.WorkDoneProgressCreateParams,
) error {
	return nil
}

// ApplyEdit refuses, always.
//
// A server offering to write is offering to write behind techne, which
// plans a change, gates it and applies it atomically. An edit that
// arrives this way is subject to none of that: nothing previewed it,
// nothing checked it, and a caller told a change was refused would find
// it on disk anyway.
//
// Refused as an answer rather than as an error, because the server asked
// a legitimate question and is entitled to know it was told no.
func (answers) ApplyEdit(
	_ context.Context,
	_ *protocol.ApplyWorkspaceEditParams,
) (*protocol.ApplyWorkspaceEditResult, error) {
	reason := declined
	return &protocol.ApplyWorkspaceEditResult{Applied: false, FailureReason: &reason}, nil
}

// ShowDocument reports that nothing was shown. There is no editor here
// and no window to open one in.
func (answers) ShowDocument(
	context.Context,
	*protocol.ShowDocumentParams,
) (*protocol.ShowDocumentResult, error) {
	return &protocol.ShowDocumentResult{Success: false}, nil
}

// ShowMessageRequest chooses none of the actions offered.
//
// Nothing is reading the message and nobody is deciding. Null is the
// protocol's answer for a dismissed prompt, and is what a server
// blocking on one needs to carry on.
func (answers) ShowMessageRequest(
	context.Context,
	*protocol.ShowMessageRequestParams,
) (*protocol.MessageActionItem, error) {
	return nil, nil //nolint:nilnil // null is the protocol's "no action chosen"
}

// The refresh requests below ask a client to throw away what it cached
// of an answer. This one caches none: every question re-asks. Accepted
// rather than refused, because a server that refreshes on every edit
// would otherwise collect an error per edit.
func (answers) CodeLensRefresh(context.Context) error       { return nil }
func (answers) FoldingRangeRefresh(context.Context) error   { return nil }
func (answers) SemanticTokensRefresh(context.Context) error { return nil }
func (answers) InlineValueRefresh(context.Context) error    { return nil }
func (answers) InlayHintRefresh(context.Context) error      { return nil }
func (answers) DiagnosticRefresh(context.Context) error     { return nil }

func (answers) TextDocumentContentRefresh(
	context.Context,
	*protocol.TextDocumentContentRefreshParams,
) error {
	return nil
}

// assert this answers everything a server may ask.
var _ protocol.Client = answers{}
