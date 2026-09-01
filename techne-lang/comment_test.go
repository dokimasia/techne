// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
)

// The styles below are the ones the language modules ship, repeated
// here so the rules that make them work are checked against the forms
// they were written for rather than against invented ones.
var (
	goStyle = lang.CommentStyle{
		Line: "// ", BlockOpen: "/*", BlockClose: "*/",
		Doc: []lang.DocStyle{
			{Open: "//"},
			{Open: "/*", Close: "*/"},
		},
	}

	rustStyle = lang.CommentStyle{
		Line: "// ", BlockOpen: "/*", BlockClose: "*/",
		Doc: []lang.DocStyle{
			{Open: "///"},
			{Open: "/**", Close: "*/", Continuation: " * "},
			{Open: "//!", Inside: true},
			{Open: "/*!", Close: "*/", Continuation: " * ", Inside: true},
		},
	}

	javaStyle = lang.CommentStyle{
		Line: "// ", BlockOpen: "/*", BlockClose: "*/",
		Doc: []lang.DocStyle{
			{Open: "/**", Close: "*/", Continuation: " * "},
			{Open: "///"},
		},
	}

	typescriptStyle = lang.CommentStyle{
		Line: "// ", BlockOpen: "/*", BlockClose: "*/",
		Doc: []lang.DocStyle{
			{Open: "/**", Close: "*/", Continuation: " * "},
		},
	}

	pythonStyle = lang.CommentStyle{
		Line: "# ",
		Doc: []lang.DocStyle{
			{Open: `"""`, Close: `"""`, Inside: true},
			{Open: `'''`, Close: `'''`, Inside: true},
			{Open: `r"""`, Close: `"""`, Inside: true},
			{Open: `r'''`, Close: `'''`, Inside: true},
		},
	}

	rubyStyle = lang.CommentStyle{
		Line: "# ", BlockOpen: "=begin", BlockClose: "=end",
		Doc: []lang.DocStyle{
			{Open: "#"},
			{Open: "=begin rdoc", Close: "=end"},
		},
	}
)

