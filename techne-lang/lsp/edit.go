// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"bytes"
	"cmp"
	"fmt"
	"maps"
	"os"
	"slices"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// changes converts a workspace edit into the changes of a plan, in the order LSP 3.17 applies
// them.
//
// The documentChanges of an edit apply in order, and an edit that has them ignores its
// changes map, as LSP 3.17 specifies for a client that declares documentChanges. The changes
// map applies in the order of its URIs. The result contains one change of content per file
// and a move after the change of a file that moves:
//
//   - A file that the edit creates is one [edit.ChangeCreate] whose content contains the
//     text edits of the file.
//   - A text edit of a file that the edit renamed applies to the file under its old path, and
//     the write path moves the file after it applies the content.
//   - A file with one list of text edits is an [edit.ChangeEdit] with those edits. A file that
//     the edit changes more than once is one edit of the lines between the first and the last
//     changed byte, from [differing].
//   - A file that the edit deletes is an [edit.ChangeDelete], without its text edits.
//
// A range converts against the document in texts for a path it contains, and against the
// file on disk otherwise. A create over an existing file, a rename onto an existing file or a
// delete of a missing file returns [engine.ErrRefuse], unless its options allow the case. A
// rename or a delete of a directory returns [engine.ErrRefuse].
func (e *Engine) changes(
	workspace *protocol.WorkspaceEdit,
	texts map[source.Path]document,
) ([]edit.Change, error) {
	if workspace == nil {
		return nil, nil
	}
	d := &draft{engine: e, texts: texts, files: map[source.Path]*drafted{}}
	if len(workspace.DocumentChanges) > 0 {
		for _, one := range workspace.DocumentChanges {
			if err := d.apply(one); err != nil {
				return nil, err
			}
		}
		return d.result(), nil
	}
	for _, of := range slices.Sorted(maps.Keys(workspace.Changes)) {
		if err := d.change(of, workspace.Changes[of]); err != nil {
			return nil, err
		}
	}
	return d.result(), nil
}

// draft is the state of the files that a workspace edit touches, as the edit applies.
type draft struct {
	engine *Engine
	texts  map[source.Path]document
	// files is each touched file by its path at this point of the edit.
	files map[source.Path]*drafted
	// order is each touched file in the order the edit first touched it.
	order []*drafted
}

// drafted is one file that a workspace edit touches.
type drafted struct {
	// from is the path of the file before the edit, or empty for a file that the edit
	// creates. at is the path of the file at this point of the edit.
	from, at source.Path
	// before is the content before the edit, and after the content at this point of it.
	before, after []byte
	// edits are the converted text edits of the first list that applied to the file, and
	// rewrites counts the operations that changed its content.
	edits    []edit.TextEdit
	rewrites int
	// gone reports that the edit deleted the file.
	gone bool
}

// apply applies one operation of documentChanges.
func (d *draft) apply(one protocol.DocumentChange) error {
	switch op := one.(type) {
	case *protocol.TextDocumentEdit:
		return d.change(op.TextDocument.URI, plain(op.Edits))
	case *protocol.CreateFile:
		overwrite, ignore := false, false
		if op.Options != nil {
			overwrite, ignore = set(op.Options.Overwrite), set(op.Options.IgnoreIfExists)
		}
		return d.create(d.engine.pathOf(op.URI), overwrite, ignore)
	case *protocol.RenameFile:
		overwrite, ignore := false, false
		if op.Options != nil {
			overwrite, ignore = set(op.Options.Overwrite), set(op.Options.IgnoreIfExists)
		}
		return d.rename(d.engine.pathOf(op.OldURI), d.engine.pathOf(op.NewURI), overwrite, ignore)
	case *protocol.DeleteFile:
		ignore := op.Options != nil && set(op.Options.IgnoreIfNotExists)
		return d.remove(d.engine.pathOf(op.URI), ignore)
	}
	return nil
}

// set reports whether an optional flag is present and true.
func set(flag *bool) bool { return flag != nil && *flag }

