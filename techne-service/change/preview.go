// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"bytes"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
)

// preview returns a rewrite for each edit of the plan, at the one-based line where the edit
// starts. Was is the text that the edit replaces in the sealed content, and Now is the text
// that it writes. A create, a deletion and a move have no rewrite, and the changes of the
// outcome list them.
func preview(plan edit.Plan, sealed map[source.Path][]byte) []edit.Rewrite {
	var out []edit.Rewrite
	for _, c := range plan.Changes {
		if c.Kind != edit.ChangeEdit {
			continue
		}
		content := sealed[c.Path]
		for _, e := range c.Edits {
			start, end := e.Span.Start.Offset, e.Span.End.Offset
			was := ""
			if start >= 0 && end <= len(content) && start < end {
				was = string(content[start:end])
			}
			out = append(out, edit.Rewrite{Path: c.Path, Line: lineAt(content, start), Was: was, Now: e.New})
		}
	}
	return out
}

// lineAt returns the one-based line of offset in content, or 0 for an offset outside
// content.
func lineAt(content []byte, offset int) int {
	if offset < 0 || offset > len(content) {
		return 0
	}
	return row(content, offset) + 1
}

// row returns the zero-based line of offset in content, with offset clamped to content.
func row(content []byte, offset int) int {
	return bytes.Count(content[:min(max(offset, 0), len(content))], []byte("\n"))
}
