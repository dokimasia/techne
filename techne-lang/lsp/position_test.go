// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
)

// The protocol counts lines and UTF-16 code units and this vocabulary
// counts bytes, so every position crossing the boundary is converted,
// in both directions.
//
// The direction out of the protocol is pinned by the outline case that
// reads "Störe" rather than "e St\xc3". The direction into it is pinned
// here, and needs a server that objects: a request carrying the wrong
// column is answered about whatever is at that column instead, which
// looks like a correct answer about something else.
func TestPosition(t *testing.T) {
	t.Parallel()

	t.Run("a position handed to a server", func(t *testing.T) {
		t.Parallel()

		t.Run("counts characters the way the protocol does", func(t *testing.T) {
			t.Parallel()
			// The name begins at byte 19 of its line and at UTF-16 unit
			// 17, because the emoji before it is four bytes and two
			// units. The fake refuses to rename anything but unit 17.
			e := serving(t, "strict", map[string]string{"a.fake": unicode})
			outlined, err := e.Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "the case can find what it points at")
			assert.Length(t, outlined.Items, 1, "the file declares one thing")

			got, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
				edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSpan, Span: outlined.Items[0].Span},
				edit.Args{edit.ArgNewName: "Vault"})

			assert.NoError(t, err,
				"the byte offset was converted, so the server was asked about the name")
			assert.Length(t, got.Items, 1, "and answered with the file to change")
		})

		t.Run("is worked out from an offset alone", func(t *testing.T) {
			t.Parallel()
			// A caller that read a position out of an answer this package
			// gave it has an offset and no line. Ignoring it would point
			// every such request at the start of the file.
			e := serving(t, "strict", map[string]string{"a.fake": unicode})

			got, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
				edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSpan, Span: source.Span{
					Path:  "a.fake",
					Start: source.Position{Offset: 30},
				}},
				edit.Args{edit.ArgNewName: "Vault"})

			assert.NoError(t, err, "an offset with no line still names the position")
			assert.Length(t, got.Items, 1, "and reaches the same declaration")
		})

		t.Run("is worked out from a line and a column alone", func(t *testing.T) {
			t.Parallel()
			// A caller looking at an editor has a line and a column and
			// no byte count, which the port's own contract allows.
			e := serving(t, "strict", map[string]string{"a.fake": unicode})

			got, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
				edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSpan, Span: source.Span{
					Path:  "a.fake",
					Start: source.Position{Line: 2, Column: 19},
				}},
				edit.Args{edit.ArgNewName: "Vault"})

			assert.NoError(t, err, "a line and a column name the position too")
			assert.Length(t, got.Items, 1, "and reach the same declaration")
		})

		t.Run("is refused where it names the wrong column", func(t *testing.T) {
			t.Parallel()
			// The guard the three cases above rely on. Without it they
			// would pass against a server that accepted anything, and
			// prove nothing about the conversion.
			e := serving(t, "strict", map[string]string{"a.fake": unicode})

			_, err := e.Plan(t.Context(), engine.Request{Scope: "a.fake"},
				edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSpan, Span: source.Span{
					Path:  "a.fake",
					Start: source.Position{Line: 2, Column: 0},
				}},
				edit.Args{edit.ArgNewName: "Vault"})

			assert.ErrorIs(t, err, engine.ErrRefuse,
				"a position naming something else is told there is nothing there")
		})
	})

	t.Run("a span the protocol named", func(t *testing.T) {
		t.Parallel()

		t.Run("is cut from the bytes it covers", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, "", map[string]string{"a.fake": content}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "outlining succeeds")
			one, _ := named(got.Items, "size")
			assert.Equal(t, one.Snippet, "size int", "the source between the two offsets")
		})

		t.Run("carries a line and a column beside the offset", func(t *testing.T) {
			t.Parallel()
			// A caller rendering an answer reports a line, and counting
			// one out of an offset needs the file it came from.
			got, err := serving(t, "", map[string]string{"a.fake": content}).
				Outline(t.Context(), engine.Request{Scope: "a.fake"})

			assert.NoError(t, err, "outlining succeeds")
			one, _ := named(got.Items, "After")
			assert.Equal(t, one.Span.Start.Line, 8, "the line the server named")
			assert.Equal(t, one.Span.Start.Column, 0, "and the column, counted in bytes")
		})
	})
}
