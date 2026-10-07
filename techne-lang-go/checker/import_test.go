// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"maps"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	golang "go.dokimi.dev/techne/lang/go"
)

// The importers of the package example.com/p/clock, on lines counted from zero: use.go on its
// line 2, and the test files use_test.go of use and clock_test.go of the external test package
// of clock on their line 5.
const (
	importer       = "use/use.go:2"
	testedImporter = "use/use_test.go:5"
	externalTest   = "clock/clock_test.go:5"
)

// cgoFile is a file that cgo compiles, which imports the package clock on its line 5, counted
// from zero.
const cgoFile = "package c\n\n// int one(void) { return 1; }\nimport \"C\"\n\nimport \"example.com/p/clock\"\n\n" +
	"func One() int { return int(C.one()) + clock.Now() }\n"

func TestImport(t *testing.T) {
	t.Parallel()

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each line that imports the package of an import path", func(t *testing.T) {
			t.Parallel()
			got, err := importersOf(t, ".", "example.com/p/clock", true)
			assert.NoError(t, err, "Relate of the importers of example.com/p/clock")
			assert.Equal(t, places(got.Items), []string{externalTest, importer, testedImporter},
				"the sites of the imports")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("returns the importing file as the far end of each import", func(t *testing.T) {
			t.Parallel()
			got, err := importersOf(t, ".", "example.com/p/clock", false)
			assert.NoError(t, err, "Relate of the importers of example.com/p/clock")
			assert.Equal(t, got.Items[0].To, sema.File(golang.Language, "use/use.go"),
				"the far end of the first import")
			assert.Equal(t, got.Items[0].Via, `import "example.com/p/clock"`, "the line of the first import")
		})

		t.Run("returns the importers of the package that the name of a package under the scope names",
			func(t *testing.T) {
				t.Parallel()
				got, err := importersOf(t, "clock", "clock", true)
				assert.NoError(t, err, "Relate of the importers of clock")
				assert.Equal(t, places(got.Items), []string{externalTest, importer, testedImporter},
					"the sites of the imports")
			})

		t.Run("returns the importers of the package of the file of a declaration", func(t *testing.T) {
			t.Parallel()
			got, err := importersOf(t, "clock/clock.go", "Now", true)
			assert.NoError(t, err, "Relate of the importers of the package of Now")
			assert.Equal(t, places(got.Items), []string{externalTest, importer, testedImporter},
				"the sites of the imports")
		})

		t.Run("returns the importers of a package outside the workspace", func(t *testing.T) {
			t.Parallel()
			got, err := importersOf(t, ".", "testing", true)
			assert.NoError(t, err, "Relate of the importers of testing")
			assert.Equal(t, places(got.Items), []string{"clock/clock_test.go:3", "use/use_test.go:3"},
				"the sites of the imports")
		})

		t.Run("returns a total answer without imports for the directory of a command", func(t *testing.T) {
			t.Parallel()
			got, err := importersOf(t, "cmd/tool", "tool", true)
			assert.NoError(t, err, "Relate of the importers of cmd/tool")
			assert.Empty(t, got.Items, "the sites of the imports")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("returns a total answer without imports for the import path of a command", func(t *testing.T) {
			t.Parallel()
			got, err := importersOf(t, ".", "example.com/p/cmd/tool", true)
			assert.NoError(t, err, "Relate of the importers of example.com/p/cmd/tool")
			assert.Empty(t, got.Items, "the sites of the imports")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("returns the span of the path of an import", func(t *testing.T) {
			t.Parallel()
			got, err := importersOf(t, ".", "example.com/p/clock", false)
			assert.NoError(t, err, "Relate of the importers of example.com/p/clock")
			at := got.Items[0].At
			assert.Equal(t, clocks()["use/use.go"][at.Start.Offset:at.End.Offset], `"example.com/p/clock"`,
				"the text of the span")
			assert.Equal(t, []int{at.Start.Column, at.End.Column}, []int{7, 28}, "the columns of the span")
		})

		t.Run("returns the imports in the order of their paths", func(t *testing.T) {
			t.Parallel()
			got, err := importersIn(t, "example.com/p/clock", map[string]string{
				"x/b.go":      "package x\n\nimport \"example.com/p/clock\"\n\nvar _ = clock.Now\n",
				"x/a_test.go": "package x\n\nimport \"example.com/p/clock\"\n\nvar _ = clock.Now\n",
			})
			assert.NoError(t, err, "Relate of the importers of example.com/p/clock")
			assert.Equal(t, places(got.Items), []string{"x/a_test.go:2", "x/b.go:2"}, "the sites of the imports")
		})

		t.Run("returns each import once for a package with a test file", func(t *testing.T) {
			t.Parallel()
			got, err := importersIn(t, "example.com/p/clock", map[string]string{
				"x/a.go":      "package x\n",
				"x/b.go":      "package x\n\nimport \"example.com/p/clock\"\n\nvar _ = clock.Now\n",
				"x/b_test.go": "package x\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n",
			})
			assert.NoError(t, err, "Relate of the importers of example.com/p/clock")
			assert.Equal(t, places(got.Items), []string{"x/b.go:2"}, "the sites of the imports")
		})

		t.Run("returns the line of an import in a file with a line directive", func(t *testing.T) {
			t.Parallel()
			got, err := importersIn(t, "example.com/p/clock", map[string]string{
				"x/x.go": "package x\n\n//line other.go:100\nimport \"example.com/p/clock\"\n\nvar _ = clock.Now\n",
			})
			assert.NoError(t, err, "Relate of the importers of example.com/p/clock")
			assert.Equal(t, places(got.Items), []string{"x/x.go:3"}, "the sites of the imports")
		})

		t.Run("returns the line of an import in a file that cgo compiles", func(t *testing.T) {
			t.Parallel()
			got, err := importersIn(t, "example.com/p/clock", map[string]string{"c/c.go": cgoFile})
			assert.NoError(t, err, "Relate of the importers of example.com/p/clock")
			assert.Equal(t, places(got.Items), []string{"c/c.go:5"}, "the sites of the imports")
			assert.Equal(t, got.Items[0].Via, `import "example.com/p/clock"`, "the line of the import")
		})

		t.Run("returns the imports of a file before an import that does not parse", func(t *testing.T) {
			t.Parallel()
			got, err := importersIn(t, "example.com/p/clock", map[string]string{
				"bad/bad.go": "package bad\n\nimport \"example.com/p/clock\"\nimport \"unterminated\n\nvar _ = clock.Now\n",
			})
			assert.NoError(t, err, "Relate of the importers of example.com/p/clock")
			assert.Equal(t, places(got.Items), []string{"bad/bad.go:2"}, "the sites of the imports")
		})

		t.Run("leaves out the imports of the files that cgo generates", func(t *testing.T) {
			t.Parallel()
			got, err := importersIn(t, "unsafe", map[string]string{"c/c.go": cgoFile})
			assert.NoError(t, err, "Relate of the importers of unsafe")
			assert.Empty(t, got.Items, "the sites of the imports")
		})

		t.Run("leaves out a test file for a request without tests", func(t *testing.T) {
			t.Parallel()
			got, err := importersOf(t, ".", "example.com/p/clock", false)
			assert.NoError(t, err, "Relate of the importers of example.com/p/clock")
			assert.Equal(t, places(got.Items), []string{importer}, "the sites of the imports")
		})

		t.Run("returns ErrRefuse for a name of two packages under the scope", func(t *testing.T) {
			t.Parallel()
			_, err := importersOf(t, ".", "clock", true)
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Relate")
			assert.Contains(t, err.Error(), "example.com/p/clock, example.com/p/other/clock", "the error of Relate")
		})

		t.Run("returns ErrDecline for a scope that contains no package of the name", func(t *testing.T) {
			t.Parallel()
			_, err := importersOf(t, ".", "nothing", true)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
		})
	})
}

// clocks returns a module with two packages named clock, at clock and at other/clock, a package
// use that imports the first and whose test imports it too, a package elsewhere that imports the
// second, and the command cmd/tool. The first clock has an external test package that imports
// it.
func clocks() map[string]string {
	return map[string]string{
		"clock/clock.go": "package clock\n\nfunc Now() int { return 0 }\n",
		"clock/clock_test.go": "package clock_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/p/clock\"\n)\n\n" +
			"func TestNow(t *testing.T) { _ = clock.Now() }\n",
		"cmd/tool/main.go":     "package main\n\nfunc main() {}\n",
		"other/clock/clock.go": "package clock\n\nfunc Then() int { return 1 }\n",
		"use/use.go":           "package use\n\nimport \"example.com/p/clock\"\n\nfunc Use() int { return clock.Now() }\n",
		"use/use_test.go": "package use\n\nimport (\n\t\"testing\"\n\n\t\"example.com/p/clock\"\n)\n\n" +
			"func TestUse(t *testing.T) { _ = clock.Now() }\n",
		"elsewhere/elsewhere.go": "package elsewhere\n\nimport \"example.com/p/other/clock\"\n\n" +
			"func Else() int { return clock.Then() }\n",
	}
}

// importersOf returns the answer of the checker over [clocks] to imported-by of the name in the
// scope, with or without the test files.
func importersOf(t *testing.T, scope, name string, tests bool) (engine.Result[sema.Relation], error) {
	t.Helper()
	of := sema.NewID(golang.Language, "", name, sema.KindUnknown)
	return serving(t, clocks()).Relate(t.Context(), engine.Request{Scope: source.Path(scope), Tests: tests},
		of, sema.ImportedBy)
}

// importersIn returns the answer of the checker to imported-by of the name over the workspace,
// with the test files, in a module of the package clock of [clocks] and files.
func importersIn(t *testing.T, name string, files map[string]string) (engine.Result[sema.Relation], error) {
	t.Helper()
	module := map[string]string{"clock/clock.go": clocks()["clock/clock.go"]}
	maps.Copy(module, files)
	of := sema.NewID(golang.Language, "", name, sema.KindUnknown)
	return serving(t, module).Relate(t.Context(), engine.Request{Scope: engine.Root, Tests: true}, of,
		sema.ImportedBy)
}
