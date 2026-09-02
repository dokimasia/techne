// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/mock"
)

// workspace is what every case here reads: a container with a member, a
// function that refers to it, and a second file that refers to both.
func workspace() fstest.MapFS {
	return fstest.MapFS{
		"src/store.mock": {Data: []byte(
			";; Store holds items by name.\n" +
				"type Store\n" +
				"  field size\n" +
				"  method Get\n" +
				"    use Store\n" +
				"\n" +
				"func New\n" +
				"  use Store\n")},
		"src/client.mock": {Data: []byte(
			"func Client\n" +
				"  use New\n" +
				"  use Store\n")},
		"notes.md": {Data: []byte("not this language\n")},
	}
}

// built returns an engine over the workspace.
func built(t *testing.T, opts ...mock.Option) *mock.Engine {
	t.Helper()
	e, err := mock.New(workspace(), mock.Declaration(mock.Language), opts...)
	assert.NoError(t, err, "an engine builds from a declaration and a filesystem")
	return e
}

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a declaration naming no language", func(t *testing.T) {
			t.Parallel()
			_, err := mock.New(workspace(), lang.Declaration{})
			assert.HasError(t, err, "a language without a name routes nowhere")
		})

		t.Run("refuses no filesystem", func(t *testing.T) {
			t.Parallel()
			_, err := mock.New(nil, mock.Declaration(mock.Language))
			assert.HasError(t, err, "an engine with nothing to read answers about nothing")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("carries the language, so instances do not collide", func(t *testing.T) {
			t.Parallel()
			// One adapter serves every mock language. A constant name
			// would have a catalogue refuse the second and a provenance
			// unable to say which answered.
			one, err := mock.New(workspace(), mock.Declaration("alpha"))
			assert.NoError(t, err, "an engine builds")
			two, err := mock.New(workspace(), mock.Declaration("beta"))
			assert.NoError(t, err, "and so does a second")
			assert.NotEqual(t, one.Name(), two.Name(), "two languages are two engines")
		})
	})

	t.Run("Fidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("is resolved unless a caller lowers it", func(t *testing.T) {
			t.Parallel()
			// Honest rather than convenient: within this language a use
			// names a declaration and the workspace is read whole.
			assert.Equal(t, built(t).Fidelity(engine.RoleOutline), trust.Resolved,
				"what it claims is what it does")
			assert.Equal(t, built(t, mock.At(trust.Syntactic)).Fidelity(engine.RoleOutline),
				trust.Syntactic,
				"and a workspace can hold one that only parses, so the refusal is real")
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("reports a language that cannot run, with the reason", func(t *testing.T) {
			t.Parallel()
			// A missing tool is a different problem from a missing
			// capability, and a caller can act on the first.
			assert.NoError(t, built(t).Available(t.Context()),
				"a language with nothing outside the process always runs")
			assert.HasError(t, built(t, mock.Missing("nothing is installed")).Available(t.Context()),
				"and one declared missing says so rather than answering")
		})
	})
}
