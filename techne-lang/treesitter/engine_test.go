// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/treesitter"
)

// complete returns a declaration without a grammar. The package tests use
// no grammar, because the grammars are in the language modules.
func complete() lang.Declaration {
	return lang.Declaration{
		Language:   "fixture",
		Extensions: []string{".fx"},
		Comment:    lang.CommentStyle{Line: "// ", Doc: []lang.DocStyle{{Open: "//"}}},
		IsTest:     lang.JavaScriptTest,
		Namespace:  lang.Stem,
		Visibility: lang.VisibilityByModifier,
	}
}

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a grammar without a language", func(t *testing.T) {
			t.Parallel()
			_, err := treesitter.New(fstest.MapFS{}, complete(), treesitter.Grammar{Tags: "(identifier) @name"})
			assert.HasError(t, err, "New")
		})

		incomplete := []struct {
			name  string
			spoil func(*lang.Declaration)
		}{
			{
				name:  "returns an error for a declaration without a language",
				spoil: func(d *lang.Declaration) { d.Language = "" },
			},
			{
				name:  "returns an error for a declaration without an extension",
				spoil: func(d *lang.Declaration) { d.Extensions = nil },
			},
			{
				name:  "returns an error for a declaration without Namespace",
				spoil: func(d *lang.Declaration) { d.Namespace = nil },
			},
			{
				name:  "returns an error for a declaration without Visibility",
				spoil: func(d *lang.Declaration) { d.Visibility = nil },
			},
		}
		for _, tt := range incomplete {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				d := complete()
				tt.spoil(&d)
				_, err := treesitter.New(fstest.MapFS{}, d, treesitter.Grammar{})
				assert.HasError(t, err, "New")
			})
		}

		t.Run("returns an error for a nil filesystem", func(t *testing.T) {
			t.Parallel()
			_, err := treesitter.New(nil, complete(), treesitter.Grammar{})
			assert.HasError(t, err, "New")
		})

		t.Run("prefixes its errors with the package name", func(t *testing.T) {
			t.Parallel()
			_, err := treesitter.New(fstest.MapFS{}, complete(), treesitter.Grammar{})
			assert.HasError(t, err, "New")
			assert.HasPrefix(t, err.Error(), "treesitter: ", "error")
		})
	})

	t.Run("Fidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("returns Syntactic for every role", func(t *testing.T) {
			t.Parallel()
			var e *treesitter.Engine
			for _, role := range engine.Roles() {
				assert.Equal(t, e.Fidelity(role), trust.Syntactic, role.String())
			}
		})
	})

	t.Run("Cost", func(t *testing.T) {
		t.Parallel()

		t.Run("returns CostParse for every role", func(t *testing.T) {
			t.Parallel()
			var e *treesitter.Engine
			for _, role := range engine.Roles() {
				assert.Equal(t, e.Cost(role), engine.CostParse, role.String())
			}
		})
	})
}
