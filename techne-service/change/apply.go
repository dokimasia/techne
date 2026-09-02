// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"bytes"
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
//
// Content is settled before anything is relocated, so a file the plan
// both rewrites and moves arrives at its destination rewritten. One
// operation produces exactly that: moving a Java file renames the class
// inside it, because the language ties the two together. Reading the
// sealed content at the move would carry the file over as it was and
// drop the rename, silently and in the one language where it matters.
//
// The relocations are read off the plan rather than performed as they
// are met, because a plan may name one move more than once: ruby-lsp
// answers a rename of a class with the file rename repeated once per
// site it found. Performed in turn, the second reads what the first left
// behind — a path with nothing at it — and the move becomes a deletion
// with the file's content nowhere. Driving a rename over a Ruby class
// lost the file.
func project(plan edit.Plan, sealed map[source.Path][]byte) (map[source.Path][]byte, error) {
	out := map[source.Path][]byte{}
	for _, c := range plan.Changes {
		switch c.Kind {
		case edit.ChangeEdit:
			applied, err := edit.Apply(sealed[c.Path], c.Edits)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", c.Path, err)
			}
			out[c.Path] = applied
		case edit.ChangeCreate:
			out[c.Path] = c.Content
		case edit.ChangeDelete:
			out[c.Path] = nil
		case edit.ChangeMove:
			// Below, once every path holds what the plan leaves in it.
		case edit.ChangeUnset:
			return nil, fmt.Errorf("%s names no kind", c.Path)
		}
	}

	moves, err := relocations(plan)
	if err != nil {
		return nil, err
	}
	for from, to := range moves {
		content, rewritten := out[from]
		if !rewritten {
			content = sealed[from]
		}
		out[to] = content
	}
	// Second, so the order the map is walked in cannot decide whether a
	// destination is written before its source is cleared.
	for from := range moves {
		out[from] = nil
	}
	return out, nil
}

// relocations is where each moved path goes.
//
// One entry per path, so a move named twice is the one move it
// describes. A plan that cannot mean one thing is refused rather than
// resolved: a path with two destinations names two different results,
// and a move onto a path that moves on again describes a file passing
// through somewhere, which no operation means and which the order of
// this map would otherwise decide.
func relocations(plan edit.Plan) (map[source.Path]source.Path, error) {
	out := map[source.Path]source.Path{}
	for _, c := range plan.Changes {
		if c.Kind != edit.ChangeMove {
			continue
		}
		if to, named := out[c.Path]; named && to != c.To {
			return nil, fmt.Errorf("%s is moved to both %s and %s", c.Path, to, c.To)
		}
		out[c.Path] = c.To
	}
	for from, to := range out {
		if _, on := out[to]; on {
			return nil, fmt.Errorf("%s moves to %s, which the same change moves on again", from, to)
		}
	}
	return out, nil
}

// write puts the projection on disk, and puts the workspace back as it
// was if it cannot finish.
//
// The gate has already passed, so a failure here is the filesystem
// refusing rather than the change being wrong. What makes it worth
// undoing is that a change is all or nothing: half a rename compiles
// about as often as none of it, and is far harder to find.
// It reports the paths it wrote, which is not every path the plan
// names: a plan whose result equals what is already there writes
// nothing. Rewriting a file to itself moves its timestamp, which is what
// every build and watcher in the workspace keys on, and reporting it as
// changed tells a caller something happened that did not.
func (s *Service) write(
	plan edit.Plan,
	sealed map[source.Path][]byte,
	projected map[source.Path][]byte,
) ([]source.Path, error) {
	var done []source.Path
	for _, p := range plan.Paths() {
		content, changed := projected[p]
		if !changed || same(sealed[p], content) {
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
		return nil, fmt.Errorf("change: write %s: %w (%s)", p, err, s.restore(done, sealed))
	}
	return done, nil
}

// same reports whether writing this content would leave the file as it
// already is. A file the plan removes is never the same as one that is
// there.
func same(held, projected []byte) bool {
	return projected != nil && bytes.Equal(held, projected)
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