// change applies one list of text edits to the file at the URI of.
func (d *draft) change(of uri.URI, edits []protocol.TextEdit) error {
	f, err := d.existing(d.engine.pathOf(of))
	if err != nil {
		return err
	}
	name := f.from
	if name == "" {
		name = f.at
	}
	converted, err := d.engine.textEdits(texted(name, f.after), edits)
	if err != nil {
		return err
	}
	content, err := edit.Apply(f.after, converted)
	if err != nil {
		return fmt.Errorf("lsp: %s: %s: %w", d.engine.server.Name, name, err)
	}
	if f.rewrites == 0 {
		f.edits = converted
	}
	f.after = content
	f.rewrites++
	return nil
}

// create applies a CreateFile operation at p. For a path without a file it creates an empty
// file. For an existing file, overwrite empties it, and ignore without overwrite leaves it as
// it is. Without either option create returns [engine.ErrRefuse] for an existing file.
func (d *draft) create(p source.Path, overwrite, ignore bool) error {
	if d.exists(p) {
		switch {
		case overwrite:
			f, err := d.existing(p)
			if err != nil {
				return err
			}
			f.after = []byte{}
			f.rewrites++
			return nil
		case ignore:
			return nil
		}
		return fmt.Errorf("%w: %s: the edit creates %s, which exists", engine.ErrRefuse, d.engine.server.Name, p)
	}
	if f, known := d.files[p]; known && f.gone {
		f.gone = false
		f.after = []byte{}
		f.rewrites++
		return nil
	}
	d.track(&drafted{at: p, after: []byte{}})
	return nil
}

// rename applies a RenameFile operation, which moves the file at from to to. For a destination
// that exists, ignore without overwrite skips the operation, and any other option returns
// [engine.ErrRefuse], because a move of the write path does not overwrite a file.
func (d *draft) rename(from, to source.Path, overwrite, ignore bool) error {
	if d.exists(to) {
		if ignore && !overwrite {
			return nil
		}
		return fmt.Errorf("%w: %s: the edit moves %s onto %s, which exists",
			engine.ErrRefuse, d.engine.server.Name, from, to)
	}
	if d.directory(from) {
		return fmt.Errorf("%w: %s: the edit moves the directory %s, and a plan moves files only",
			engine.ErrRefuse, d.engine.server.Name, from)
	}
	f, err := d.existing(from)
	if err != nil {
		return err
	}
	delete(d.files, from)
	f.at = to
	d.files[to] = f
	return nil
}

// remove applies a DeleteFile operation. With ignore it leaves a missing file alone.
func (d *draft) remove(p source.Path, ignore bool) error {
	if !d.exists(p) {
		if ignore {
			return nil
		}
		return fmt.Errorf("%w: %s: the edit deletes %s, which does not exist",
			engine.ErrRefuse, d.engine.server.Name, p)
	}
	if d.directory(p) {
		return fmt.Errorf("%w: %s: the edit deletes the directory %s, and a plan deletes files only",
			engine.ErrRefuse, d.engine.server.Name, p)
	}
	f, err := d.existing(p)
	if err != nil {
		return err
	}
	f.gone = true
	return nil
}

// existing returns the file at p at this point of the edit, and reads a file that the edit
// has not touched from texts or from disk. A file that the edit deleted returns
// [engine.ErrRefuse].
func (d *draft) existing(p source.Path) (*drafted, error) {
	if f, known := d.files[p]; known {
		if f.gone {
			return nil, fmt.Errorf("%w: %s: the edit changes %s after it deletes it",
				engine.ErrRefuse, d.engine.server.Name, p)
		}
		return f, nil
	}
	doc, given := d.texts[p]
	if !given {
		var err error
		if doc, err = d.engine.read(p); err != nil {
			return nil, err
		}
	}
	f := &drafted{from: p, at: p, before: doc.content, after: doc.content}
	d.track(f)
	return f, nil
}

// exists reports whether a file exists at p at this point of the edit.
func (d *draft) exists(p source.Path) bool {
	if f, known := d.files[p]; known {
		return !f.gone
	}
	if _, given := d.texts[p]; given {
		return true
	}
	_, err := os.Stat(d.engine.fullPath(p))
	return err == nil
}

// directory reports whether p names a directory on disk that the edit has not touched.
func (d *draft) directory(p source.Path) bool {
	if _, known := d.files[p]; known {
		return false
	}
	info, err := os.Stat(d.engine.fullPath(p))
	return err == nil && info.IsDir()
}

