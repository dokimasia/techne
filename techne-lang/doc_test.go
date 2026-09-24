// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"bytes"
	"errors"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Registry", func(t *testing.T) {
		t.Parallel()

		t.Run("contains only the languages the caller registers", func(t *testing.T) {
			t.Parallel()
			full, empty := lang.NewRegistry(), lang.NewRegistry()
			assert.NoError(t, full.Register(engine.NewCatalog(), declared()), "Register")
			assert.Equal(t, full.Languages(), []source.Language{fixture}, "full")
			assert.Empty(t, empty.Languages(), "empty")
		})
	})

	t.Run("Readable", func(t *testing.T) {
		t.Parallel()

		t.Run("agrees with Walk on every file", func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{
				".gitignore":       {Data: []byte("dist/\n*.gen.fx\n")},
				"src/a.fx":         {Data: []byte("a")},
				"src/b.gen.fx":     {Data: []byte("b")},
				"dist/c.fx":        {Data: []byte("c")},
				"src/big.fx":       {Data: bytes.Repeat([]byte("x"), lang.Largest+1)},
				"src/deep/d.fx":    {Data: []byte("d")},
				"src/deep/.keep":   {Data: []byte("")},
				"src/deep/e.gen.x": {Data: []byte("e")},
			}
			files, err := lang.Walk(fsys, ".", claimed)
			assert.NoError(t, err, "Walk")
			for name := range fsys {
				if !lang.Claims(name, claimed) {
					continue
				}
				err := lang.Readable(fsys, source.Path(name))
				_, large := errors.AsType[lang.LargeError](err)
				assert.Equal(t, err == nil, slices.Contains(files.Read, source.Path(name)), name+" in Read")
				assert.Equal(t, large, slices.Contains(files.Unread, source.Path(name)), name+" in Unread")
			}
		})
	})
}
