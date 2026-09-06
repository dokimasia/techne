// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// faulty is the fixture with the word this language will not take
// written into it. It and [content] are the pair a parse gate cannot
// tell apart: both are the language they claim to be, and one of them
// does not compile.
const faulty = "package a\n\ntype Store struct {\n\tsize undeclared\n}\n"

func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("judges content the workspace does not hold", func(t *testing.T) {
			t.Parallel()
			// The protocol has no request that takes content and does not
			// need one: a server analyses the buffers it is given, and an
			// editor gives it text nobody has written all day.
			e := serving(t, modeCompiles, map[string]string{"a.fake": content})
			got, err := e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(faulty)})

			assert.NoError(t, err, "checking content that is not on disk succeeds")
			assert.Length(t, got.Items, 1, "the server objected to what it was shown")
			assert.Equal(t, got.Items[0].Diagnostic.Severity, diag.SeverityError,
				"and graded it a fault")
			assert.Equal(t, got.Completeness, trust.ScopeTotal,
				"having reported on every file it was shown")
		})

		t.Run("finds nothing wrong with content that is whole", func(t *testing.T) {
			t.Parallel()
			e := serving(t, modeCompiles, map[string]string{"a.fake": content})
			got, err := e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(content)})

			assert.NoError(t, err, "checking succeeds")
			assert.Empty(t, got.Items, "nothing is wrong with it")
		})

		t.Run("carries the one obvious fix beside the fault", func(t *testing.T) {
			t.Parallel()
			// The server that reports a fault offers what resolves it and
			// is being asked already. A caller that has to work the edit
			// out from the message pays a round trip for something the
			// server had.
			e := serving(t, modeCompiles, map[string]string{"a.fake": content})
			got, err := e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(faulty)})

			assert.NoError(t, err, "checking succeeds")
			assert.Length(t, got.Items[0].Fix, 1, "one change resolves it")
			assert.Equal(t, got.Items[0].Fix[0].Edits[0].New, "declared",
				"which is what the server said to write")
		})

		t.Run("measures the fix against the content it judged", func(t *testing.T) {
			t.Parallel()
			// A gate judges what a change would produce, and the fault
			// and the fix are both written in that. Measured against the
			// file on disk the edit names other bytes, and writing over
			// them still parses.
			e := serving(t, modeCompiles, map[string]string{"a.fake": content})
			got, err := e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(faulty)})

			assert.NoError(t, err, "checking succeeds")
			at := got.Items[0].Fix[0].Edits[0].Span
			assert.Equal(t, faulty[at.Start.Offset:at.End.Offset], "undeclared",
				"the range covers the word the server would replace")
		})

		t.Run("puts the files back before it returns", func(t *testing.T) {
			t.Parallel()
			// The buffer shown to the server is not a file, and a gate is
			// not an apply. A server left holding content nobody wrote
			// answers every later question about code that is nowhere.
			e := serving(t, modeCompiles, map[string]string{"a.fake": content})
			_, err := e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(faulty)})
			assert.NoError(t, err, "checking succeeds")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "a.fake"}, nil)
			assert.NoError(t, err, "asking about the file succeeds")
			assert.Empty(t, got.Items, "and the file on disk has nothing wrong with it")
		})

		t.Run("leaves alone what this language does not claim", func(t *testing.T) {
			t.Parallel()
			// One change can touch several languages and each engine
			// judges its own. Refusing what it cannot read would have the
			// first engine asked veto every mixed change.
			e := serving(t, modeCompiles, map[string]string{"a.fake": content})
			_, err := e.Check(t.Context(), map[source.Path][]byte{"notes.md": []byte("# notes\n")})

			assert.ErrorIs(t, err, engine.ErrDecline,
				"nothing here is this language, so another engine gets a turn")
		})

		t.Run("says nothing about a file the change takes away", func(t *testing.T) {
			t.Parallel()
			// A path with no content is one the change removes. There is
			// nothing to analyse and nothing to object to.
			e := serving(t, modeCompiles, map[string]string{"a.fake": content})
			_, err := e.Check(t.Context(), map[source.Path][]byte{"a.fake": nil})

			assert.ErrorIs(t, err, engine.ErrDecline, "there is nothing left to judge")
		})

		t.Run("declines where the server said nothing about what it was shown", func(t *testing.T) {
			t.Parallel()
			// A gate saying content is clean is the claim a caller acts
			// on by writing it, and a server that reported nothing has
			// not made it. Declined rather than answered short, so the
			// parser beside this one gets a turn: answered short, Scala
			// would have no gate at all where it could still have had a
			// grammar's.
			e := serving(t, modeUngated, map[string]string{"a.fake": content})
			_, err := e.Check(t.Context(), map[source.Path][]byte{"a.fake": []byte(faulty)})

			assert.ErrorIs(t, err, engine.ErrDecline, "so something weaker judges it instead")
		})
	})
}
