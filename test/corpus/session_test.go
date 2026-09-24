// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package corpus_test

import (
	"io"
	"os"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/test/corpus"
	"go.dokimi.dev/techne/tool"
)

func TestSession(t *testing.T) {
	t.Parallel()
	root, err := corpus.ModuleRoot(t.Context())
	assert.NoError(t, err, "ModuleRoot")
	binary, err := corpus.Binary(t.Context(), root, t.TempDir())
	assert.NoError(t, err, "Binary")

	// session starts techne over a workspace of the mock language, which
	// serves every tool without a language server.
	session := func(t *testing.T) *corpus.Session {
		t.Helper()
		dir := t.TempDir()
		writeFile(t, dir, "a.mock",
			";; Store maps a name to an item.\ntype Store\n  field size\nfunc New\n  use Store\n")
		s, err := corpus.Start(t.Context(), binary, dir, append(os.Environ(), "TECHNE_MOCK=1"), io.Discard)
		assert.NoError(t, err, "Start")
		t.Cleanup(func() { _ = s.Close() })
		return s
	}

	t.Run("Call", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes the answer of a tool", func(t *testing.T) {
			t.Parallel()
			var answer tool.Answer
			arguments := map[string]any{"scope": "a.mock", "private": true}
			_, err := session(t).Call(t.Context(), "outline", arguments, &answer)
			assert.NoError(t, err, "Call of outline")
			stored := slices.ContainsFunc(answer.Items, func(d tool.Declaration) bool { return d.Name == "Store" })
			assert.True(t, stored, "the outline of a.mock")
		})

		t.Run("returns an error for an unknown tool", func(t *testing.T) {
			t.Parallel()
			var answer tool.Answer
			_, err := session(t).Call(t.Context(), "no.such.tool", map[string]any{}, &answer)
			assert.HasError(t, err, "Call of an unknown tool")
		})
	})

	t.Run("Calls", func(t *testing.T) {
		t.Parallel()

		t.Run("records the tool of each call", func(t *testing.T) {
			t.Parallel()
			s := session(t)
			for _, name := range []string{"outline", "search"} {
				var answer tool.Matches
				_, err := s.Call(t.Context(), name, map[string]any{"scope": "a.mock", "text": "Store"}, &answer)
				assert.NoError(t, err, "Call of "+name)
			}
			var got []string
			for _, c := range s.Calls() {
				got = append(got, c.Tool)
			}
			assert.Equal(t, got, []string{"outline", "search"}, "the tools of the calls")
		})

		t.Run("marks the calls after Warm as warm", func(t *testing.T) {
			t.Parallel()
			s := session(t)
			var answer tool.Answer
			_, err := s.Call(t.Context(), "outline", map[string]any{"scope": "a.mock"}, &answer)
			assert.NoError(t, err, "the cold Call")
			s.Warm()
			returned, err := s.Call(t.Context(), "outline", map[string]any{"scope": "a.mock"}, &answer)
			assert.NoError(t, err, "the warm Call")
			assert.True(t, returned.Warm, "the warmth of the returned call")
			var warm []bool
			for _, c := range s.Calls() {
				warm = append(warm, c.Warm)
			}
			assert.Equal(t, warm, []bool{false, true}, "the warmth of the recorded calls")
		})

		t.Run("records the provenance of each answer", func(t *testing.T) {
			t.Parallel()
			s := session(t)
			var answer tool.Answer
			returned, err := s.Call(t.Context(), "outline", map[string]any{"scope": "a.mock"}, &answer)
			assert.NoError(t, err, "Call of outline")
			assert.Equal(t, returned.Fidelity, answer.Provenance.Fidelity, "the fidelity of the call")
			assert.Equal(t, returned.Completeness, answer.Provenance.Completeness, "the completeness of the call")
			assert.NotEmpty(t, returned.Fidelity, "the fidelity of the mock answer")
		})
	})
}
