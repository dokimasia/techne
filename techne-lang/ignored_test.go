// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// A fixed list of directory names cannot name build output: dist and
// build are generated in one project and hand-written in the next. The
// repository has already answered the question, so the answer is read
// from it.
func TestIgnored(t *testing.T) {
	t.Parallel()

	t.Run("what a workspace says is not its source", func(t *testing.T) {
		t.Parallel()

		t.Run("is left out of a walk", func(t *testing.T) {
			t.Parallel()
			// Measured over one TypeScript monorepo: 680 of the 1,725
			// files a walk returned were a storybook bundle and
			// fifty-four dist directories, every one of them named by
			// the project's own .gitignore. Some were minified bundles
			// of three megabytes, which is what a search was spending
			// its time on.
			held := fstest.MapFS{
				".gitignore":            {Data: []byte("dist\nstorybook-static\n")},
				"src/a.ts":              {Data: []byte("")},
				"dist/a.ts":             {Data: []byte("")},
				"ui/dist/b.ts":          {Data: []byte("")},
				"storybook-static/c.ts": {Data: []byte("")},
			}
			got, err := lang.FilesIn(held, ".", []string{".ts"})

			assert.NoError(t, err, "walking a workspace with a .gitignore succeeds")
			assert.Equal(t, got, []source.Path{"src/a.ts"},
				"a name with no slash covers every directory of that name, at any depth")
		})

		t.Run("is the file in each directory, applying below it", func(t *testing.T) {
			t.Parallel()
			// A monorepo has one per package, and each speaks for its own
			// subtree and for nothing above it.
			held := fstest.MapFS{
				"one/.gitignore":     {Data: []byte("generated\n")},
				"one/a.ts":           {Data: []byte("")},
				"one/generated/b.ts": {Data: []byte("")},
				"two/generated/c.ts": {Data: []byte("")},
			}
			got, err := lang.FilesIn(held, ".", []string{".ts"})

			assert.NoError(t, err, "walking succeeds")
			assert.Equal(t, got, []source.Path{"one/a.ts", "two/generated/c.ts"},
				"one package's rule does not reach into another's")
		})

		t.Run("is decided by the last pattern that matches", func(t *testing.T) {
			t.Parallel()
			// Which is git's own precedence, and what makes a negation
			// able to re-include something an earlier line excluded.
			held := fstest.MapFS{
				".gitignore":  {Data: []byte("*.gen.ts\n!keep.gen.ts\n")},
				"a.gen.ts":    {Data: []byte("")},
				"keep.gen.ts": {Data: []byte("")},
				"b.ts":        {Data: []byte("")},
			}
			got, err := lang.FilesIn(held, ".", []string{".ts"})

			assert.NoError(t, err, "walking succeeds")
			assert.Equal(t, got, []source.Path{"b.ts", "keep.gen.ts"},
				"the negation has the last word over the pattern above it")
		})

		t.Run("is anchored where the pattern carries a slash", func(t *testing.T) {
			t.Parallel()
			// A slash fixes the pattern to the directory the file sits
			// in. Without one it names something at any depth, and the
			// difference is what keeps /build from covering ui/build.
			held := fstest.MapFS{
				".gitignore":    {Data: []byte("/build\n")},
				"build/a.ts":    {Data: []byte("")},
				"ui/build/b.ts": {Data: []byte("")},
			}
			got, err := lang.FilesIn(held, ".", []string{".ts"})

			assert.NoError(t, err, "walking succeeds")
			assert.Equal(t, got, []source.Path{"ui/build/b.ts"},
				"the anchored pattern named the one at the root and no other")
		})

		t.Run("is a directory alone where the pattern ends in a slash", func(t *testing.T) {
			t.Parallel()
			held := fstest.MapFS{
				".gitignore": {Data: []byte("out/\n")},
				"out/a.ts":   {Data: []byte("")},
				"out.ts":     {Data: []byte("")},
			}
			got, err := lang.FilesIn(held, ".", []string{".ts"})

			assert.NoError(t, err, "walking succeeds")
			assert.Equal(t, got, []source.Path{"out.ts"},
				"a file of that name is not the directory the pattern named")
		})

		t.Run("crosses directories only where the pattern says so", func(t *testing.T) {
			t.Parallel()
			// A star stops at a separator and two cross any number of
			// them, which is what the wildcards mean everywhere else
			// they are written.
			held := fstest.MapFS{
				".gitignore":                   {Data: []byte("ui/*/gen\nsrc/**/vendorish\n")},
				"ui/one/gen/a.ts":              {Data: []byte("")},
				"ui/one/two/gen/b.ts":          {Data: []byte("")},
				"src/deep/down/vendorish/c.ts": {Data: []byte("")},
				"src/d.ts":                     {Data: []byte("")},
			}
			got, err := lang.FilesIn(held, ".", []string{".ts"})

			assert.NoError(t, err, "walking succeeds")
			assert.Equal(t, got, []source.Path{"src/d.ts", "ui/one/two/gen/b.ts"},
				"one star stayed inside a segment and two crossed as many as there were")
		})

		t.Run("applies the rules above a scope a caller narrowed to", func(t *testing.T) {
			t.Parallel()
			// A walk that started inside the package would never read the
			// root's file, and generated code under it would come back as
			// the package's own source.
			held := fstest.MapFS{
				".gitignore":   {Data: []byte("dist\n")},
				"ui/a.ts":      {Data: []byte("")},
				"ui/dist/b.ts": {Data: []byte("")},
			}
			got, err := lang.FilesIn(held, "ui", []string{".ts"})

			assert.NoError(t, err, "walking one package succeeds")
			assert.Equal(t, got, []source.Path{"ui/a.ts"},
				"what the workspace said still holds inside it")
		})

		t.Run("is said for a file a caller names, not only a directory", func(t *testing.T) {
			t.Parallel()
			// The bundle that made this worth measuring is a file. A
			// scope that names one went straight past the rules and was
			// parsed: three megabytes of minified JavaScript, twenty-five
			// seconds, and no answer.
			held := fstest.MapFS{
				".gitignore":     {Data: []byte("dist\n")},
				"dist/bundle.ts": {Data: []byte("")},
			}
			_, err := lang.FilesIn(held, "dist/bundle.ts", []string{".ts"})

			var generated lang.GeneratedError
			assert.True(t, errors.As(err, &generated), "a file is judged as a directory is")
		})

		t.Run("is said rather than walked when a caller names one", func(t *testing.T) {
			t.Parallel()
			// A dependency directory is exempt when a caller names one,
			// because reading a dependency is a thing to want. This is
			// not the same: it is the project's own statement that the
			// directory holds output, and parsing it costs whatever the
			// build wrote — one minified bundle of three megabytes took
			// eight seconds.
			//
			// Said rather than answered empty, because the two are
			// different facts. Told nothing is there, a caller concludes
			// the directory is empty; told the workspace calls it
			// generated, it knows to look at what wrote it.
			held := fstest.MapFS{
				".gitignore": {Data: []byte("dist\n")},
				"dist/a.ts":  {Data: []byte("")},
			}
			_, err := lang.FilesIn(held, "dist", []string{".ts"})

			var generated lang.GeneratedError
			assert.True(t, errors.As(err, &generated), "the reason is one a caller can act on")
			assert.Equal(t, string(generated.Scope), "dist", "and names what it was about")
		})

		t.Run("is nothing where the workspace says nothing", func(t *testing.T) {
			t.Parallel()
			// A directory that is not a repository has no rules, and a
			// walk over it returns what it holds.
			held := fstest.MapFS{"a.ts": {Data: []byte("")}, "dist/b.ts": {Data: []byte("")}}
			got, err := lang.FilesIn(held, ".", []string{".ts"})

			assert.NoError(t, err, "walking succeeds")
			assert.Equal(t, got, []source.Path{"a.ts", "dist/b.ts"},
				"nothing declared it, so nothing is skipped")
		})

		t.Run("reads a comment and a blank line as neither", func(t *testing.T) {
			t.Parallel()
			held := fstest.MapFS{
				".gitignore": {Data: []byte("# what the build writes\n\n   \ndist\n")},
				"dist/a.ts":  {Data: []byte("")},
				"b.ts":       {Data: []byte("")},
			}
			got, err := lang.FilesIn(held, ".", []string{".ts"})

			assert.NoError(t, err, "walking succeeds")
			assert.Equal(t, got, []source.Path{"b.ts"}, "the one pattern there was still applied")
		})
	})
}
