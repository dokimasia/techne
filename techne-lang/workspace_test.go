// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
)

func TestWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("OnDisk", func(t *testing.T) {
		t.Parallel()

		t.Run("returns false for a tree without a root", func(t *testing.T) {
			t.Parallel()
			w := lang.Workspace{FS: fstest.MapFS{"a.go": {Data: []byte("package a")}}}
			assert.False(t, w.OnDisk(), "OnDisk")
		})

		t.Run("returns true for a tree with a root", func(t *testing.T) {
			t.Parallel()
			w := lang.Workspace{FS: fstest.MapFS{}, Root: t.TempDir()}
			assert.True(t, w.OnDisk(), "OnDisk")
		})

		t.Run("returns false when zero", func(t *testing.T) {
			t.Parallel()
			assert.False(t, lang.Workspace{}.OnDisk(), "OnDisk")
		})
	})
}
