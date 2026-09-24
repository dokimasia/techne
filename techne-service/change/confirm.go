// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// disk is what the verifier of the language found in files on disk.
type disk struct {
	// found are the errors of the files.
	found []edit.Finding
	// by is the evidence of the verifier: the engine of the first answer, the lowest tier and
	// the least coverage of the answers, and the caveats of every answer.
	by trust.Provenance
}

// whole reports whether the verifier checked every file at [trust.Resolved] with total
// coverage, and without a [trust.CaveatPartialCheck] caveat, as the compiler of the language
// checks it.
func (d disk) whole() bool {
	return d.by.Fidelity >= trust.Resolved && d.by.Completeness == trust.ScopeTotal && !partial(d.by.Caveats)
}

// verified asks the engine of the strongest tier that verifies the language of each of paths
// for the errors of the file on disk. The answer is partial when no engine verifies a path.
func (s *Service) verified(ctx context.Context, req edit.Request, paths []source.Path) (disk, error) {
	out := disk{by: trust.Provenance{Fidelity: trust.Resolved, Completeness: trust.ScopeTotal}}
	for _, p := range paths {
		asking := engine.Request{Scope: p, Language: req.Language, Tests: true}
		answered, ok, _, err := engine.AskAny(ctx, s.catalog, s.router, asking, engine.RoleVerify,
			func(e engine.Engine) (engine.Result[edit.Finding], error) {
				return e.(engine.Verifier).Verify(ctx, asking, nil)
			})
		if err != nil {
			return disk{}, err
		}
		if !ok {
			out.by.Completeness = min(out.by.Completeness, trust.ScopePartial)
			out.by.Caveats = append(out.by.Caveats, trust.Caveat{
				Code: trust.CaveatUnsupported, Note: "no engine verifies " + string(p), Paths: []source.Path{p},
			})
			continue
		}
		out.found = append(out.found, errorsIn(answered.Items)...)
		out.by.Engine = cmp.Or(out.by.Engine, answered.Provenance.Engine)
		out.by.Fidelity = min(out.by.Fidelity, answered.Provenance.Fidelity)
		out.by.Completeness = min(out.by.Completeness, answered.Provenance.Completeness)
		for _, c := range answered.Provenance.Caveats {
			if !slices.ContainsFunc(out.by.Caveats, func(kept trust.Caveat) bool { return kept.Note == c.Note }) {
				out.by.Caveats = append(out.by.Caveats, c)
			}
		}
	}
	return out, nil
}

// confirm checks paths, the files that a change wrote, on disk. It compares their errors with
// before, the check of the files that the change read before the write. It returns out, the
// outcome of the change, as the two checks decide it:
//
//   - The files have more errors after the write, and both checks are whole. confirm takes
//     the steps of done back and refuses the change with the errors after the write.
//   - The files have no more errors, and both checks are whole. The change is kept. Its gate
//     loses its [trust.CaveatPartialCheck] caveats, because the check on disk covered what the
//     gate left out.
//   - A check is not whole, or the check after the write fails. The change is kept. A
//     [trust.CaveatPartialCheck] caveat of its gate states the reason.
func (s *Service) confirm(
	ctx context.Context,
	req edit.Request,
	plan edit.Plan,
	paths []source.Path,
	before disk,
	done []step,
	out edit.Outcome,
) edit.Outcome {
	gate := *out.Gate
	after, err := s.verified(ctx, req, paths)
	switch {
	case err != nil:
		gate.Caveats = append(slices.Clone(gate.Caveats), unconfirmed("failed", []string{reason(err)}))
	case !before.whole() || !after.whole():
		gate.Caveats = append(slices.Clone(gate.Caveats),
			unconfirmed("did not confirm the change", notes(before, after)))
	case len(after.found) > len(before.found):
		stopped := refusedBy(plan, fmt.Sprintf("the change stops %s %s after the write, and %s",
			where(after.found), judging(after.by), restore(done)))
		stopped.Diagnostics, stopped.Rewrites, stopped.Gate = after.found, out.Rewrites, &after.by
		return stopped
	default:
		gate.Caveats = slices.DeleteFunc(slices.Clone(gate.Caveats), func(c trust.Caveat) bool {
			return c.Code == trust.CaveatPartialCheck
		})
	}
	out.Gate = &gate
	out.Applied, out.Changed = true, touched(done)
	return out
}

// unconfirmed returns the caveat of a change that the check of its files on disk did not
// confirm: what the check did, and the reasons.
func unconfirmed(what string, reasons []string) trust.Caveat {
	note := "the check of the files on disk " + what
	if len(reasons) > 0 {
		note += ": " + strings.Join(reasons, "; ")
	}
	return trust.Caveat{Code: trust.CaveatPartialCheck, Note: note}
}

// notes returns why the checks are not whole, without duplicates: the tier of a check below
// [trust.Resolved], and the notes of the caveats of the checks except the [trust.CaveatDynamic]
// caveat of every resolved answer.
func notes(checks ...disk) []string {
	var out []string
	add := func(note string) {
		if !slices.Contains(out, note) {
			out = append(out, note)
		}
	}
	for _, check := range checks {
		if check.by.Fidelity < trust.Resolved {
			add(fmt.Sprintf("%s checks at the tier %s", check.by.Engine, check.by.Fidelity))
		}
		for _, c := range check.by.Caveats {
			if c.Code != trust.CaveatDynamic {
				add(c.Note)
			}
		}
	}
	return out
}

// arrived returns the paths of the projection that have content after the write, sorted.
func arrived(projected map[source.Path][]byte) []source.Path {
	var out []source.Path
	for p, content := range projected {
		if content != nil {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}
