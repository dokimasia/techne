// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// The edits of a rename of size to count in [lsptest.Literal], as the protocol writes them: at
// the field size, at the variable size, and at the shorthand properties of Make and of Copy.
// spelledMake is an edit at the shorthand property of Make that writes the key out itself.
const (
	countField    = `{"range":{"start":{"line":3,"character":7},"end":{"line":3,"character":11}},"newText":"count"}`
	countVariable = `{"range":{"start":{"line":6,"character":4},"end":{"line":6,"character":8}},"newText":"count"}`
	countMake     = `{"range":{"start":{"line":8,"character":32},"end":{"line":8,"character":36}},"newText":"count"}`
	countCopy     = `{"range":{"start":{"line":10,"character":32},"end":{"line":10,"character":36}},"newText":"count"}`
	spelledMake   = `{"range":{"start":{"line":8,"character":32},"end":{"line":8,"character":36}},` +
		`"newText":"size: count"}`
	// countItem is an edit at the Item of the literal of Make, at which the outline declares
	// nothing, and countMethod one at the name of the method size of Item.
	countItem   = `{"range":{"start":{"line":8,"character":26},"end":{"line":8,"character":30}},"newText":"count"}`
	countMethod = `{"range":{"start":{"line":12,"character":14},"end":{"line":12,"character":18}},` +
		`"newText":"count"}`
)

// unspelled is the text of the caveat about the shorthand property of Make, on line 9 counted
// from one, whose renamed side the definition does not show.
const unspelled = "shorthand property at a.fake:9, and the definition there does not show whether it " +
	"renames the key or the value"

// shorthandsOf returns the plan of the rename of the variable size of [lsptest.Literal] to count by
// a server in mode whose rename returns edits, and the texts that the plan writes at the
// shorthand properties of Make and of Copy, in that order.
func shorthandsOf(t *testing.T, mode lsptest.Mode, edits ...string) (engine.Result[edit.Change], []string) {
	t.Helper()
	root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Literal})
	renaming := `{"changes":{"{file}":[` + strings.Join(edits, ",") + `]}}`
	e := lsptest.Parsing(t, root, lsptest.Server(mode, lsptest.Renames(renaming)))
	got, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
		variable(t, root, "size"), edit.Args{edit.ArgNewName: "count"})
	assert.NoError(t, err, "Plan of the rename of size")
	shorthands := []int{
		strings.Index(lsptest.Literal, "{ size }") + len("{ "),
		strings.LastIndex(lsptest.Literal, "{ size }") + len("{ "),
	}
	var written []string
	for _, c := range got.Items {
		for _, one := range c.Edits {
			if slices.Contains(shorthands, one.Span.Start.Offset) {
				written = append(written, one.New)
			}
		}
	}
	return got, written
}

func TestShorthand(t *testing.T) {
	t.Parallel()

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the key before the new name at a shorthand property whose value it renames",
			func(t *testing.T) {
				t.Parallel()
				_, got := shorthandsOf(t, lsptest.Shorthand, countVariable, countMake, countCopy)
				assert.Equal(t, got, []string{"size: count", "size: count"}, "the texts of the shorthand properties")
			})

		t.Run("writes the new name before the value at a shorthand property whose key it renames",
			func(t *testing.T) {
				t.Parallel()
				_, got := shorthandsOf(t, lsptest.Shorthand, countField, countMake, countCopy)
				assert.Equal(t, got, []string{"count: size", "count: size"}, "the texts of the shorthand properties")
			})

		t.Run("writes the new name before the value at a shorthand property whose method it renames",
			func(t *testing.T) {
				t.Parallel()
				_, got := shorthandsOf(t, lsptest.Shorthand, countMethod, countMake, countCopy)
				assert.Equal(t, got[1], "count: size", "the text of the shorthand property of Copy")
			})

		t.Run("keeps the new name at a shorthand property whose key and value it renames", func(t *testing.T) {
			t.Parallel()
			plan, got := shorthandsOf(t, lsptest.Shorthand, countField, countVariable, countMake, countCopy)
			assert.Equal(t, got, []string{"count", "count"}, "the texts of the shorthand properties")
			assert.False(t, unrewritten(plan.Caveats, unspelled), "the plan has the caveat of a shorthand property")
		})

		t.Run("keeps an edit that writes the key of a shorthand property out", func(t *testing.T) {
			t.Parallel()
			_, got := shorthandsOf(t, lsptest.Shorthand, countVariable, spelledMake, countCopy)
			assert.Equal(t, got, []string{"size: count", "size: count"}, "the texts of the shorthand properties")
		})

		t.Run("returns a partial plan with a shorthand property whose sides it renames neither of",
			func(t *testing.T) {
				t.Parallel()
				plan, got := shorthandsOf(t, lsptest.Shorthand, countMake, countCopy)
				assert.Equal(t, got, []string{"count", "count"}, "the texts of the shorthand properties")
				assert.Equal(t, plan.Completeness, trust.ScopePartial, "the completeness of the plan")
				assert.True(t, unrewritten(plan.Caveats, unspelled), "the plan has the caveat of a shorthand property")
			})

		t.Run("returns a partial plan with a shorthand property whose renamed declaration the outline lacks",
			func(t *testing.T) {
				t.Parallel()
				plan, got := shorthandsOf(t, lsptest.Shorthand, countItem, countMake, countCopy)
				assert.Equal(t, got, []string{"count", "count"}, "the texts of the shorthand properties")
				assert.True(t, unrewritten(plan.Caveats, unspelled), "the plan has the caveat of a shorthand property")
			})

		t.Run("returns a partial plan with a shorthand property for a server without definitions",
			func(t *testing.T) {
				t.Parallel()
				plan, got := shorthandsOf(t, lsptest.Thin, countVariable, countMake, countCopy)
				assert.Equal(t, got, []string{"count", "count"}, "the texts of the shorthand properties")
				assert.Equal(t, plan.Completeness, trust.ScopePartial, "the completeness of the plan")
				assert.True(t, unrewritten(plan.Caveats, unspelled), "the plan has the caveat of a shorthand property")
			})
	})
}
