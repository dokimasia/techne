// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// Plan returns the changes of an operation over the files of the workspace:
//
//   - [edit.RenameSymbol] rewrites the name of the declaration and the name of every use of it.
//   - [edit.DocumentSymbol] replaces the documentation above the declaration.
//   - [edit.MoveFile] moves the file, which no line of the language names.
//
// Plan reads every file of the workspace, because a use can be in any file. A target in a scope
// without a file of the language gets a skipped result. Plan declines every other operation.
// It refuses a target that does not identify a declaration or a file of the workspace.
func (e *Engine) Plan(
	ctx context.Context,
	req engine.Request,
	op edit.Operation,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	switch op {
	case edit.RenameSymbol, edit.DocumentSymbol, edit.MoveFile:
	default:
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: mock: the language has no planner for %s", engine.ErrDecline, op)
	}
	w, err := e.read(ctx, everywhere(req))
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	if p := targeted(target); !lang.Claims(string(p), e.declared.Extensions) && !w.claims(p) {
		return result(e, []edit.Change(nil), w, p), nil
	}

	var out engine.Result[edit.Change]
	switch op {
	case edit.RenameSymbol:
		out, err = renaming(w, target, strings.TrimSpace(args[edit.ArgNewName]))
	case edit.DocumentSymbol:
		out, err = documenting(w, target, args[edit.ArgDoc])
	case edit.MoveFile:
		out, err = moving(w, target, strings.TrimSpace(args[edit.ArgDestination]))
	}
	if err != nil {
		return engine.Result[edit.Change]{}, err
	}
	return result(e, out.Items, w, engine.Root), nil
}

// targeted returns the path of the file of target, or the empty string for a target that
// identifies a declaration by its ID.
func targeted(target edit.Target) source.Path {
	switch target.Kind {
	case edit.TargetSpan:
		return target.Span.Path
	case edit.TargetFile:
		return target.Path
	case edit.TargetUnset, edit.TargetSymbol:
	}
	return ""
}

// renaming returns the edits that rename the declaration that target names, and every use of
// its name, to name. It refuses a name that is not one word, the name that the declaration
// has, and a declaration whose name another declaration shares, because a use refers to a
// declaration by its name alone.
func renaming(w workspace, target edit.Target, name string) (engine.Result[edit.Change], error) {
	one, err := pointed(w, target)
	switch {
	case err != nil:
		return engine.Result[edit.Change]{}, err
	case name == "" || strings.ContainsAny(name, " \t\n"):
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: mock: %s needs a new name of one word", engine.ErrRefuse, edit.RenameSymbol)
	case name == one.Name:
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: mock: %s is the name that the declaration has",
			engine.ErrRefuse, name)
	}
	if shared := sharing(w.symbols, one.Name); shared > 1 {
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: mock: a use of %s refers to each of its %d declarations", engine.ErrRefuse, one.Name, shared)
	}

	at := map[source.Path][]edit.TextEdit{}
	for _, p := range slices.Sorted(maps.Keys(w.lines)) {
		for _, line := range w.lines[p] {
			if line.Name == one.Name || line.Uses == one.Name {
				at[p] = append(at[p], edit.TextEdit{Span: line.At, New: name})
			}
		}
	}
	return changes(at), nil
}

// sharing returns the number of declarations of items named name.
func sharing(items []sema.Symbol, name string) int {
	n := 0
	for _, one := range items {
		if one.Name == name {
			n++
		}
	}
	return n
}

