// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"bytes"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
)

// preview reads a plan's edits back against the content they were
// computed on.
//
// Only the ranges within a file are read. Creating, deleting and moving
// a file are what the change itself says they are, and putting a whole
// file's bytes in a preview would cost more than reading the file.
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
			out = append(out, edit.Rewrite{
				Path: c.Path,
				Line: lineAt(content, start),
				Was:  was,
				Now:  e.New,
			})
		}
	}
	return out
}

// lineAt returns the line an offset sits on, counting from one.
func lineAt(content []byte, offset int) int {
	if offset < 0 || offset > len(content) {
		return 0
	}
	return bytes.Count(content[:offset], []byte("\n")) + 1
}
