// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"fmt"

	"go.dokimi.dev/techne/core/engine"
	"go.lsp.dev/protocol"
)

// provides reports whether a server said at initialise that it answers a
// request.
//
// Every provider in the reply is the same shape: absent, a boolean, or
// an options object. Absent and false both mean no; an options object
// means yes and carries settings nothing here reads.
//
// Asked rather than found out by trying, because a server refuses an
// unsupported request with an error, and an error stops the whole call.
// pyright answers "unhandled method" to a request for implementations,
// which turned a question it simply does not answer into a broken read.
func provides(held any) bool {
	switch declared := held.(type) {
	case nil:
		return false
	case protocol.Boolean:
		return bool(declared)
	}
	return true
}

// prepares reports whether a server answers textDocument/prepareRename,
// which is a flag inside the rename provider rather than a provider of
// its own.
//
// A server that renames but does not prepare is common. Asking anyway
// refuses every rename it would have done, because an unsupported
// request comes back as an error and there is no way to tell that from
// the position being unrenameable.
func prepares(held protocol.RenameProvider) bool {
	options, declared := held.(*protocol.RenameOptions)
	return declared && options.PrepareProvider != nil && *options.PrepareProvider
}

// unsupported is the decline for a request a server said it does not
// answer.
//
// Declined rather than errored, so a parser beside it gets a turn and
// the caller is told which server would not and what it would not do.
func (e *Engine) unsupported(what string) error {
	return fmt.Errorf("%w: %s does not answer %s",
		engine.ErrDecline, e.server.Name, what)
}
