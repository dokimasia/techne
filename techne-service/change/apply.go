// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
)

// project returns the content of each path that the plan changes, as the plan leaves it:
// nil for a path that the plan deletes or moves away. The gate checks the projection, and
// the write puts the same bytes on disk. The edits of a path apply before its move, so a
// file that the plan edits and moves arrives edited. A create without content is an empty
// file.
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
			if c.Content == nil {
				out[c.Path] = []byte{}
			}
		case edit.ChangeDelete:
			out[c.Path] = nil
		case edit.ChangeUnset, edit.ChangeMove:
		}
	}
	for from, to := range relocations(plan) {
		content, edited := out[from]
		if !edited {
			content = sealed[from]
		}
		out[to], out[from] = content, nil
	}
	return out, nil
}

// relocations returns the destination of each path that the plan moves. A move that the
// plan repeats is one entry. [edit.Policy.Admit] refuses a plan that moves a path to two
// destinations, or that moves a file on from its destination.
func relocations(plan edit.Plan) map[source.Path]source.Path {
	out := map[source.Path]source.Path{}
	for _, c := range plan.Changes {
		if c.Kind == edit.ChangeMove {
			out[c.Path] = c.To
		}
	}
	return out
}

// step is one change to the workspace. check runs right before do, under the lock of the
// workspace, and returns a refusal when the file is not as the plan read it. undo takes the
// change back.
type step struct {
	paths []source.Path
	check func() (string, error)
	do    func() error
	undo  func() error
}

// write puts the projection on disk under the lock of the workspace, and returns the paths
// that it changed. It takes the steps of [Service.steps] in order. A step that is refused or
// fails stops the write, the steps already taken are taken back, and the refusal or the error
// states whether that succeeded.
func (s *Service) write(
	ctx context.Context,
	plan edit.Plan,
	sealed, projected map[source.Path][]byte,
) ([]source.Path, string, error) {
	unlock, err := s.files.Lock(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("change: lock the workspace: %w", err)
	}
	defer unlock()

	var done []step
	for _, one := range s.steps(plan, sealed, projected) {
		switch why, err := one.check(); {
		case err != nil:
			return nil, "", fmt.Errorf("%w, and %s", err, restore(done))
		case why != "":
			return nil, why + ", and " + restore(done), nil
		}
		if err := one.do(); err != nil {
			return nil, "", fmt.Errorf("change: write %s: %w, and %s", one.paths[0], err, restore(done))
		}
		done = append(done, one)
	}

	var changed []source.Path
	for _, one := range done {
		changed = append(changed, one.paths...)
	}
	slices.Sort(changed)
	return slices.Compact(changed), "", nil
}

// steps returns the steps that take the workspace from sealed to projected, in this order:
//
//   - each move, in the order of its source, with the write of its destination when the
//     plan also edits the file
//   - the write, the creation or the deletion of each other path, in path order
//
// A path whose projected content equals its sealed content has no step.
func (s *Service) steps(plan edit.Plan, sealed, projected map[source.Path][]byte) []step {
	moves := relocations(plan)
	relocated := map[source.Path]bool{}
	var out []step
	for _, from := range slices.Sorted(maps.Keys(moves)) {
		to := moves[from]
		relocated[from], relocated[to] = true, true
		out = append(out, step{
			paths: []source.Path{from, to},
			check: func() (string, error) {
				if _, why, err := s.current(plan, from); why != "" || err != nil {
					return why, err
				}
				return s.vacant(to)
			},
			do:   func() error { return s.files.Move(from, to) },
			undo: func() error { return s.files.Move(to, from) },
		})
		if content := projected[to]; !bytes.Equal(content, sealed[from]) {
			out = append(out, step{
				paths: []source.Path{to},
				check: func() (string, error) { return "", nil },
				do:    func() error { return s.files.Write(to, content) },
				undo:  func() error { return s.files.Write(to, sealed[from]) },
			})
		}
	}

	for _, p := range plan.Paths() {
		content, changed := projected[p]
		original, read := sealed[p]
		switch {
		case !changed || relocated[p] || read && content != nil && bytes.Equal(original, content):
		case content == nil:
			out = append(out, s.replacing(plan, p, original, func() error { return s.files.Remove(p) }))
		case read:
			out = append(out, s.replacing(plan, p, original, func() error { return s.files.Write(p, content) }))
		default:
			out = append(out, step{
				paths: []source.Path{p},
				check: func() (string, error) { return s.vacant(p) },
				do:    func() error { return s.files.Write(p, content) },
				undo:  func() error { return s.files.Remove(p) },
			})
		}
	}
	return out
}

// replacing returns the step that runs do on the sealed file at p, which original takes
// back.
func (s *Service) replacing(plan edit.Plan, p source.Path, original []byte, do func() error) step {
	return step{
		paths: []source.Path{p},
		check: func() (string, error) {
			_, why, err := s.current(plan, p)
			return why, err
		},
		do:   do,
		undo: func() error { return s.files.Write(p, original) },
	}
}

// restore takes back the steps of done in reverse order, and returns what that did for the
// message of a write that stopped.
func restore(done []step) string {
	if len(done) == 0 {
		return "nothing had been written"
	}
	var failed []string
	for _, one := range slices.Backward(done) {
		if err := one.undo(); err != nil {
			failed = append(failed, string(one.paths[0]))
		}
	}
	if len(failed) > 0 {
		return strings.Join(failed, ", ") + " could not be put back"
	}
	return "the files already written were put back"
}
