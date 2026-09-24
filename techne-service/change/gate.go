// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// verdict is what the gate made of a projection.
type verdict struct {
	// checked reports whether an engine judged the projection.
	checked bool
	// worse reports whether the projection has more errors than the content it replaces.
	worse bool
	// found are the errors of the projection that the gate counted.
	found []edit.Finding
	// by is the evidence of the engine that judged the projection. A parser can judge what a
	// type checker planned.
	by trust.Provenance
}

// gate judges the projection and the content that it replaces, and reports the change as
// worse when the projection has more errors. A change is refused for what it broke, and not
// for what the files contained before it:
//
//   - The gate of a plan that moves a file leaves out the errors that [imports] returns, and
//     lists them in a [trust.CaveatPartialCheck] caveat of the evidence of the gate.
//   - A gate below [trust.Resolved] counts only the errors that [edited] returns.
func (s *Service) gate(
	ctx context.Context,
	req edit.Request,
	plan edit.Plan,
	sealed, projected map[source.Path][]byte,
) (verdict, error) {
	after, by, checked, err := s.check(ctx, req, projected)
	if err != nil || !checked {
		return verdict{checked: checked}, err
	}
	after, left := imports(plan, after)
	if len(left) > 0 {
		by.Caveats = append(slices.Clone(by.Caveats), unverified(left))
	}
	if len(after) == 0 {
		return verdict{checked: true, by: by}, nil
	}

	was := map[source.Path][]byte{}
	for p := range projected {
		if content, read := sealed[p]; read {
			was[p] = content
		}
	}
	before, _, _, err := s.check(ctx, req, was)
	if err != nil {
		return verdict{}, err
	}
	if by.Fidelity < trust.Resolved {
		after, before = edited(plan, sealed, projected, after, before)
	}
	return verdict{checked: true, worse: len(after) > len(before), found: after, by: by}, nil
}

// check asks the engine of the strongest tier that checks the language of req for the
// errors of files, and returns them with the evidence of the engine. It reports whether an
// engine judged every file: an answer of partial coverage did not.
func (s *Service) check(
	ctx context.Context,
	req edit.Request,
	files map[source.Path][]byte,
) ([]edit.Finding, trust.Provenance, bool, error) {
	if len(files) == 0 {
		return nil, trust.Provenance{}, true, nil
	}
	asking := engine.Request{Scope: req.Scope, Language: req.Language}
	answered, ok, _, err := engine.AskAny(ctx, s.catalog, s.router, asking, engine.RoleCheck,
		func(e engine.Engine) (engine.Result[edit.Finding], error) {
			return e.(engine.Checker).Check(ctx, files)
		})
	if err != nil || !ok {
		return nil, trust.Provenance{}, false, err
	}
	whole := answered.Provenance.Completeness != trust.ScopePartial
	return errorsIn(answered.Items), answered.Provenance, whole, nil
}

// errorsIn returns the findings of found at [diag.SeverityError] or above. A warning does
// not refuse a change.
func errorsIn(found []edit.Finding) []edit.Finding {
	var out []edit.Finding
	for _, one := range found {
		if one.Diagnostic.Severity >= diag.SeverityError {
			out = append(out, one)
		}
	}
	return out
}

// imports returns the errors of found without the ones inside an edit of a file that the
// plan does not move, for a plan that moves a file, and the errors that it left out. Such an
// edit rewrites an import of the moved file, and a server resolves an import on disk, where
// the moved file is not before the write.
func imports(plan edit.Plan, found []edit.Finding) (kept, left []edit.Finding) {
	moves := relocations(plan)
	if len(moves) == 0 {
		return found, nil
	}
	written := map[source.Path][][2]int{}
	for _, c := range plan.Changes {
		if _, moved := moves[c.Path]; c.Kind == edit.ChangeEdit && !moved {
			written[c.Path] = placed(c.Edits)
		}
	}
	for _, one := range found {
		if overlaps(written[one.Diagnostic.Span.Path], one.Diagnostic.Span) {
			left = append(left, one)
		} else {
			kept = append(kept, one)
		}
	}
	return kept, left
}

// unverified returns the caveat that lists the errors that the gate of a move left out, by
// file and one-based line.
func unverified(left []edit.Finding) trust.Caveat {
	var sites []string
	var paths []source.Path
	for _, one := range left {
		at := one.Diagnostic.Span
		sites = append(sites, fmt.Sprintf("%s:%d", at.Path, at.Start.Line+1))
		if !slices.Contains(paths, at.Path) {
			paths = append(paths, at.Path)
		}
	}
	return trust.Caveat{
		Code: trust.CaveatPartialCheck,
		Note: "the gate did not check the imports that the move rewrites, because a server resolves an import " +
			"on disk, where the moved file is not before the write: " + strings.Join(sites, ", "),
		Paths: paths,
	}
}

