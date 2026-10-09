// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"bytes"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
)

// preview returns the rewrites of the edits of the plan as a diff of lines, one for each run of
// lines that the edits of a file change. Was is the lines of a run in the sealed content, Now is
// the lines that replace them in the projection, and Line is the one-based line of the sealed
// content on which they start. Edits on one line or on adjacent lines share a run. A run leaves
// out its first and its last lines while the edits keep them, so an insertion of whole lines
// has an empty Was, and a run whose edits keep every line has no rewrite. The lines of a rewrite
// have no line ending. A create, a deletion and a move have no rewrite, and the changes of the
// outcome list them.
//
// [edit.Policy.Admit] admits one change of the content of a path, with its edits in the order
// that [edit.Apply] requires, so the projection of the path, or of the destination of its move,
// is its sealed content with the edits of the change.
func preview(plan edit.Plan, sealed, projected map[source.Path][]byte) []edit.Rewrite {
	moves := relocations(plan)
	var out []edit.Rewrite
	for _, c := range plan.Changes {
		if c.Kind != edit.ChangeEdit {
			continue
		}
		after := projected[c.Path]
		if to, moved := moves[c.Path]; moved {
			after = projected[to]
		}
		out = append(out, runs(c.Path, sealed[c.Path], after, c.Edits)...)
	}
	return out
}

// runs returns the rewrites of the file at p by the rules of [preview]. before is the sealed
// content of the file, and after is before with edits.
func runs(p source.Path, before, after []byte, edits []edit.TextEdit) []edit.Rewrite {
	was := strings.Split(string(before), "\n")
	now := strings.Split(string(after), "\n")
	spans := rows(before, replaced(edits))
	var out []edit.Rewrite
	// shift is the number of lines that the edits before a run add, or remove when negative.
	shift := 0
	for i := 0; i < len(edits); {
		first, last := spans[i][0], spans[i][1]
		grown := 0
		for ; i < len(edits) && spans[i][0] <= last+1; i++ {
			last = spans[i][1]
			cut := before[edits[i].Span.Start.Offset:edits[i].Span.End.Offset]
			grown += strings.Count(edits[i].New, "\n") - bytes.Count(cut, []byte("\n"))
		}
		old, fresh := was[first:last+1], now[first+shift:last+1+shift+grown]
		kept := 0
		for kept < min(len(old), len(fresh)) && old[kept] == fresh[kept] {
			kept++
		}
		tail := 0
		for tail < min(len(old), len(fresh))-kept && old[len(old)-1-tail] == fresh[len(fresh)-1-tail] {
			tail++
		}
		old, fresh = old[kept:len(old)-tail], fresh[kept:len(fresh)-tail]
		if len(old) > 0 || len(fresh) > 0 {
			out = append(out, edit.Rewrite{Path: p, Line: first + kept + 1, Was: joined(old), Now: joined(fresh)})
		}
		shift += grown
	}
	return out
}

// joined returns lines as text: each line without the carriage return of a crlf line ending,
// with a line feed between two lines.
func joined(lines []string) string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, strings.TrimSuffix(line, "\r"))
	}
	return strings.Join(out, "\n")
}

// row returns the zero-based line of offset in content, with offset clamped to content.
func row(content []byte, offset int) int {
	return bytes.Count(content[:min(max(offset, 0), len(content))], []byte("\n"))
}
