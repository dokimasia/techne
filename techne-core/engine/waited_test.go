// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
)

// pause is the length of the waits of the tests.
const pause = 20 * time.Millisecond

func TestWaited(t *testing.T) {
	t.Parallel()

	t.Run("Waiting", func(t *testing.T) {
		t.Parallel()

		t.Run("adds a wait to the Waited of the context", func(t *testing.T) {
			t.Parallel()
			ctx, waited := engine.Timing(t.Context())
			done := engine.Waiting(ctx)
			time.Sleep(pause)
			done()
			assert.InRange(t, waited.Total(), float64(pause), 1<<63, "the total is at least the wait")
		})

		t.Run("adds nothing without a Waited", func(t *testing.T) {
			t.Parallel()
			ctx, waited := engine.Timing(t.Context())
			engine.Waiting(engine.Untimed(ctx))()
			engine.Waiting(t.Context())()
			assert.Equal(t, waited.Total(), time.Duration(0), "the total after untimed waits")
		})

		t.Run("adds each of two waits", func(t *testing.T) {
			t.Parallel()
			ctx, waited := engine.Timing(t.Context())
			for range 2 {
				done := engine.Waiting(ctx)
				time.Sleep(pause)
				done()
			}
			assert.InRange(t, waited.Total(), float64(2*pause), 1<<63, "the total is at least both waits")
		})
	})

	t.Run("Total", func(t *testing.T) {
		t.Parallel()

		t.Run("returns zero before a wait", func(t *testing.T) {
			t.Parallel()
			_, waited := engine.Timing(t.Context())
			assert.Equal(t, waited.Total(), time.Duration(0), "the total")
		})
	})
}