// edited returns the errors of after and of before that a parser gate counts. A file that
// had no error before the change counts every error after it, so a change that breaks the
// file away from its edits is refused. A file that had errors counts only the errors on the
// lines that the change edits: in after on the lines of the projection, and in before on the
// lines that the edits replace. An edit elsewhere in such a file can change how the grammar
// recovers from a fault that the file contained. An error of a file that the change creates
// always counts.
func edited(
	plan edit.Plan,
	sealed, projected map[source.Path][]byte,
	after, before []edit.Finding,
) (counted, was []edit.Finding) {
	// origin is the sealed path of the content of each projected path.
	origin := map[source.Path]source.Path{}
	for p := range projected {
		if _, read := sealed[p]; read {
			origin[p] = p
		}
	}
	for from, to := range relocations(plan) {
		origin[to] = from
	}
	edits := map[source.Path][]edit.TextEdit{}
	for _, c := range plan.Changes {
		if c.Kind == edit.ChangeEdit {
			edits[c.Path] = c.Edits
		}
	}
	faulty := map[source.Path]bool{}
	for _, one := range before {
		faulty[one.Diagnostic.Span.Path] = true
	}

	for _, one := range after {
		p := one.Diagnostic.Span.Path
		from, read := origin[p]
		if !read || !faulty[from] || meets(rows(projected[p], placed(edits[from])), one.Diagnostic.Span) {
			counted = append(counted, one)
		}
	}
	for _, one := range before {
		p := one.Diagnostic.Span.Path
		if meets(rows(sealed[p], replaced(edits[p])), one.Diagnostic.Span) {
			was = append(was, one)
		}
	}
	return counted, was
}

// placed returns the byte range that the text of each edit covers in the content that the
// edits produce. The edits are in the order that [edit.Apply] requires.
func placed(edits []edit.TextEdit) [][2]int {
	out := make([][2]int, 0, len(edits))
	shift := 0
	for _, e := range edits {
		start := e.Span.Start.Offset + shift
		out = append(out, [2]int{start, start + len(e.New)})
		shift += len(e.New) - (e.Span.End.Offset - e.Span.Start.Offset)
	}
	return out
}

// replaced returns the byte range that each edit replaces in the content it applies to.
func replaced(edits []edit.TextEdit) [][2]int {
	out := make([][2]int, 0, len(edits))
	for _, e := range edits {
		out = append(out, [2]int{e.Span.Start.Offset, e.Span.End.Offset})
	}
	return out
}

// rows returns the zero-based lines of content that each byte range covers, both ends
// included. An empty range covers the line of its offset.
func rows(content []byte, ranges [][2]int) [][2]int {
	out := make([][2]int, 0, len(ranges))
	for _, r := range ranges {
		last := r[0]
		if r[1] > r[0] {
			last = r[1] - 1
		}
		out = append(out, [2]int{row(content, r[0]), row(content, last)})
	}
	return out
}

// meets reports whether the lines of span meet one of lines.
func meets(lines [][2]int, span source.Span) bool {
	return slices.ContainsFunc(lines, func(r [2]int) bool {
		return span.Start.Line <= r[1] && r[0] <= span.End.Line
	})
}

// overlaps reports whether span overlaps one of ranges. An empty span overlaps a range that
// contains its offset.
func overlaps(ranges [][2]int, span source.Span) bool {
	from, to := span.Start.Offset, max(span.End.Offset, span.Start.Offset+1)
	return slices.ContainsFunc(ranges, func(r [2]int) bool { return from < r[1] && r[0] < to })
}

// judging returns what a gate of the tier of by checks, for a refusal: whether the result
// compiles or whether it parses.
func judging(by trust.Provenance) string {
	if by.Fidelity >= trust.Resolved {
		return "compiling"
	}
	return "parsing"
}

// where returns the files of found, for a refusal, or "the file" for none.
func where(found []edit.Finding) string {
	var named []string
	for _, one := range found {
		if !slices.Contains(named, string(one.Diagnostic.Span.Path)) {
			named = append(named, string(one.Diagnostic.Span.Path))
		}
	}
	if len(named) == 0 {
		return "the file"
	}
	return strings.Join(named, ", ")
}
