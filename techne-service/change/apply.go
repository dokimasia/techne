// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
)

// project returns the workspace as the plan would leave it.
//
// It is what the gate judges and, byte for byte, what the write puts on
// disk. A projection built one way and written another would let a dry
// run promise something the apply does not deliver.
//
// A path the plan deletes is present with no content, so a caller can
// tell "this file goes" from "this file was not touched".
func project(plan edit.Plan, sealed map[source.Path][]byte) (map[source.Path][]byte, error) {
	out := map[source.Path][]byte{}
	for _, c := range plan.Changes {
		switch c.Kind {
		case edit.ChangeEdit:
			applied, err := rewritten(sealed[c.Path], c.Edits)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", c.Path, err)
			}
			out[c.Path] = applied
		case edit.ChangeCreate:
			out[c.Path] = c.Content
		case edit.ChangeDelete:
			out[c.Path] = nil
		case edit.ChangeMove:
			out[c.To] = sealed[c.Path]
			out[c.Path] = nil
		case edit.ChangeUnset:
			return nil, fmt.Errorf("%s names no kind", c.Path)
		}
	}
	return out, nil
}

// rewritten applies an edit list to one file in a single pass.
//
// The policy has already established that the edits are sorted and do
// not meet, so the walk copies what lies between them and never looks
// back.
func rewritten(content []byte, edits []edit.TextEdit) ([]byte, error) {
	var out strings.Builder
	out.Grow(len(content))

	at := 0
	for _, e := range edits {
		start, end := e.Span.Start.Offset, e.Span.End.Offset
		if start < at || end > len(content) {
			return nil, fmt.Errorf(
				"the edit at %d..%d is outside the %d bytes it was computed against",
				start, end, len(content))
		}
		out.Write(content[at:start])
		out.WriteString(e.New)
		at = end
	}
	out.Write(content[at:])
	return []byte(out.String()), nil
}

// write puts the projection on disk, and puts the workspace back as it
// was if it cannot finish.
//
// The gate has already passed, so a failure here is the filesystem
// refusing rather than the change being wrong. What makes it worth
// undoing is that a change is all or nothing: half a rename compiles
// about as often as none of it, and is far harder to find.
func (s *Service) write(
	plan edit.Plan,
	sealed map[source.Path][]byte,
	projected map[source.Path][]byte,
) error {
	var done []source.Path
	for _, p := range plan.Paths() {
		content, changed := projected[p]
		if !changed {
			continue
		}
		var err error
		if content == nil {
			err = s.files.Remove(p)
		} else {
			err = s.files.Write(p, content)
		}
		if err == nil {
			done = append(done, p)
			continue
		}
		return fmt.Errorf("change: write %s: %w (%s)", p, err, s.restore(done, sealed))
	}
	return nil
}

// restore puts back the files a failed write had already changed, and
// reports what happened.
//
// The report is a string rather than an error because it is context for
// the failure that stopped the write, not a second failure. A caller
// told only that the write failed does not know whether it is holding a
// workspace that was put back or one that was left half changed.
func (s *Service) restore(done []source.Path, sealed map[source.Path][]byte) string {
	var failed []string
	for _, p := range done {
		original, existed := sealed[p]
		var err error
		if !existed {
			// The file was created by this change, so putting the
			// workspace back means taking it away again.
			err = s.files.Remove(p)
		} else {
			err = s.files.Write(p, original)
		}
		if err != nil {
			failed = append(failed, string(p))
		}
	}
	if len(failed) > 0 {
		return "and " + strings.Join(failed, ", ") + " could not be put back"
	}
	if len(done) == 0 {
		return "nothing had been written"
	}
	return "the files already written were put back"
}
