// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Check reports where content stopped parsing.
//
// It is the weakest gate that is worth running and the only one a
// grammar can serve: it says the file is still the language it was, and
// nothing about whether it means what it did. That is enough for a
// change that writes a comment, which fails by ending the comment early
// and turning prose into code, and it is not enough for a change that
// moves a name.
//
// What it does not catch is worth stating, because a gate is only worth
// what it refuses. A grammar is more forgiving than a compiler:
// tree-sitter-python accepts a file CPython rejects with an
// IndentationError, which was measured rather than assumed. A change
// that passes here is the language it claims to be and may still not
// compile.
//
// Files this language does not claim are left alone rather than refused.
// One change can touch several languages, and each engine judges its
// own.
func (e *Engine) Check(
	ctx context.Context,
	files map[source.Path][]byte,
) (engine.Result[edit.Finding], error) {
	mine := make([]source.Path, 0, len(files))
	for p := range files {
		if slices.Contains(e.declared.Extensions, path.Ext(string(p))) {
			mine = append(mine, p)
		}
	}
	if len(mine) == 0 {
		return engine.Result[edit.Finding]{}, fmt.Errorf(
			"%w: nothing here is %s", engine.ErrDecline, e.declared.Language)
	}
	slices.Sort(mine)

	parser := ts.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(e.grammar.Language); err != nil {
		return engine.Result[edit.Finding]{}, fmt.Errorf("treesitter: %w", err)
	}

	var out []edit.Finding
	for _, p := range mine {
		if err := ctx.Err(); err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		// A path with no content is one the change takes away. There is
		// nothing to parse and nothing to object to.
		if files[p] == nil {
			continue
		}
		found, err := e.broken(parser, p, files[p])
		if err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		out = append(out, found...)
	}

	return engine.Result[edit.Finding]{
		Items:        out,
		Completeness: trust.ScopeTotal,
	}, nil
}

// broken parses one file and reports where it stopped being the language
// it claims to be.
func (e *Engine) broken(parser *ts.Parser, p source.Path, content []byte) ([]edit.Finding, error) {
	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, fmt.Errorf("treesitter: %s: parser returned no tree", p)
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
		// No fix: a file that stopped parsing has no one obvious change
		// that resumes it, and guessing at one would write over what the
		// author meant.
		out = append(out, edit.Finding{Diagnostic: diag.Diagnostic{
			Severity: diag.SeverityError,
			Code:     "parse",
			Message:  fault(node, content),
			Span:     spanOf(p, *node),
			Source:   e.Name(),
			Snippet:  line(content, *node),
		}})
		if len(out) == faultLimit {
			break
		}
	}
	return out, nil
}

// faults collects the nodes a parse failed at.
//
// A node carrying an error is not descended into: what a grammar makes
// of the text after it has stopped matching is not a second fault, and
// reporting the tree beneath one broken line as twenty problems buries
// the one.
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

// fault says what is wrong in the words a reader can act on.
//
// A missing node names the token the grammar wanted and the text does
// not have. An error node has text, so the text is what is quoted: a
// caller reading "unexpected \"*/\"" knows where to look, and one
// reading "ERROR" does not.
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
	if len(text) > faultWidth {
		text = text[:faultWidth] + "…"
	}
	return fmt.Sprintf("unexpected %q", text)
}

// line returns the source line a node starts on, so a caller reading a
// fault reads the code as well as the complaint.
func line(content []byte, node ts.Node) string {
	start := int(node.StartByte())
	if start < 0 || start > len(content) {
		return ""
	}
	from := start
	for from > 0 && content[from-1] != '\n' {
		from--
	}
	to := start
	for to < len(content) && content[to] != '\n' {
		to++
	}
	return strings.TrimRight(string(content[from:to]), " \t\r")
}

const (
	// faultLimit caps how many parse faults one check reports. A gate
	// needs one to refuse, and a file that stopped parsing on line 3
	// produces a fault for most of what follows.
	faultLimit = 8
	// faultWidth is how much of the offending text is quoted.
	faultWidth = 60
)

// assert the engine serves the role it claims.
var _ engine.Checker = (*Engine)(nil)
