// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"context"
	"fmt"
	"slices"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
)

// Check returns a finding for each place where content stops parsing as the
// language. A passing check means the files parse, not that they compile: a
// grammar accepts code that a compiler rejects, such as a Python file with
// an indentation error that CPython refuses.
//
// Check judges the files of its own language and ignores the others. It
// returns [engine.ErrDecline] when no file is of its language. A file with
// nil content is a deletion and has nothing to parse.
func (e *Engine) Check(
	ctx context.Context,
	files map[source.Path][]byte,
) (engine.Result[edit.Finding], error) {
	mine := make([]source.Path, 0, len(files))
	for p := range files {
		if lang.Claims(string(p), e.declared.Extensions) {
			mine = append(mine, p)
		}
	}
	if len(mine) == 0 {
		return engine.Result[edit.Finding]{}, fmt.Errorf(
			"%w: no file is %s", engine.ErrDecline, e.declared.Language)
	}
	slices.Sort(mine)

	var out []edit.Finding
	for _, p := range mine {
		if err := ctx.Err(); err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		if files[p] == nil {
			continue
		}
		found, err := e.broken(p, files[p])
		if err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		out = append(out, found...)
	}
	return engine.Result[edit.Finding]{Items: out, Completeness: trust.ScopeTotal}, nil
}

// broken parses one file with the grammar of its extension and returns a
// finding for each fault, at most faultLimit. A finding has no fix, because
// a file that stops parsing has no single change that repairs it.
func (e *Engine) broken(p source.Path, content []byte) ([]edit.Finding, error) {
	tree, _, err := e.parsed(p, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	root := tree.RootNode()
	if !root.HasError() {
		return nil, nil
	}
	walk := root.Walk()
	defer walk.Close()

	var out []edit.Finding
	for _, node := range faults(walk, root) {
		out = append(out, edit.Finding{Diagnostic: diag.Diagnostic{
			Severity: diag.SeverityError,
			Code:     "parse",
			Message:  fault(node, content),
			Span:     spanOf(p, *node),
			Source:   e.Name(),
			Snippet:  strings.TrimRight(lang.LineAt(content, int(node.StartByte())), " \t"),
		}})
		if len(out) == faultLimit {
			break
		}
	}
	return out, nil
}

// faults returns the error and missing nodes under from. It does not descend
// into an error node, because the nodes after the first fault are the
// grammar's recovery from it, not further faults.
func faults(walk *ts.TreeCursor, from *ts.Node) []*ts.Node {
	if from.IsError() || from.IsMissing() {
		return []*ts.Node{from}
	}
	var out []*ts.Node
	for _, child := range from.Children(walk) {
		if !child.HasError() && !child.IsError() && !child.IsMissing() {
			continue
		}
		inner := child.Walk()
		out = append(out, faults(inner, &child)...)
		inner.Close()
	}
	return out
}

// fault returns the message of one fault: for a missing node, the token the
// grammar expected, and for an error node, the first line of its text, cut
// by [lang.Clipped] to faultWidth bytes.
func fault(node *ts.Node, content []byte) string {
	if node.IsMissing() {
		return fmt.Sprintf("expected %s", node.Kind())
	}
	text := strings.TrimSpace(node.Utf8Text(content))
	if text == "" {
		return "this does not parse"
	}
	if i := strings.IndexAny(text, "\n\r"); i >= 0 {
		text = text[:i]
	}
	return fmt.Sprintf("unexpected %q", lang.Clipped(text, faultWidth))
}

const (
	// faultLimit is the number of faults one file reports at most. One fault
	// refuses a change.
	faultLimit = 8
	// faultWidth is the number of bytes of the faulty text a message quotes.
	faultWidth = 60
)

var _ engine.Checker = (*Engine)(nil)
