// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"fmt"

	"go.dokimi.dev/techne/core/engine"
	"go.lsp.dev/protocol"
)

// provides reports whether a provider field of the capabilities that a server returned from
// initialize offers its request: true for the value true or an options object, and false for
// an absent field or the value false.
//
// A role checks the capabilities before it sends a request, because a server refuses a request
// it does not serve with an error, and an error ends the whole call.
func provides(provider any) bool {
	switch declared := provider.(type) {
	case nil:
		return false
	case protocol.Boolean:
		return bool(declared)
	}
	return true
}

// prepares reports whether the rename provider offers textDocument/prepareRename.
func prepares(provider protocol.RenameProvider) bool {
	options, isOptions := provider.(*protocol.RenameOptions)
	return isOptions && options.PrepareProvider != nil && *options.PrepareProvider
}

// resolves reports whether the code action provider offers codeAction/resolve, which computes
// the edit of an action that a server offered without one.
func resolves(provider any) bool {
	options, isOptions := provider.(*protocol.CodeActionOptions)
	return isOptions && options.ResolveProvider != nil && *options.ResolveProvider
}

// willRename reports whether the server offers workspace/willRenameFiles for at least one
// filter. gopls 0.23.0 refuses the request with an error and offers no filter.
func willRename(capable protocol.ServerCapabilities) bool {
	return capable.Workspace != nil &&
		capable.Workspace.FileOperations != nil &&
		len(capable.Workspace.FileOperations.WillRename.Filters) > 0
}

// workspaceWide reports whether the diagnostic provider offers workspace/diagnostic. Of the ten
// servers that the language modules declare, csharp-ls offers it.
func workspaceWide(provider protocol.DiagnosticProvider) bool {
	switch declared := provider.(type) {
	case *protocol.DiagnosticOptions:
		return declared.WorkspaceDiagnostics
	case *protocol.DiagnosticRegistrationOptions:
		return declared.WorkspaceDiagnostics
	}
	return false
}

// unsupported returns an error that wraps [engine.ErrDecline] and names the request that the
// server does not serve, so the catalogue asks the next engine.
func (e *Engine) unsupported(request string) error {
	return fmt.Errorf("%w: %s does not serve %s", engine.ErrDecline, e.server.Name, request)
}
