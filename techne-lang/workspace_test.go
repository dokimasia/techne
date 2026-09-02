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

		t.Run("is false for a tree that is nowhere", func(t *testing.T) {
			t.Parallel()
			// A language server is a process that opens files by name.
			// Pointed at a tree that was never written, it opens nothing
			// and answers about nothing, and being answered about nothing
			// is the failure this project is built to avoid reporting as
			// an answer.
			held := lang.Workspace{FS: fstest.MapFS{"a.go": {Data: []byte("package a")}}}
			assert.False(t, held.OnDisk(),
				"a tree with no path cannot host a process that opens files by name")
		})

		t.Run("is true for a tree with a path", func(t *testing.T) {
			t.Parallel()
			held := lang.Workspace{FS: fstest.MapFS{}, Root: t.TempDir()}
			assert.True(t, held.OnDisk(), "a directory can")
		})

		t.Run("is false for the zero value", func(t *testing.T) {
			t.Parallel()
			assert.False(t, lang.Workspace{}.OnDisk(), "a workspace nobody set names nothing")
		})
	})
}