func TestCommentStyle(t *testing.T) {
	t.Parallel()

	t.Run("Documents", func(t *testing.T) {
		t.Parallel()

		t.Run("is the language's preferred form", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, rustStyle.Documents().Open, "///",
				"a tool writing documentation must write the form the language documents with")
			assert.Equal(t, javaStyle.Documents().Open, "/**",
				"a tool writing documentation must write the form the language documents with")
		})

		t.Run("says where the form is written", func(t *testing.T) {
			t.Parallel()
			assert.False(t, goStyle.Documents().Inside,
				"a tool must know whether to write above the declaration or in its body")
			assert.True(t, pythonStyle.Documents().Inside,
				"a tool must know whether to write above the declaration or in its body")
		})

		t.Run("falls back to the line comment where none is stated", func(t *testing.T) {
			t.Parallel()
			bare := lang.CommentStyle{Line: "-- "}
			assert.Equal(t, bare.Documents().Open, "-- ",
				"a language stating no documentation form still writes comments")
		})
	})

	t.Run("Documentation", func(t *testing.T) {
		t.Parallel()

		t.Run("reads a line form", func(t *testing.T) {
			t.Parallel()
			got, ok := goStyle.Documentation("// Digest hashes a file.")
			assert.True(t, ok, "Go documents with its ordinary line comment")
			assert.Equal(t, got, "Digest hashes a file.",
				"the delimiter and the one space after it are not part of the text")
		})

		t.Run("reads a block form and strips its continuation", func(t *testing.T) {
			t.Parallel()
			got, ok := javaStyle.Documentation("/**\n * Digest hashes a file.\n * It is safe to call twice.\n */")
			assert.True(t, ok, "the traditional Javadoc form is documentation")
			assert.Equal(t, got, "Digest hashes a file.\nIt is safe to call twice.",
				"the asterisk column is the form's own punctuation, not the author's text")
		})

		t.Run("takes the longest matching form", func(t *testing.T) {
			t.Parallel()
			got, ok := rustStyle.Documentation("/// Digest hashes a file.")
			assert.True(t, ok, "a language declaring several forms reads each as itself")
			assert.Equal(t, got, "Digest hashes a file.",
				"reading /// as // would leave a stray slash at the front of every line")
		})

		t.Run("reads Rust's inner form as documenting what encloses it", func(t *testing.T) {
			t.Parallel()
			got, ok := rustStyle.Documentation("//! This module hashes files.")
			assert.True(t, ok, "an inner form is documentation, of the item it is written inside")
			assert.Equal(t, got, "This module hashes files.",
				"the delimiter and the one space after it are not part of the text")
		})

		t.Run("refuses a comment that is not documentation", func(t *testing.T) {
			t.Parallel()
			_, ok := rustStyle.Documentation("// a note to the next reader")
			assert.False(t, ok,
				"Rust documents with /// and //!, so an ordinary comment is not documentation")
			_, ok = javaStyle.Documentation("/* a note to the next reader */")
			assert.False(t, ok,
				"Java documents with /** and ///, so an ordinary block comment is not documentation")
		})

		t.Run("refuses TypeScript's triple slash, which is a compiler directive", func(t *testing.T) {
			t.Parallel()
			_, ok := typescriptStyle.Documentation(`/// <reference types="node" />`)
			assert.False(t, ok,
				"/// documents in Java, Rust, C and C#, and instructs the compiler in TypeScript")
		})

		t.Run("reads a Python docstring, raw prefix included", func(t *testing.T) {
			t.Parallel()
			got, ok := pythonStyle.Documentation(`"""Hash a file."""`)
			assert.True(t, ok, "Python documents with a string literal rather than a comment")
			assert.Equal(t, got, "Hash a file.",
				"the quotes are the form's delimiters and not part of the text")

			raw, ok := pythonStyle.Documentation(`r"""Match \d+."""`)
			assert.True(t, ok, "a raw docstring is a docstring")
			assert.Equal(t, raw, `Match \d+.`,
				"the raw prefix is longer than the plain one, so it is matched first")
		})

		t.Run("reads only the Ruby block comment RDoc reads", func(t *testing.T) {
			t.Parallel()
			got, ok := rubyStyle.Documentation("=begin rdoc\nHash a file.\n=end")
			assert.True(t, ok, "RDoc reads a block comment only when it is flagged")
			assert.Equal(t, got, "Hash a file.",
				"the delimiters are the form's own and not part of the text")

			_, ok = rubyStyle.Documentation("=begin\nscratch notes\n=end")
			assert.False(t, ok,
				"an unflagged =begin block is a comment RDoc passes over, not documentation")
		})

		t.Run("keeps the indentation a code block depends on", func(t *testing.T) {
			t.Parallel()
			got, ok := goStyle.Documentation("//\tsum := Add(1, 2)")
			assert.True(t, ok, "Go documents with its ordinary line comment")
			assert.Equal(t, got, "\tsum := Add(1, 2)",
				"one space separates the delimiter from the text; the rest is the author's layout")
		})

		t.Run("finds nothing in a language stating no form", func(t *testing.T) {
			t.Parallel()
			var bare lang.CommentStyle
			_, ok := bare.Documentation("// anything")
			assert.False(t, ok,
				"a language that states no documentation form has none to recognise")
		})
	})
}

func TestDocStyle(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("matches nothing", func(t *testing.T) {
			t.Parallel()
			style := lang.CommentStyle{Doc: []lang.DocStyle{{}}}
			_, ok := style.Documentation("// anything")
			assert.False(t, ok,
				"a form with no opening delimiter would otherwise match every comment")
		})
	})

	t.Run("Close", func(t *testing.T) {
		t.Parallel()

		t.Run("is empty for a form running to the end of the line", func(t *testing.T) {
			t.Parallel()
			assert.Empty(t, rustStyle.Documents().Close,
				"a line form ends at the newline, so it names no closing delimiter")
		})
	})

	t.Run("Inside", func(t *testing.T) {
		t.Parallel()

		t.Run("separates the forms documenting what encloses them", func(t *testing.T) {
			t.Parallel()
			outer, inner := 0, 0
			for _, form := range rustStyle.Doc {
				if form.Inside {
					inner++
					continue
				}
				outer++
			}
			assert.Equal(t, outer, 2,
				"Rust documents what follows with /// and /**, and what encloses with //! and /*!")
			assert.Equal(t, inner, 2,
				"Rust documents what follows with /// and /**, and what encloses with //! and /*!")
		})
	})
}
