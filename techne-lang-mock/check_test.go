// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang/mock"
)

func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("finds nothing wrong with content that is this language", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Check(t.Context(), map[source.Path][]byte{
				"a.mock": []byte("type Store\n  use Store\n"),
			})
			assert.NoError(t, err, "checking content this engine claims succeeds")
			assert.Empty(t, got.Items, "four shapes, all of them this language")
		})

		t.Run("reports content that stopped being it", func(t *testing.T) {
			t.Parallel()
			// This is what the write path's gate runs, so a change that
			// wrote over the wrong bytes is refused before it lands.
			got, err := built(t).Check(t.Context(), map[source.Path][]byte{
				"a.mock": []byte("type Store\n)]}%\n"),
			})
			assert.NoError(t, err, "content that does not parse is an answer, not a fault")
			assert.Length(t, got.Items, 1, "the line that is not this language is reported")
			assert.Equal(t, got.Items[0].Diagnostic.Severity, diag.SeverityError,
				"a file that stopped parsing is an error")
			assert.Equal(t, got.Items[0].Diagnostic.Snippet, ")]}%",
				"and the line comes with it, for a reader with no filesystem")
			assert.Empty(t, got.Items[0].Fix,
				"nothing here is one obvious change that resumes it")
		})

		t.Run("leaves alone a file it does not claim", func(t *testing.T) {
			t.Parallel()
			_, err := built(t).Check(t.Context(), map[source.Path][]byte{"a.other": []byte("x")})
			assert.ErrorIs(t, err, engine.ErrDecline,
				"one change can touch several languages, and each engine judges its own")
		})

		t.Run("says nothing about a file the change takes away", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Check(t.Context(), map[source.Path][]byte{"a.mock": nil})
			assert.NoError(t, err, "a path with no content is one being removed")
			assert.Empty(t, got.Items, "and there is nothing to object to")
		})
	})

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("finds a reference naming nothing", func(t *testing.T) {
			t.Parallel()
			// The one thing that parses and is still wrong, which is
			// what a build gate catches and a parse gate does not.
			e, err := mock.New(broken(), mock.Declaration(mock.Language))
			assert.NoError(t, err, "an engine builds")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "verifying a scope that exists succeeds")
			assert.Length(t, got.Items, 1, "one use names nothing")
			assert.Contains(t, got.Items[0].Diagnostic.Message, "Absent",
				"and the message says which")
			assert.Empty(t, got.Items[0].Fix,
				"which declaration was meant is not something the engine knows, so it guesses none")
		})

		t.Run("finds nothing wrong with a workspace that resolves", func(t *testing.T) {
			t.Parallel()
			got, err := built(t).Verify(t.Context(), engine.Request{Scope: "src"}, nil)
			assert.NoError(t, err, "verifying succeeds")
			assert.Empty(t, got.Items, "every use names something")
		})

		t.Run("runs nothing for a suite it does not have", func(t *testing.T) {
			t.Parallel()
			// What a language whose linter is not installed answers.
			e, err := mock.New(broken(), mock.Declaration(mock.Language))
			assert.NoError(t, err, "an engine builds")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, []string{"nothing-like-it"})
			assert.NoError(t, err, "a suite this language has not is not a fault")
			assert.Empty(t, got.Items, "and nothing ran")

			named, err := e.Verify(t.Context(), engine.Request{Scope: "."}, []string{mock.Suite})
			assert.NoError(t, err, "and the one it has runs when named")
			assert.NotEmpty(t, named.Items, "finding what it finds")
		})
	})
}

// broken is a workspace holding a use that names nothing.
func broken() fstest.MapFS {
	return fstest.MapFS{"a.mock": {Data: []byte("func Uses\n  use Absent\n")}}
}
