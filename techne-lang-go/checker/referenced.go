// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"unicode"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/lang"
)

// Referenced returns the changes of a rename of a Go declaration with the edits that rewrite a
// reference to it. It keeps these edits of a Go file:
//
//   - an edit outside the comments, which rewrites a name of the code
//   - the edit of the last name of a doc link, such as runs in [Pipeline.runs]
//   - the edit of the word that starts the doc comment of a declaration whose name the change
//     also renames, which the Go convention makes the name of the declaration
//
// It leaves out every other edit in a comment, which rewrites a word of its text: gopls renames
// each word of the doc comment of the renamed declaration that equals the old name.
//
// Referenced keeps a change as it is when the change is no edit of a Go file, or when the file
// cannot be read or does not parse. It leaves out a change when it leaves out each of its edits.
func (e *Engine) Referenced(changes []edit.Change) []edit.Change {
	out := make([]edit.Change, 0, len(changes))
	for _, c := range changes {
		comments, leading, parsed := e.commentsOf(c)
		if !parsed {
			out = append(out, c)
			continue
		}
		kept := slices.DeleteFunc(slices.Clone(c.Edits), func(one edit.TextEdit) bool {
			return worded(comments, leading, one)
		})
		if len(kept) > 0 {
			c.Edits = kept
			out = append(out, c)
		}
	}
	return out
}

// comment is one comment of a Go file: the offset of its first byte in the file, and its text
// with its markers.
type comment struct {
	text  string
	start int
}

// commentsOf returns the comments of the Go file that c edits, in the order of the file, and the
// offsets of the words that start the doc comments of the declarations whose name is at the
// start of an edit of c, by the rule of [leads]. It reports false when c is no edit of a Go file,
// or when the file cannot be read or does not parse.
func (e *Engine) commentsOf(c edit.Change) ([]comment, map[int]bool, bool) {
	if c.Kind != edit.ChangeEdit || !lang.Claims(string(c.Path), e.declared.Extensions) {
		return nil, nil, false
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, e.fullPath(c.Path), nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, false
	}
	var comments []comment
	for _, group := range f.Comments {
		for _, one := range group.List {
			comments = append(comments, comment{start: fset.Position(one.Pos()).Offset, text: one.Text})
		}
	}
	return comments, leads(fset, f, c.Edits), true
}

// leads returns the offsets of the words that start the doc comments of the declarations of f
// whose name is at the start of an edit of edits. The word that starts a doc comment follows
// the markers and the spaces of its first comment.
func leads(fset *token.FileSet, f *ast.File, edits []edit.TextEdit) map[int]bool {
	starts := map[int]bool{}
	for _, one := range edits {
		starts[one.Span.Start.Offset] = true
	}
	out := map[int]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		doc, names := documented(n)
		for _, name := range names {
			if doc != nil && starts[fset.Position(name.Pos()).Offset] {
				first := doc.List[0]
				unmarked := strings.TrimPrefix(strings.TrimPrefix(first.Text, "//"), "/*")
				text := strings.TrimLeftFunc(unmarked, unicode.IsSpace)
				out[fset.Position(first.Pos()).Offset+len(first.Text)-len(text)] = true
			}
		}
		return true
	})
	return out
}

// documented returns the doc comment of the declaration n and the names that it declares. A
// declaration of one type, variable or constant without parentheses has the doc comment of its
// keyword. Any other node has neither.
func documented(n ast.Node) (*ast.CommentGroup, []*ast.Ident) {
	switch held := n.(type) {
	case *ast.FuncDecl:
		return held.Doc, []*ast.Ident{held.Name}
	case *ast.GenDecl:
		if held.Lparen.IsValid() {
			return nil, nil
		}
		switch spec := held.Specs[0].(type) {
		case *ast.TypeSpec:
			return held.Doc, []*ast.Ident{spec.Name}
		case *ast.ValueSpec:
			return held.Doc, spec.Names
		}
	case *ast.TypeSpec:
		return held.Doc, []*ast.Ident{held.Name}
	case *ast.ValueSpec:
		return held.Doc, held.Names
	case *ast.Field:
		return held.Doc, held.Names
	}
	return nil, nil
}

// worded reports whether the edit one rewrites a word of the text of a comment: it starts in
// one of comments, and it is neither one of the words that leading lists nor the last name of
// a doc link by the rule of [linked].
func worded(comments []comment, leading map[int]bool, one edit.TextEdit) bool {
	start := one.Span.Start.Offset
	i, inside := slices.BinarySearchFunc(comments, start, func(c comment, at int) int {
		switch {
		case c.start+len(c.text) <= at:
			return -1
		case c.start > at:
			return 1
		}
		return 0
	})
	if !inside || leading[start] {
		return false
	}
	c := comments[i]
	return !linked(c.text, start-c.start, min(one.Span.End.Offset-c.start, len(c.text)))
}

// linked reports whether the bytes from start to end of the comment text are the last name of a
// doc link: [Name], [Recv.Name], [pkg.Name] or [pkg.Recv.Name], also with a star before the
// first name. Each name before the last is an identifier with a dot after it.
func linked(text string, start, end int) bool {
	open := strings.LastIndexByte(text[:start], '[')
	if open == -1 || !strings.HasPrefix(text[end:], "]") {
		return false
	}
	qualified := strings.TrimPrefix(text[open+1:start], "*")
	if qualified == "" {
		return true
	}
	qualifiers, dotted := strings.CutSuffix(qualified, ".")
	return dotted && !slices.ContainsFunc(strings.Split(qualifiers, "."), func(name string) bool {
		return !token.IsIdentifier(name)
	})
}