// track adds f to the files of the edit.
func (d *draft) track(f *drafted) {
	d.files[f.at] = f
	d.order = append(d.order, f)
}

// result returns the changes of the edit, in the order the edit first touched each file.
func (d *draft) result() []edit.Change {
	var out []edit.Change
	for _, f := range d.order {
		switch {
		case f.from == "" && f.gone:
		case f.from == "":
			out = append(out, edit.Change{Kind: edit.ChangeCreate, Path: f.at, Content: f.after})
		case f.gone:
			out = append(out, edit.Change{Kind: edit.ChangeDelete, Path: f.from})
		default:
			switch {
			case bytes.Equal(f.before, f.after):
			case f.rewrites == 1 && len(f.edits) > 0:
				out = append(out, edit.Change{Kind: edit.ChangeEdit, Path: f.from, Edits: f.edits})
			default:
				whole, _ := differing(f.from, f.before, f.after)
				out = append(out, whole)
			}
			if f.at != f.from {
				out = append(out, edit.Change{Kind: edit.ChangeMove, Path: f.from, To: f.at})
			}
		}
	}
	return out
}

// plain returns the text edits of a TextDocumentEdit. An annotated edit is a text edit with a
// label. A snippet edit is skipped, because its text is a template with placeholders.
func plain(edits []protocol.TextDocumentEditElement) []protocol.TextEdit {
	out := make([]protocol.TextEdit, 0, len(edits))
	for _, one := range edits {
		switch held := one.(type) {
		case *protocol.TextEdit:
			out = append(out, *held)
		case *protocol.AnnotatedTextEdit:
			out = append(out, protocol.TextEdit{Range: held.Range, NewText: held.NewText})
		}
	}
	return out
}

// textEdits converts the text edits of one document into [edit.TextEdit] values against doc,
// sorted by start offset. Edits with one start offset keep their order, as LSP 3.17 requires
// for inserts at one position. An edit outside doc, and an edit that starts before the end of
// the edit before it, return [engine.ErrRefuse].
func (e *Engine) textEdits(doc document, edits []protocol.TextEdit) ([]edit.TextEdit, error) {
	out := make([]edit.TextEdit, 0, len(edits))
	for _, one := range edits {
		if !doc.inside(one.Range) {
			return nil, fmt.Errorf("%w: %s: an edit of %s ends on line %d, past the end of the file",
				engine.ErrRefuse, e.server.Name, doc.path, one.Range.End.Line+1)
		}
		out = append(out, edit.TextEdit{Span: doc.span(one.Range), New: one.NewText})
	}
	slices.SortStableFunc(out, func(a, b edit.TextEdit) int {
		return cmp.Compare(a.Span.Start.Offset, b.Span.Start.Offset)
	})
	for i := 1; i < len(out); i++ {
		if out[i].Span.Start.Offset < out[i-1].Span.End.Offset {
			return nil, fmt.Errorf("%w: %s: overlapping edits of %s at bytes %d and %d",
				engine.ErrRefuse, e.server.Name, doc.path, out[i-1].Span.Start.Offset, out[i].Span.Start.Offset)
		}
	}
	return out, nil
}

// differing returns the change of p from before to after as one edit of whole lines: from
// the start of the first line that differs to the end of the last line that differs. It
// reports false when the contents are equal.
func differing(p source.Path, before, after []byte) (edit.Change, bool) {
	if bytes.Equal(before, after) {
		return edit.Change{}, false
	}
	head := 0
	for head < len(before) && head < len(after) && before[head] == after[head] {
		head++
	}
	for head > 0 && before[head-1] != '\n' {
		head--
	}
	tail := 0
	for tail < len(before)-head && tail < len(after)-head &&
		before[len(before)-1-tail] == after[len(after)-1-tail] {
		tail++
	}
	for tail > 0 && before[len(before)-tail-1] != '\n' {
		tail--
	}
	return edit.Change{Kind: edit.ChangeEdit, Path: p, Edits: []edit.TextEdit{{
		Span: source.Span{
			Path:  p,
			Start: source.Position{Offset: head},
			End:   source.Position{Offset: len(before) - tail},
		},
		New: string(after[head : len(after)-tail]),
	}}}, true
}
