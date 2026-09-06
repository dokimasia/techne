// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"bytes"
	"errors"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
)

// A walk is not the only way a file reaches an engine. A caller naming
// one goes straight to whatever reads it, so the rules a walk applies
// are asked about a single path here.
func TestReadable(t *testing.T) {
	t.Parallel()

	t.Run("a file of a size worth parsing", func(t *testing.T) {
		t.Parallel()

		t.Run("is read", func(t *testing.T) {
			t.Parallel()
			held := fstest.MapFS{
				"src/a.ts": {Data: bytes.Repeat([]byte("x"), lang.Largest)},
			}

			assert.NoError(t, lang.Readable(held, "src/a.ts"),
				"a file at the bound is inside it")
		})

		t.Run("is not read past the bound, and the reason names the size", func(t *testing.T) {
			t.Parallel()
			// Outlining one minified bundle with the TypeScript grammar
			// took 1.3s at a megabyte, 4.2s at two and 11.8s at 3.3. A
			// call with two seconds cannot spend eleven of them on one
			// file whose declarations are a bundler's.
			held := fstest.MapFS{
				"src/a.ts": {Data: bytes.Repeat([]byte("x"), lang.Largest+1)},
			}

			err := lang.Readable(held, "src/a.ts")

			var large lang.LargeError
			assert.True(t, errors.As(err, &large), "the refusal says what it was")
			assert.Equal(t, large.Size, int64(lang.Largest+1), "and how big the file is")
			assert.Contains(t, err.Error(), "src/a.ts", "and which file it was about")
		})
	})

	t.Run("a file the workspace calls generated", func(t *testing.T) {
		t.Parallel()

		t.Run("is refused however small it is", func(t *testing.T) {
			t.Parallel()
			// The write path is where this was missing: extracting a
			// function from a bundle under dist reached the engine
			// without passing a walk, and cost three seconds.
			held := fstest.MapFS{
				".gitignore":     {Data: []byte("dist\n")},
				"web/dist/a.ts":  {Data: []byte("export const a = 1")},
				"web/src/b.ts":   {Data: []byte("export const b = 2")},
				"web/.gitignore": {Data: []byte("!keep.ts\n")},
			}

			err := lang.Readable(held, "web/dist/a.ts")

			var generated lang.GeneratedError
			assert.True(t, errors.As(err, &generated), "the workspace's own rule decided it")
			assert.NoError(t, lang.Readable(held, "web/src/b.ts"),
				"and a file beside it that nothing names is read")
		})

		t.Run("is decided by the rules above it, not only the root", func(t *testing.T) {
			t.Parallel()
			held := fstest.MapFS{
				"one/.gitignore":     {Data: []byte("generated\n")},
				"one/generated/a.ts": {Data: []byte("")},
				"two/generated/b.ts": {Data: []byte("")},
			}

			assert.HasError(t, lang.Readable(held, "one/generated/a.ts"),
				"the rule in one/ speaks for its own subtree")
			assert.NoError(t, lang.Readable(held, "two/generated/b.ts"),
				"and for nothing beside it")
		})
	})

	t.Run("a path that is not there", func(t *testing.T) {
		t.Parallel()

		t.Run("is an error rather than a refusal", func(t *testing.T) {
			t.Parallel()
			// Told a file is generated, a caller looks at what wrote it.
			// Told it is missing, it looks at the path it typed. Both
			// are wrong answers to give for the other.
			err := lang.Readable(fstest.MapFS{}, "nowhere.ts")

			assert.HasError(t, err, "nothing to read")
			var generated lang.GeneratedError
			var large lang.LargeError
			assert.False(t, errors.As(err, &generated) || errors.As(err, &large),
				"and it is not one of this package's own refusals")
		})
	})
}
