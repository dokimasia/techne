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

// workspace returns a type with a member, a function that uses the type, a file that uses
// both, and a file of another language.
func workspace() fstest.MapFS {
	return fstest.MapFS{
		"src/store.mock": {Data: []byte(
			";; Store maps a name to an item.\n" +
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

// built returns an engine over [workspace] with opts applied.
func built(t *testing.T, opts ...mock.Option) *mock.Engine {
	t.Helper()
	return over(t, workspace(), opts...)
}

// over returns an engine over fsys with opts applied.
func over(t *testing.T, fsys fstest.MapFS, opts ...mock.Option) *mock.Engine {
	t.Helper()
	e, err := mock.New(fsys, mock.Declaration(mock.Language), opts...)
	assert.NoError(t, err, "New")
	return e
}

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a declaration without a language", func(t *testing.T) {
			t.Parallel()
			_, err := mock.New(workspace(), lang.Declaration{})
			assert.HasError(t, err, "New without a language")
		})

		t.Run("returns an error for a nil filesystem", func(t *testing.T) {
			t.Parallel()
			_, err := mock.New(nil, mock.Declaration(mock.Language))
			assert.HasError(t, err, "New without a filesystem")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a name per language", func(t *testing.T) {
			t.Parallel()
			alpha, err := mock.New(workspace(), mock.Declaration("alpha"))
			assert.NoError(t, err, "New of alpha")
			beta, err := mock.New(workspace(), mock.Declaration("beta"))
			assert.NoError(t, err, "New of beta")
			assert.Equal(t, alpha.Name(), "mock/alpha", "the name of alpha")
			assert.Equal(t, beta.Name(), "mock/beta", "the name of beta")
		})
	})

	t.Run("Fidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("returns resolved by default", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, built(t).Fidelity(engine.RoleOutline), trust.Resolved, "the tier of outline")
		})

		t.Run("returns the tier that At sets", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, built(t, mock.At(trust.Syntactic)).Fidelity(engine.RoleOutline), trust.Syntactic,
				"the tier of outline")
		})
	})

	t.Run("Cost", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an analysis by default", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, built(t).Cost(engine.RoleRelate), engine.CostAnalyze, "the cost of relate")
		})

		t.Run("returns the cost that Costing sets", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, built(t, mock.Costing(engine.CostSession)).Cost(engine.RoleRelate), engine.CostSession,
				"the cost of relate")
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil by default", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, built(t).Available(t.Context()), "Available")
		})

		t.Run("returns the reason that Missing sets", func(t *testing.T) {
			t.Parallel()
			err := built(t, mock.Missing("nothing is installed")).Available(t.Context())
			assert.HasError(t, err, "Available of a missing language")
			assert.Contains(t, err.Error(), "nothing is installed", "the reason of the error")
		})
	})
}
