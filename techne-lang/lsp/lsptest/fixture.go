// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// Language is the language that the scripted server serves.
const Language source.Language = "fake"

// Extension is the suffix of the files that [Language] claims.
const Extension = ".fake"

// Content is the file that the answers of the script describe. Line 2 declares Store, line 3
// its field size, line 6 the method Get, and line 8 the function After, which calls Get.
// Store is used on line 6 at character 9 and on line 8 at character 28.
const Content = `package a

type Store struct {
	size int
}

func (s *Store) Get() int { return s.size }

func After() int { return (&Store{}).Get() }
`

// Emoji is a file with an emoji outside the Basic Multilingual Plane on line 2, before the
// declaration of Störe. The name starts at byte 19 and at UTF-16 code unit 17 of the line,
// which is byte 30 of the file.
const Emoji = "package a\n\nvar 🌍 = 1; type Störe struct{}\n"

// Twins is a file with the types Store and Cache on lines 2 and 6, which each declare a method
// Get, on lines 4 and 8. The name of each method starts at character 16 of its line.
const Twins = "package a\n\ntype Store struct{}\n\nfunc (s *Store) Get() int { return 1 }\n\n" +
	"type Cache struct{}\n\nfunc (c *Cache) Get() int { return 2 }\n"

// Bundle returns a file with a package clause on line 0 and n+1 functions on line 1: F0 to
// F<n-1>, then After, which calls F0. A minifier writes a program on one line this way. The
// Minified mode reads the declarations and the uses of F0 from this file.
func Bundle(n int) string {
	var out strings.Builder
	out.WriteString("package a\n")
	for i := range n {
		fmt.Fprintf(&out, "func F%d() int { return %d }; ", i, i)
	}
	out.WriteString("func After() int { return F0() }\n")
	return out.String()
}

// Broken is the word that the Compiles, Unbound and WorkspaceDiagnostics modes report as an
// error.
const Broken = "undeclared"

// Faulty is the declaration of Store from [Content] with [Broken] as the type of size. It
// parses and does not compile.
const Faulty = "package a\n\ntype Store struct {\n\tsize " + Broken + "\n}\n"

// Placeholder is the name that the Extracts and Commands modes give the function they
// extract.
const Placeholder = "newFunction"

// Test is the base name of the one file that [Declaration] treats as a test.
const Test = "a_test" + Extension

// Declaration returns the declaration of [Language]. A name that starts with an upper-case
// letter is exported. The namespace of a file is its directory.
func Declaration() lang.Declaration {
	return lang.Declaration{
		Language:   Language,
		Extensions: []string{Extension},
		Comment:    lang.CommentStyle{Line: "// "},
		IsTest:     func(p string) bool { return path.Base(p) == Test },
		Namespace:  path.Dir,
		Visibility: func(name string) sema.Visibility {
			if first, _ := utf8.DecodeRuneInString(name); unicode.IsUpper(first) {
				return sema.Exported
			}
			return sema.Unexported
		},
	}
}

// Workspace writes files into a new temporary directory and returns the directory. Each key
// is a slash-separated path relative to the directory. The test fails if a file cannot be
// written.
func Workspace(tb testing.TB, files map[string]string) string {
	tb.Helper()
	root := tb.TempDir()
	for name, content := range files {
		at := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
			tb.Fatalf("lsptest: create the directory of %s: %v", name, err)
		}
		if err := os.WriteFile(at, []byte(content), 0o644); err != nil {
			tb.Fatalf("lsptest: write %s: %v", name, err)
		}
	}
	return root
}
