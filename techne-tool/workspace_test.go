// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/tool"
)

// declaring returns a declaration of the language fixture named name, of kind, in the unit
// and the file path. It starts at offset from and ends 5 bytes after it, and has the
// documentation doc.
func declaring(unit, path, name string, kind sema.Kind, from int, doc string) sema.Symbol {
	return sema.Symbol{
		ID:   sema.NewID(fixture, source.Path(unit), name, kind),
		Name: name, Kind: kind, Language: fixture,
		Span: source.Span{
			Path:  source.Path(path),
			Start: source.Position{Offset: from},
			End:   source.Position{Offset: from + 5},
		},
		Visibility: sema.Exported, Doc: doc,
	}
}

// mapped returns a read service of found that claims the root, the workspace tool over it, and
// the decoded output of input.
func mapped(t *testing.T, input string, found ...sema.Symbol) tool.Units {
	t.Helper()
	over := serving(found...)
	over.claims = map[source.Path]bool{".": true}
	built, err := tool.Workspace(over)
	assert.NoError(t, err, "the error of Workspace")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.Units
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the output")
	return out
}

// workspace is two units: pkg with a package comment, two files and three declarations, one
// of them unexported, and pkg/sub with one file and one declaration.
func workspace() []sema.Symbol {
	hidden := declaring("pkg", "pkg/b.fx", "helper", sema.KindFunction, 20, "")
	hidden.Visibility = sema.Unexported
	clause := declaring("pkg", "pkg/doc.fx", "pkg", sema.KindPackage, 0, "Package pkg keeps stores.\n\nMore text.")
	clause.Visibility = sema.Unexported
	return []sema.Symbol{
		declaring("pkg/sub", "pkg/sub/c.fx", "Sub", sema.KindStruct, 0, ""),
		clause,
		declaring("pkg", "pkg/a.fx", "Store", sema.KindStruct, 0, ""),
		declaring("pkg", "pkg/b.fx", "Get", sema.KindFunction, 0, ""),
		hidden,
	}
}

func TestWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Workspace", func(t *testing.T) {
		t.Parallel()

		t.Run("starts its description with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Workspace(serving())
			assert.NoError(t, err, "the error of Workspace")
			assert.HasPrefix(t, built.Description(), "PREFER OVER ", "the description")
		})

		t.Run("returns each unit once in the order of its path", func(t *testing.T) {
			t.Parallel()
			got := mapped(t, `{}`, workspace()...)
			assert.Length(t, got.Items, 2, "the units")
			assert.Equal(t, got.Items[0].Unit, "pkg", "the first unit")
			assert.Equal(t, got.Items[1].Unit, "pkg/sub", "the second unit")
		})

		t.Run("counts the files of a unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, mapped(t, `{}`, workspace()...).Items[0].Files, 3, "the files of pkg")
		})

		t.Run("counts the visible declarations of a unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, mapped(t, `{}`, workspace()...).Items[0].Declarations, 2, "the declarations of pkg")
		})

		t.Run("counts every declaration of a unit with private", func(t *testing.T) {
			t.Parallel()
			got := mapped(t, `{"private":true}`, workspace()...)
			assert.Equal(t, got.Items[0].Declarations, 4, "the declarations of pkg")
		})

		t.Run("returns the first sentence of the package comment as the summary", func(t *testing.T) {
			t.Parallel()
			got := mapped(t, `{}`, workspace()...)
			assert.Equal(t, got.Items[0].Summary, "Package pkg keeps stores.", "the summary of pkg")
			assert.Empty(t, got.Items[1].Summary, "the summary of pkg/sub")
		})

		t.Run("returns the language of each unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, mapped(t, `{}`, workspace()...).Items[0].Language, string(fixture), "the language of pkg")
		})

		t.Run("refuses a scope that names a file", func(t *testing.T) {
			t.Parallel()
			got := mapped(t, `{"scope":"pkg/a.fx"}`, workspace()...)
			assert.Equal(t, got.Error.Code, "refused", "the code of the failure")
			assert.Contains(t, got.Error.Reason, "names a file", "the reason of the failure")
		})

		t.Run("returns an unsupported failure for a scope that no engine serves", func(t *testing.T) {
			t.Parallel()
			got := mapped(t, `{"scope":"docs"}`, workspace()...)
			assert.Equal(t, got.Error.Code, "unsupported", "the code of the failure")
		})

		t.Run("returns the first units that fit the budget with a truncation caveat", func(t *testing.T) {
			t.Parallel()
			var found []sema.Symbol
			for i := range 40 {
				found = append(found, declaring(fmt.Sprintf("unit%02d", i), fmt.Sprintf("unit%02d/a.fx", i),
					"Store", sema.KindStruct, 0, ""))
			}
			got := mapped(t, `{"max_tokens":100}`, found...)
			assert.InRange(t, len(got.Items), -1<<63, 39, "the units within 100 tokens")
			assert.Equal(t, truncations(got.Provenance), []string{fmt.Sprintf(
				"%d of 40 units returned within the token budget, those with the most declarations", len(got.Items))},
				"the notes of the truncation caveats")
		})

		t.Run("keeps the units with the most declarations in the order of their paths", func(t *testing.T) {
			t.Parallel()
			var found []sema.Symbol
			for i := range 40 {
				unit := fmt.Sprintf("unit%02d", i)
				found = append(found, declaring(unit, unit+"/a.fx", "Store", sema.KindStruct, 0, ""))
			}
			// unit30 has the most declarations and unit10 the second most, so their rank and their
			// paths order them differently.
			for unit, extra := range map[string]int{"unit30": 5, "unit10": 3} {
				for j := range extra {
					more := declaring(unit, unit+"/b.fx", fmt.Sprintf("Extra%d", j), sema.KindStruct, 10*j+10, "")
					found = append(found, more)
				}
			}
			got := mapped(t, `{"max_tokens":50}`, found...)
			assert.Length(t, got.Items, 2, "the units within 50 tokens")
			assert.Equal(t, got.Items[0].Unit, "unit10", "the first unit")
			assert.Equal(t, got.Items[1].Unit, "unit30", "the second unit")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the directory and the count of units", func(t *testing.T) {
			t.Parallel()
			assert.HasPrefix(t, mapped(t, `{}`, workspace()...).Render(), ". — 2 units\n", "the heading")
		})

		t.Run("writes each unit on one line with its counts and its summary", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, mapped(t, `{}`, workspace()...).Render(),
				"\npkg      fixture  3 files  2 declarations  Package pkg keeps stores.\n", "the line of pkg")
		})

		t.Run("aligns the counts of units of one and of two digits", func(t *testing.T) {
			t.Parallel()
			found := []sema.Symbol{declaring("a", "a/x.fx", "One", sema.KindStruct, 0, "")}
			for i := range 10 {
				found = append(found, declaring("b", fmt.Sprintf("b/%d.fx", i), "Two", sema.KindStruct, 0, ""))
			}
			got := mapped(t, `{}`, found...).Render()
			assert.That(t, got).
				Contains("\na  fixture   1 file    1 declaration\n", "the line of a").
				Contains("\nb  fixture  10 files  10 declarations\n", "the line of b")
		})

		t.Run("writes the reason of a failure", func(t *testing.T) {
			t.Parallel()
			got := mapped(t, `{"scope":"pkg/a.fx"}`).Render()
			assert.HasPrefix(t, got, "refused: ", "the render of the failure")
		})
	})
}
