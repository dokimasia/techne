// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"context"
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
)

// closing is a fake that implements Closer. Close counts its calls in closed
// and returns err.
type closing struct {
	fake
	closed *int
	err    error
}

func (c closing) Close(context.Context) error {
	*c.closed++
	return c.err
}

var _ engine.Closer = closing{}

func TestClose(t *testing.T) {
	t.Parallel()

	t.Run("Close", func(t *testing.T) {
		t.Parallel()

		t.Run("closes every engine that implements Closer", func(t *testing.T) {
			t.Parallel()
			first, second := 0, 0
			c := catalog(t, closing{name: "first", closed: &first}, closing{name: "second", closed: &second})
			assert.NoError(t, c.Close(t.Context()), "Close")
			assert.Equal(t, []int{first, second}, []int{1, 1}, "Close calls")
		})

		t.Run("returns nil for engines without Closer", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, catalog(t, fake{name: "parser"}).Close(t.Context()), "Close")
		})

		t.Run("closes the remaining engines after a failure", func(t *testing.T) {
			t.Parallel()
			first, second := 0, 0
			c := catalog(t,
				closing{name: "first", closed: &first, err: errors.New("first: stuck")},
				closing{name: "second", closed: &second})
			assert.HasError(t, c.Close(t.Context()), "Close")
			assert.Equal(t, second, 1, "second Close calls")
		})

		t.Run("returns every failure", func(t *testing.T) {
			t.Parallel()
			one, two := errors.New("first: stuck"), errors.New("second: stuck")
			first, second := 0, 0
			c := catalog(t,
				closing{name: "first", closed: &first, err: one},
				closing{name: "second", closed: &second, err: two})
			err := c.Close(t.Context())
			assert.ErrorIs(t, err, one, "first failure")
			assert.ErrorIs(t, err, two, "second failure")
		})

		t.Run("returns nil for an empty catalogue", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, engine.NewCatalog().Close(t.Context()), "Close")
		})
	})
}
