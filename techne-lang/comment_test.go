// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
)

// The comment styles of seven language modules.
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

	csharpStyle = lang.CommentStyle{
		Line: "// ", BlockOpen: "/*", BlockClose: "*/",
		Doc: []lang.DocStyle{
			{Open: "///", Element: "summary"},
			{Open: "/**", Close: "*/", Continuation: " * ", Element: "summary"},
		},
	}
)

func TestComment(t *testing.T) {
	t.Parallel()

	t.Run("Documents", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the first documentation form", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, rustStyle.Documents().Open, "///", "Rust")
			assert.Equal(t, javaStyle.Documents().Open, "/**", "Java")
		})

		t.Run("returns a form that reports its placement", func(t *testing.T) {
			t.Parallel()
			assert.False(t, goStyle.Documents().Inside, "Go")
			assert.True(t, pythonStyle.Documents().Inside, "Python")
		})

		t.Run("returns the line comment for a style without documentation forms", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, lang.CommentStyle{Line: "-- "}.Documents().Open, "-- ", "Open")
		})
	})

	t.Run("Documentation", func(t *testing.T) {
		t.Parallel()

		read := []struct {
			name  string
			style lang.CommentStyle
			give  string
			want  string
		}{
			{
				name:  "reads a line form",
				style: goStyle,
				give:  "// Digest hashes a file.",
				want:  "Digest hashes a file.",
			},
			{
				name:  "removes the continuation of a block form",
				style: javaStyle,
				give:  "/**\n * Digest hashes a file.\n * It is safe to call twice.\n */",
				want:  "Digest hashes a file.\nIt is safe to call twice.",
			},
			{
				name:  "reads the longest matching form",
				style: rustStyle,
				give:  "/// Digest hashes a file.",
				want:  "Digest hashes a file.",
			},
			{
				name:  "reads an inner form",
				style: rustStyle,
				give:  "//! This module hashes files.",
				want:  "This module hashes files.",
			},
			{name: "reads a Python docstring", style: pythonStyle, give: `"""Hash a file."""`, want: "Hash a file."},
			{name: "reads a raw Python docstring", style: pythonStyle, give: `r"""Match \d+."""`, want: `Match \d+.`},
			{
				name:  "reads a flagged Ruby block",
				style: rubyStyle,
				give:  "=begin rdoc\nHash a file.\n=end",
				want:  "Hash a file.",
			},
			{
				name:  "keeps the indentation after the separating space",
				style: goStyle,
				give:  "//\tsum := Add(1, 2)",
				want:  "\tsum := Add(1, 2)",
			},
		}
		for _, tt := range read {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, ok := tt.style.Documentation(tt.give)
				assert.True(t, ok, "documentation")
				assert.Equal(t, got, tt.want, "text")
			})
		}

		refused := []struct {
			name  string
			style lang.CommentStyle
			give  string
		}{
			{name: "returns false for a plain line comment in Rust", style: rustStyle, give: "// a note"},
			{name: "returns false for a plain block comment in Java", style: javaStyle, give: "/* a note */"},
			{
				name:  "returns false for a TypeScript triple-slash directive",
				style: typescriptStyle,
				give:  `/// <reference types="node" />`,
			},
			{name: "returns false for an unflagged Ruby block", style: rubyStyle, give: "=begin\nscratch notes\n=end"},
			{name: "returns false for a style without documentation forms", give: "// anything"},
			{
				name:  "returns false for a form without an opening delimiter",
				style: lang.CommentStyle{Doc: []lang.DocStyle{{}}},
				give:  "// anything",
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, ok := tt.style.Documentation(tt.give)
				assert.False(t, ok, "documentation")
			})
		}
	})

	t.Run("Document", func(t *testing.T) {
		t.Parallel()

		written := []struct {
			name  string
			style lang.CommentStyle
			give  string
			want  string
		}{
			{name: "writes a Go line form", style: goStyle, give: "Store keeps items.", want: "// Store keeps items."},
			{
				name:  "writes a Rust line form",
				style: rustStyle,
				give:  "Store keeps items.",
				want:  "/// Store keeps items.",
			},
			{
				name:  "writes a Ruby line form",
				style: rubyStyle,
				give:  "Store keeps items.",
				want:  "# Store keeps items.",
			},
			{
				name:  "writes a Java block form",
				style: javaStyle,
				give:  "Store keeps items.",
				want:  "/**\n * Store keeps items.\n */",
			},
			{
				name:  "writes a TypeScript block form",
				style: typescriptStyle,
				give:  "Store keeps items.",
				want:  "/**\n * Store keeps items.\n */",
			},
			{
				name:  "writes a Python docstring",
				style: pythonStyle,
				give:  "Store keeps items.",
				want:  `"""Store keeps items."""`,
			},
			{
				name:  "writes a blank line of a line form as the bare marker",
				style: goStyle,
				give:  "One.\n\nTwo.",
				want:  "// One.\n//\n// Two.",
			},
			{
				name:  "writes a blank line of a block form as the bare continuation",
				style: javaStyle,
				give:  "One.\n\nTwo.",
				want:  "/**\n * One.\n *\n * Two.\n */",
			},
			{
				name:  "writes a blank line of a docstring as an empty line",
				style: pythonStyle,
				give:  "One.\n\nTwo.",
				want:  "\"\"\"One.\n\nTwo.\n\"\"\"",
			},
			{
				name:  "writes the text of a C# form inside its summary element",
				style: csharpStyle,
				give:  "Store keeps items.",
				want:  "/// <summary>\n/// Store keeps items.\n/// </summary>",
			},
			{
				name:  "writes text that starts with a tag without another element",
				style: csharpStyle,
				give:  "<summary>Store keeps items.</summary>",
				want:  "/// <summary>Store keeps items.</summary>",
			},
		}
		for _, tt := range written {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, tt.style.Document(tt.give, ""), tt.want, "comment")
			})
		}

		t.Run("indents every line", func(t *testing.T) {
			t.Parallel()
			for line := range strings.SplitSeq(javaStyle.Document("One.\n\nTwo.", "    "), "\n") {
				assert.HasPrefix(t, line, "    ", line)
			}
		})

		t.Run("writes no indentation on an empty docstring line", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, pythonStyle.Document("One.\n\nTwo.", "    "), "\n\n", "comment")
		})

		t.Run("returns text through Documentation for a block form", func(t *testing.T) {
			t.Parallel()
			for _, style := range []lang.CommentStyle{javaStyle, typescriptStyle, pythonStyle} {
				for _, text := range []string{"One.", "One.\n\nTwo.", "One.\n    indented\nTwo."} {
					for _, indent := range []string{"", "    ", "\t\t"} {
						got, ok := style.Documentation(style.Document(text, indent))
						assert.True(t, ok, "documentation")
						assert.Equal(t, got, text, "text")
					}
				}
			}
		})

		t.Run("returns each line through Documentation for a line form", func(t *testing.T) {
			t.Parallel()
			for _, style := range []lang.CommentStyle{goStyle, rustStyle, rubyStyle, csharpStyle} {
				var read []string
				for line := range strings.SplitSeq(style.Document("One.\n\nTwo.", "  "), "\n") {
					got, ok := style.Documentation(line)
					assert.True(t, ok, line)
					read = append(read, got)
				}
				assert.Equal(t, style.Unwrapped(strings.Join(read, "\n")), "One.\n\nTwo.", "text")
			}
		})
	})

	t.Run("Unwrapped", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			style lang.CommentStyle
			give  string
			want  string
		}{
			{
				name:  "returns the text inside an element that makes up the documentation",
				style: csharpStyle,
				give:  "<summary>\nStore keeps items.\n</summary>",
				want:  "Store keeps items.",
			},
			{
				name:  "returns the text of an element on one line",
				style: csharpStyle,
				give:  "<summary>Store keeps items.</summary>",
				want:  "Store keeps items.",
			},
			{
				name:  "keeps the markup of documentation with a second element",
				style: csharpStyle,
				give:  "<summary>Gets it.</summary>\n<param name=\"key\">The key.</param>",
				want:  "<summary>Gets it.</summary>\n<param name=\"key\">The key.</param>",
			},
			{
				name:  "keeps the markup of two summaries",
				style: csharpStyle,
				give:  "<summary>One.</summary><summary>Two.</summary>",
				want:  "<summary>One.</summary><summary>Two.</summary>",
			},
			{
				name:  "returns documentation unchanged for a form without an element",
				style: goStyle,
				give:  "<summary>Store keeps items.</summary>",
				want:  "<summary>Store keeps items.</summary>",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, tt.style.Unwrapped(tt.give), tt.want, "the unwrapped documentation")
			})
		}
	})
}
