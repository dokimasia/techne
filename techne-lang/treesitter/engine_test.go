// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/treesitter"
)

// complete returns a declaration a case can spoil one field of. It
// carries no grammar: a grammar belongs to a language module, and this
// module must not depend on one.
func complete() lang.Declaration {
	return lang.Declaration{
		Language:   source.Language("fixture"),
		Extensions: []string{".fx"},
		Comment: lang.CommentStyle{
			Line: "// ", BlockOpen: "/*", BlockClose: "*/",
			Doc: []lang.DocStyle{{Open: "//"}},
		},
		IsTest:     func(string) bool { return false },
		Namespace:  func(p string) string { return strings.TrimSuffix(p, ".fx") },
		Visibility: visibility,
	}
}

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a language module that supplies no grammar", func(t *testing.T) {
			t.Parallel()
			// Every mistake in a language module should surface at
			// startup. A nil grammar would surface as a crash on the
			// first parse instead.
			for _, g := range []treesitter.Grammar{
				{},
				{Tags: "(identifier) @name"},
			} {
				_, err := treesitter.New(fstest.MapFS{}, complete(), g)
				assert.HasError(t, err,
					"a nil grammar would surface as a crash on the first parse rather than at startup")
			}
		})

		t.Run("refuses an incomplete declaration", func(t *testing.T) {
			t.Parallel()
			for name, spoil := range map[string]func(*lang.Declaration){
				"language":   func(d *lang.Declaration) { d.Language = "" },
				"extensions": func(d *lang.Declaration) { d.Extensions = nil },
				"Namespace":  func(d *lang.Declaration) { d.Namespace = nil },
				"Visibility": func(d *lang.Declaration) { d.Visibility = nil },
			} {
				d := complete()
				spoil(&d)
				_ = name
				_, err := treesitter.New(fstest.MapFS{}, d, treesitter.Grammar{})
				assert.HasError(t, err, "every mistake in a language module surfaces at startup")
			}
		})

		t.Run("refuses no filesystem to read from", func(t *testing.T) {
			t.Parallel()
			_, err := treesitter.New(nil, complete(), treesitter.Grammar{})
			assert.HasError(t, err, "an engine with nothing to read from can answer nothing")
		})

		t.Run("reports errors prefixed with the package name", func(t *testing.T) {
			t.Parallel()
			_, err := treesitter.New(fstest.MapFS{}, complete(), treesitter.Grammar{})
			assert.HasError(t, err, "a zero grammar is refused")
			assert.HasPrefix(t, err.Error(), "treesitter: ",
				"an error names the package it came from, so a caller can tell which layer refused")
		})
	})
}

// visibility is the fixture language's rule: a capitalised name is
// visible outside its unit.
func visibility(n string) sema.Visibility {
	if n != "" && n[0] >= 'A' && n[0] <= 'Z' {
		return sema.Exported
	}
	return sema.Unexported
}
