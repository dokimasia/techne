// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
)

// Plan returns the changes of an operation and writes nothing. The write path of techne
// seals, gates and applies the changes, as it does for every planner. Plan sends these
// requests of LSP 3.17:
//
//   - [edit.RenameSymbol]: textDocument/rename, after textDocument/prepareRename where the
//     server offers it.
//   - [edit.MoveFile]: workspace/willRenameFiles, the edits a move implies, followed by the
//     move itself.
//   - [edit.ExtractFunction]: a code action that extracts the lines, followed by a rename of
//     the function it adds.
//
// Plan returns [engine.ErrDecline] for every other operation, and for a server that does not
// answer within [Server.Answering]. For a target in a file of another language it returns a
// skipped result.
func (e *Engine) Plan(
	ctx context.Context,
	req engine.Request,
	op edit.Operation,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	var out engine.Result[edit.Change]
	var err error
	switch op {
	case edit.RenameSymbol:
		out, err = e.renaming(ctx, req, target, args)
	case edit.MoveFile:
		out, err = e.relocating(ctx, target, args)
	case edit.ExtractFunction:
		out, err = e.extracting(ctx, target, args)
	default:
		err = fmt.Errorf("%w: %s: no request plans %s", engine.ErrDecline, e.server.Name, op)
	}
	return out, e.unanswered(ctx, err)
}

// preloads is the number of files that [Engine.preload] opens for one plan.
const preloads = 200

// preload opens the files of the workspace that writes reports as writing what, for a
// [Server.Scoped] server, so that the server finds the uses in them. It skips the file at skip,
// which the plan has open. preload returns a caveat when more files write what than it opens,
// and nil otherwise.
func (e *Engine) preload(
	ctx context.Context,
	held *session,
	what string,
	writes func(p source.Path, content []byte) bool,
	skip source.Path,
) (*trust.Caveat, error) {
	if !e.server.Scoped || what == "" {
		return nil, nil
	}
	files, err := e.walk(engine.Request{Scope: engine.Root})
	if err != nil {
		return nil, err
	}
	var writers []source.Path
	for _, p := range files.Read {
		if p == skip {
			continue
		}
		content, err := os.ReadFile(e.fullPath(p))
		if err == nil && writes(p, content) {
			writers = append(writers, p)
		}
	}
	for _, p := range writers[:min(len(writers), preloads)] {
		if _, err := e.open(ctx, held, p); err != nil && !refused(err) {
			return nil, err
		}
	}
	if len(writers) <= preloads {
		return nil, nil
	}
	return &trust.Caveat{
		Code: trust.CaveatIndexWarming,
		Note: fmt.Sprintf("%s loads only the files it has open, and techne opened %d of the %d files "+
			"that write %s, so a use in the others may be missing", e.server.Name, preloads, len(writers), what),
	}, nil
}

// preloaded runs [Engine.preload] for the declaration whose name is at the protocol position at
// of doc, over the files that write the name. A declaration that its file does not offer to the
// rest of a program, such as a parameter or a local, has no use in another file, so preloaded
// opens no file for it.
func (e *Engine) preloaded(
	ctx context.Context,
	held *session,
	found *finder,
	doc document,
	at protocol.Position,
) (*trust.Caveat, error) {
	if !e.server.Scoped {
		return nil, nil
	}
	offered, err := found.offers(ctx, doc.path, at)
	if err != nil || !offered {
		return nil, err
	}
	word := doc.word(at)
	return e.preload(ctx, held, word, wording(word), doc.path)
}

// wording returns the rule of [Engine.preload] for a file that writes word as a word.
func wording(word string) func(source.Path, []byte) bool {
	return func(_ source.Path, content []byte) bool {
		return bytes.Contains(content, []byte(word)) && lang.Worded(string(content), word) >= 0
	}
}

// beyond returns the first path of changes that is outside the workspace, or the empty string
// when every path is inside. techne writes under its root only, so a plan with such a path
// cannot be applied as the server described it.
func beyond(changes []edit.Change) string {
	for _, one := range changes {
		for _, p := range []source.Path{one.Path, one.To} {
			if p != "" && filepath.IsAbs(filepath.FromSlash(string(p))) {
				return string(p)
			}
		}
	}
	return ""
}
