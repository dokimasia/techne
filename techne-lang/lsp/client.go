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

// Why a server is told its edit was not applied. Both are true and they
// are not the same: one is an offer nobody asked for, and the other is
// the answer to a question techne asked, which it will gate and apply
// itself.
const (
	declined  = "techne applies edits through its own gate"
	collected = "techne took the edit and applies it through its own gate"
)

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

	// working is what the server has said it is still doing. A server
	// mid-load answers every question with nothing, and nothing reported
	// as a complete answer is a claim that there is nothing there.
	working *working

	// offering is where an edit techne asked a server to compute is
	// kept. Some servers expose a refactoring only as a command, and
	// answer it by offering the client the result to apply.
	offering *asking
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

// WorkDoneProgressCreate accepts a progress token, and counts the job it
// stands for as started.
//
// It arrives before the notification that begins the job, which is what
// makes it the useful signal: a question asked between the two would
// otherwise find the server idle and believe an answer it was still
// working on. Ending the job clears the token either way.
func (a answers) WorkDoneProgressCreate(
	_ context.Context,
	params *protocol.WorkDoneProgressCreateParams,
) error {
	a.working.began(tokened(params.Token))
	return nil
}

// ApplyEdit never applies anything, and keeps what was offered when
// techne asked for it.
//
// A server offering to write unprompted is offering to write behind
// techne, which plans a change, gates it and applies it atomically. An
// edit that arrives that way is subject to none of it: nothing
// previewed it, nothing checked it, and a caller told a change was
// refused would find it on disk anyway.
//
// One case is not that. A refactoring some servers expose only as a
// command is performed rather than described: the server computes the
// result and sends it here to be applied. techne asked for exactly that
// edit, so it is kept and becomes the plan — and still goes through the
// gate rather than onto disk.
//
// Either way the answer is that nothing was applied, which is true. The
// server asked a legitimate question and is entitled to know.
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