// documenting returns the edit that replaces the documentation above the declaration that
// target names with text, one documentation line per line of text, at the indentation of the
// declaration. A file whose lines end in a carriage return and a line feed gets both after
// each line. It refuses a blank text.
func documenting(w workspace, target edit.Target, text string) (engine.Result[edit.Change], error) {
	one, err := pointed(w, target)
	switch {
	case err != nil:
		return engine.Result[edit.Change]{}, err
	case strings.TrimSpace(text) == "":
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: mock: %s needs a text to write", engine.ErrRefuse, edit.DocumentSymbol)
	}

	p := one.Span.Path
	from, to := above(w.content[p], one)
	ending := "\n"
	if bytes.Contains(w.content[p], []byte("\r\n")) {
		ending = "\r\n"
	}
	indent := strings.Repeat(" ", one.Span.Start.Column)
	var written []string
	for line := range strings.SplitSeq(text, "\n") {
		written = append(written, indent+documents+" "+line+ending)
	}
	return changes(map[source.Path][]edit.TextEdit{p: {{
		Span: source.Span{Path: p, Start: source.Position{Offset: from}, End: source.Position{Offset: to}},
		New:  strings.Join(written, ""),
	}}}), nil
}

// above returns the byte range of the documentation lines right above the declaration of,
// or the empty range at the start of its line when there are none.
func above(content []byte, of sema.Symbol) (from, to int) {
	start := of.Span.Start.Offset - of.Span.Start.Column
	from = start
	for from > 0 {
		line := from - 1
		for line > 0 && content[line-1] != '\n' {
			line--
		}
		if !strings.HasPrefix(strings.TrimLeft(string(content[line:from-1]), " "), documents) {
			break
		}
		from = line
	}
	return from, start
}

// moving returns the move of the file that target names to to. It refuses a target that names
// no file of the workspace, and a move without a destination.
func moving(w workspace, target edit.Target, to string) (engine.Result[edit.Change], error) {
	switch {
	case target.Kind != edit.TargetFile || !slices.Contains(w.claimed, target.Path):
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: mock: %s names a file of the workspace, and the target names none", engine.ErrRefuse, edit.MoveFile)
	case to == "":
		return engine.Result[edit.Change]{}, fmt.Errorf(
			"%w: mock: %s needs %s", engine.ErrRefuse, edit.MoveFile, edit.ArgDestination)
	}
	return engine.Result[edit.Change]{
		Items: []edit.Change{{Kind: edit.ChangeMove, Path: target.Path, To: source.Path(to)}},
	}, nil
}

// pointed returns the declaration that target names: the declaration that starts at the start
// of its span, or the one declaration with its ID. It returns [engine.ErrRefuse] for a target
// that does not identify a declaration, and for an ID of two or more declarations.
func pointed(w workspace, target edit.Target) (sema.Symbol, error) {
	switch target.Kind {
	case edit.TargetSpan:
		for _, one := range w.symbols {
			if one.Span.Path == target.Span.Path && one.Span.Start.Offset == target.Span.Start.Offset {
				return one, nil
			}
		}
	case edit.TargetSymbol:
		switch found := matching(w.symbols, target.Symbol); len(found) {
		case 0:
		case 1:
			return found[0], nil
		default:
			return sema.Symbol{}, fmt.Errorf("%w: mock: %s names %d declarations",
				engine.ErrRefuse, target.Symbol, len(found))
		}
	case edit.TargetUnset, edit.TargetFile:
	}
	return sema.Symbol{}, fmt.Errorf("%w: mock: the target names no declaration", engine.ErrRefuse)
}

// changes returns the edits of at as one change per file, in path order, with the edits of
// each file sorted by offset, as the write path applies them.
func changes(at map[source.Path][]edit.TextEdit) engine.Result[edit.Change] {
	var out []edit.Change
	for _, p := range slices.Sorted(maps.Keys(at)) {
		edits := at[p]
		slices.SortFunc(edits, func(a, b edit.TextEdit) int { return a.Span.Start.Offset - b.Span.Start.Offset })
		out = append(out, edit.Change{Kind: edit.ChangeEdit, Path: p, Edits: edits})
	}
	return engine.Result[edit.Change]{Items: out}
}

// Engine implements the port of every role that it serves.
var _ interface {
	engine.Outliner
	engine.Searcher
	engine.Resolver
	engine.Relator
	engine.Planner
	engine.Checker
	engine.Verifier
} = (*Engine)(nil)
