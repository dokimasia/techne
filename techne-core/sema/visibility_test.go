// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
)

func TestVisibility(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("is unknown, not unexported", func(t *testing.T) {
			t.Parallel()
			var unset sema.Visibility
			assert.Equal(t, unset, sema.VisibilityUnknown,
				"an engine that cannot tell says so rather than claiming the commoner answer")
			assert.NotEqual(t, unset, sema.Unexported,
				"a caller filtering to public API must not silently drop what an engine was unsure about")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("is the wire form", func(t *testing.T) {
			t.Parallel()
			for v, want := range map[sema.Visibility]string{
				sema.VisibilityUnknown: "unknown",
				sema.Unexported:        "unexported",
				sema.Exported:          "exported",
			} {
				assert.Equal(t, v.String(), want, "the wire form reaches a caller, so it is pinned")
			}
		})

		t.Run("falls back to unknown outside the set", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.Visibility(200).String(), "unknown",
				"an invented value must not read as a claim about visibility")
		})
	})
}
