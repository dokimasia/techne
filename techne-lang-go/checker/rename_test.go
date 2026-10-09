// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	golang "go.dokimi.dev/techne/lang/go"
)

// renamedFiles returns a module whose files that the build constraints exclude use the
// declarations of the package p. The excluded files are files of freebsd and plan9, which no
// system that runs the tests builds by default:
//
//   - p declares Old, the struct T with the field Field and the method Method, the struct E,
//     which the struct S embeds and to which the struct P embeds a pointer, F, an alias of E,
//     which the struct A embeds, and the struct U with the field Value of type E
//   - p/p_freebsd.go, a file of freebsd, uses Old, Field, Method, the field E of an S and of a
//     P, the field F of an A and the field Value of a U, and p/p_plan9.go, a file of plan9, uses
//     Old twice
//   - p/local_freebsd.go declares a local variable Old, and p/nocgo_freebsd.go, which a build of
//     freebsd includes with cgo off, uses Old
//   - p/never.go, which no build includes, and p/cgo_freebsd.go, which imports C, use Old
//   - p/broken_freebsd.go uses Old and has a type error
//   - p/clash_freebsd.go declares the methods Method and Other of its own type, and
//     p/iface_freebsd.go an interface with the methods Method and Other, and both use the Method
//     of T
//   - p/sized_freebsd.go declares a function Method, an interface with a method Size and a
//     method Area, and calls the Method of T
//   - q imports p and uses Old, q/q_freebsd.go declares a method Old and calls Old through p,
//     and q/q_plan9.go uses Old through p
//   - every file of bare is a file of freebsd, and bare uses Old through p
//   - odd/odd_plan9.go uses Old in a package of a name that the other files of odd do not have,
//     so the build of plan9 does not load odd
//   - twin/twin.go declares Twin in every build but plan9, and twin/twin_plan9.go declares a Twin
//     of plan9, which twin/use_plan9.go uses
func renamedFiles() map[string]string {
	return map[string]string{
		"p/p.go": "package p\n\n// Old returns one.\nfunc Old() int { return 1 }\n\n" +
			"// T has a field and a method.\ntype T struct{ Field int }\n\n" +
			"// Method returns two.\nfunc (T) Method() int { return 2 }\n\n" +
			"// E is embedded.\ntype E struct{}\n\n// S embeds E.\ntype S struct{ E }\n\n" +
			"// P embeds a pointer to E.\ntype P struct{ *E }\n\n" +
			"// F is E under another name.\ntype F = E\n\n// A embeds F.\ntype A struct{ F }\n\n" +
			"// U has a field of type E.\ntype U struct{ Value E }\n",
		"p/p_freebsd.go": "package p\n\nvar _ = Old()\n\nvar _ = T{Field: 1}.Method()\n\nvar _ = S{}.E\n\n" +
			"var _ = P{}.E\n\nvar _ = A{}.F\n\nvar _ = U{}.Value\n",
		"p/p_plan9.go":        "package p\n\nvar _ = Old\n\nvar _ = Old()\n",
		"p/local_freebsd.go":  "package p\n\nfunc local() int {\n\tOld := 3\n\treturn Old\n}\n",
		"p/nocgo_freebsd.go":  "//go:build !cgo\n\npackage p\n\nvar _ = Old\n",
		"p/never.go":          never + "package p\n\nvar _ = Old\n",
		"p/cgo_freebsd.go":    "package p\n\nimport \"C\"\n\nvar _ = Old\n",
		"p/broken_freebsd.go": "package p\n\nvar _ = Old\n\nvar _ int = \"broken\"\n",
		"p/clash_freebsd.go": "package p\n\ntype other struct{}\n\nfunc (other) Method() int { return 3 }\n\n" +
			"func (other) Other() int { return 5 }\n\nvar _ = T{}.Method\n",
		"p/iface_freebsd.go": "package p\n\ntype actor interface {\n\tMethod() int\n\tOther() int\n}\n\n" +
			"var _ = T{}.Method\n",
		"p/sized_freebsd.go": "package p\n\ntype sizer interface{ Size() int }\n\ntype box struct{}\n\n" +
			"func (box) Area() int { return T{}.Method() }\n\nfunc Method() int { return 4 }\n",
		"q/q.go": "package q\n\nimport \"example.com/p/p\"\n\n// Q is the Old of p.\nvar Q = p.Old\n",
		"q/q_freebsd.go": "package q\n\nimport \"example.com/p/p\"\n\ntype r struct{}\n\n" +
			"func (r) Old() int { return p.Old() }\n",
		"q/q_plan9.go":         "package q\n\nimport \"example.com/p/p\"\n\nvar _ = p.Old\n",
		"bare/bare_freebsd.go": "package bare\n\nimport \"example.com/p/p\"\n\nvar _ = p.Old()\n",
		"odd/odd.go":           "package odd\n",
		"odd/odd_plan9.go":     "package other\n\nimport \"example.com/p/p\"\n\nvar _ = p.Old\n",
		"twin/twin.go":         "//go:build !plan9\n\npackage twin\n\n// Twin returns one.\nfunc Twin() int { return 1 }\n",
		"twin/twin_plan9.go":   "package twin\n\n// Twin returns two.\nfunc Twin() int { return 2 }\n",
		"twin/use_plan9.go":    "package twin\n\nvar _ = Twin\n",
		"lone/lone.go":         "package lone\n\n// Lone returns three.\nfunc Lone() int { return 3 }\n",
		"lone/lone_freebsd.go": "package lone\n",
	}
}

// workedFiles returns a workspace of two modules, a, which its go.work file lists, and c, which
// it does not. The package p of each declares Old. The package w of each has only files of
// freebsd and uses Old through p.
func workedFiles() map[string]string {
	return map[string]string{
		"go.work":          "go 1.24\n\nuse ./a\n",
		"a/go.mod":         "module example.com/a\n\ngo 1.24\n",
		"a/p/p.go":         "package p\n\n// Old returns one.\nfunc Old() int { return 1 }\n",
		"a/w/w_freebsd.go": "package w\n\nimport \"example.com/a/p\"\n\nvar _ = p.Old()\n",
		"c/go.mod":         "module example.com/c\n\ngo 1.24\n",
		"c/p/p.go":         "package p\n\n// Old returns two.\nfunc Old() int { return 2 }\n",
		"c/w/w_freebsd.go": "package w\n\nimport \"example.com/c/p\"\n\nvar _ = p.Old()\n",
	}
}

func TestRename(t *testing.T) {
	t.Parallel()

	t.Run("Renamed", func(t *testing.T) {
		t.Parallel()

		t.Run("renames each use of the declaration in an excluded file of another port", func(t *testing.T) {
			t.Parallel()
			root := workspace(t, renamedFiles())
			got, _, err := over(t, root).Renamed(t.Context(), renaming("p/p.go", "Old", sema.KindFunction), "New")
			assert.NoError(t, err, "Renamed of Old")
			assert.Equal(t, afterRename(t, root, got), map[source.Path]string{
				"bare/bare_freebsd.go": "package bare\n\nimport \"example.com/p/p\"\n\nvar _ = p.New()\n",
				"p/nocgo_freebsd.go":   "//go:build !cgo\n\npackage p\n\nvar _ = New\n",
				"p/p_plan9.go":         "package p\n\nvar _ = New\n\nvar _ = New()\n",
				"p/p_freebsd.go": "package p\n\nvar _ = New()\n\nvar _ = T{Field: 1}.Method()\n\nvar _ = S{}.E\n\n" +
					"var _ = P{}.E\n\nvar _ = A{}.F\n\nvar _ = U{}.Value\n",
				"q/q_plan9.go": "package q\n\nimport \"example.com/p/p\"\n\nvar _ = p.New\n",
				"q/q_freebsd.go": "package q\n\nimport \"example.com/p/p\"\n\ntype r struct{}\n\n" +
					"func (r) Old() int { return p.New() }\n",
			}, "the excluded files after the rename")
		})

		t.Run("returns the changes in the order of their paths", func(t *testing.T) {
			t.Parallel()
			got, _, err := serving(t, renamedFiles()).Renamed(t.Context(),
				renaming("p/p.go", "Old", sema.KindFunction), "New")
			assert.NoError(t, err, "Renamed of Old")
			assert.Equal(t, changed(engine.Result[edit.Change]{Items: got}), []source.Path{
				"bare/bare_freebsd.go", "p/nocgo_freebsd.go", "p/p_freebsd.go", "p/p_plan9.go", "q/q_freebsd.go",
				"q/q_plan9.go",
			}, "the paths of the changes")
		})

		t.Run("names the excluded files that it leaves as they are in the caveat", func(t *testing.T) {
			t.Parallel()
			_, got, err := serving(t, renamedFiles()).Renamed(t.Context(),
				renaming("p/p.go", "Old", sema.KindFunction), "New")
			assert.NoError(t, err, "Renamed of Old")
			assert.Equal(t, pathsOf(got, trust.CaveatUnrewritten),
				[]source.Path{"odd/odd_plan9.go", "p/broken_freebsd.go", "p/cgo_freebsd.go", "p/never.go"},
				"the files of the caveat")
		})

		t.Run("counts the files of the caveat in its note", func(t *testing.T) {
			t.Parallel()
			_, got, err := serving(t, renamedFiles()).Renamed(t.Context(),
				renaming("p/p.go", "Old", sema.KindFunction), "New")
			assert.NoError(t, err, "Renamed of Old")
			assert.Equal(t, notes(got), []string{"the plan does not rename the declaration in 4 files that the " +
				"build constraints exclude from the build that the plan reads, such as odd/odd_plan9.go"},
				"the notes of the caveats")
		})

		t.Run("renames a method that an excluded file calls through a value", func(t *testing.T) {
			t.Parallel()
			root := workspace(t, renamedFiles())
			got, _, err := over(t, root).Renamed(t.Context(), renaming("p/p.go", "T.Method", sema.KindMethod), "Act")
			assert.NoError(t, err, "Renamed of Method")
			assert.Equal(t, afterRename(t, root, got), map[source.Path]string{
				"p/p_freebsd.go": "package p\n\nvar _ = Old()\n\nvar _ = T{Field: 1}.Act()\n\nvar _ = S{}.E\n\n" +
					"var _ = P{}.E\n\nvar _ = A{}.F\n\nvar _ = U{}.Value\n",
				"p/sized_freebsd.go": "package p\n\ntype sizer interface{ Size() int }\n\ntype box struct{}\n\n" +
					"func (box) Area() int { return T{}.Act() }\n\nfunc Method() int { return 4 }\n",
			}, "the excluded files after the rename")
		})

		t.Run("leaves the excluded files that declare a method of the name of the method", func(t *testing.T) {
			t.Parallel()
			_, got, err := serving(t, renamedFiles()).Renamed(t.Context(),
				renaming("p/p.go", "T.Method", sema.KindMethod), "Act")
			assert.NoError(t, err, "Renamed of Method")
			assert.Equal(t, got, []trust.Caveat{{
				Code: trust.CaveatUnrewritten,
				Note: "the plan does not rename the declaration in 2 files that the build constraints exclude from " +
					"the build that the plan reads, such as p/clash_freebsd.go",
				Paths: []source.Path{"p/clash_freebsd.go", "p/iface_freebsd.go"},
			}}, "the caveats")
		})

		t.Run("renames a field that an excluded file names in a composite literal", func(t *testing.T) {
			t.Parallel()
			root := workspace(t, renamedFiles())
			got, _, err := over(t, root).Renamed(t.Context(), renaming("p/p.go", "T.Field", sema.KindField), "Count")
			assert.NoError(t, err, "Renamed of Field")
			assert.Equal(t, afterRename(t, root, got), map[source.Path]string{
				"p/p_freebsd.go": "package p\n\nvar _ = Old()\n\nvar _ = T{Count: 1}.Method()\n\nvar _ = S{}.E\n\n" +
					"var _ = P{}.E\n\nvar _ = A{}.F\n\nvar _ = U{}.Value\n",
			}, "the excluded files after the rename")
		})

		t.Run("renames the fields that embed a renamed type in an excluded file", func(t *testing.T) {
			t.Parallel()
			root := workspace(t, renamedFiles())
			got, _, err := over(t, root).Renamed(t.Context(), renaming("p/p.go", "E", sema.KindStruct), "Embedded")
			assert.NoError(t, err, "Renamed of E")
			assert.Equal(t, afterRename(t, root, got), map[source.Path]string{
				"p/p_freebsd.go": "package p\n\nvar _ = Old()\n\nvar _ = T{Field: 1}.Method()\n\nvar _ = S{}.Embedded\n\n" +
					"var _ = P{}.Embedded\n\nvar _ = A{}.F\n\nvar _ = U{}.Value\n",
			}, "the excluded files after the rename")
		})

		t.Run("renames the field that embeds a renamed alias in an excluded file", func(t *testing.T) {
			t.Parallel()
			root := workspace(t, renamedFiles())
			got, _, err := over(t, root).Renamed(t.Context(), renaming("p/p.go", "F", sema.KindType), "G")
			assert.NoError(t, err, "Renamed of F")
			assert.Equal(t, afterRename(t, root, got), map[source.Path]string{
				"p/p_freebsd.go": "package p\n\nvar _ = Old()\n\nvar _ = T{Field: 1}.Method()\n\nvar _ = S{}.E\n\n" +
					"var _ = P{}.E\n\nvar _ = A{}.G\n\nvar _ = U{}.Value\n",
			}, "the excluded files after the rename")
		})

		t.Run("names in the caveat the files of a port whose build excludes the declaration", func(t *testing.T) {
			t.Parallel()
			got, left, err := serving(t, renamedFiles()).Renamed(t.Context(),
				renaming("twin/twin.go", "Twin", sema.KindFunction), "Once")
			assert.NoError(t, err, "Renamed of Twin")
			assert.Empty(t, got, "the changes")
			assert.Equal(t, pathsOf(left, trust.CaveatUnrewritten),
				[]source.Path{"twin/twin_plan9.go", "twin/use_plan9.go"}, "the files of the caveat")
		})

		t.Run("renames the uses in an excluded file of a module that the go.work file lists", func(t *testing.T) {
			t.Parallel()
			root := written(t, workedFiles())
			got, _, err := over(t, root).Renamed(t.Context(), renaming("a/p/p.go", "Old", sema.KindFunction), "New")
			assert.NoError(t, err, "Renamed of the Old of a")
			assert.Equal(t, afterRename(t, root, got), map[source.Path]string{
				"a/w/w_freebsd.go": "package w\n\nimport \"example.com/a/p\"\n\nvar _ = p.New()\n",
			}, "the excluded files after the rename")
		})

		t.Run("renames the uses in an excluded file of a module that the go.work file leaves out", func(t *testing.T) {
			t.Parallel()
			root := written(t, workedFiles())
			got, _, err := over(t, root).Renamed(t.Context(), renaming("c/p/p.go", "Old", sema.KindFunction), "New")
			assert.NoError(t, err, "Renamed of the Old of c")
			assert.Equal(t, afterRename(t, root, got), map[source.Path]string{
				"c/w/w_freebsd.go": "package w\n\nimport \"example.com/c/p\"\n\nvar _ = p.New()\n",
			}, "the excluded files after the rename")
		})

		t.Run("returns a caveat without paths for a declaration in a file of another port", func(t *testing.T) {
			t.Parallel()
			got, left, err := serving(t, gated()).Renamed(t.Context(),
				edit.Target{
					Kind:   edit.TargetSpan,
					Symbol: sema.NewID(golang.Language, "ported", "Plan", sema.KindFunction),
					Span:   source.Span{Path: "ported/ported_plan9.go"},
				}, "Program")
			assert.NoError(t, err, "Renamed of Plan")
			assert.Empty(t, got, "the changes")
			assert.Equal(t, left, []trust.Caveat{{
				Code: trust.CaveatUnrewritten,
				Note: "the server renames the declaration in ported/ported_plan9.go in the build of another port, " +
					"and the build constraints can exclude other files that name it from that build",
			}}, "the caveats")
		})

		t.Run("returns no change and no caveat when no excluded file names the declaration", func(t *testing.T) {
			t.Parallel()
			got, left, err := serving(t, renamedFiles()).Renamed(t.Context(),
				renaming("lone/lone.go", "Lone", sema.KindFunction), "Alone")
			assert.NoError(t, err, "Renamed of Lone")
			assert.Empty(t, got, "the changes")
			assert.Empty(t, left, "the caveats")
		})

		t.Run("returns the error of the walk of a removed workspace", func(t *testing.T) {
			t.Parallel()
			_, _, err := removed(t).Renamed(t.Context(), renaming("p/p.go", "Old", sema.KindFunction), "New")
			assert.ErrorIs(t, err, fs.ErrNotExist, "the error of Renamed over a removed workspace")
			assert.HasPrefix(t, err.Error(), walking, "the error of Renamed over a removed workspace")
		})

		t.Run("returns the error of a workspace without a Go module", func(t *testing.T) {
			t.Parallel()
			_, _, err := moduleless(t).Renamed(t.Context(), renaming("p/p.go", "P", sema.KindFunction), "Q")
			assert.HasError(t, err, "Renamed over a workspace without a Go module")
			assert.Contains(t, err.Error(), "contains no Go module", "the error of Renamed")
		})
	})
}

// renaming returns the target of a rename of a declaration in the file at declared. Its ID has
// the qualified name and the kind that the arguments give, and the unit of the directory of
// the file.
func renaming(declared source.Path, name string, kind sema.Kind) edit.Target {
	unit := source.Path(filepath.ToSlash(filepath.Dir(filepath.FromSlash(string(declared)))))
	return edit.Target{
		Kind:   edit.TargetSpan,
		Symbol: sema.NewID(golang.Language, unit, name, kind),
		Span:   source.Span{Path: declared},
	}
}

// afterRename returns the content of each file of changes under root after its edits, by
// workspace path.
func afterRename(t *testing.T, root string, changes []edit.Change) map[source.Path]string {
	t.Helper()
	out := map[source.Path]string{}
	for _, one := range changes {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(string(one.Path))))
		assert.NoError(t, err, "ReadFile of "+string(one.Path))
		after, err := edit.Apply(content, one.Edits)
		assert.NoError(t, err, "Apply of the edits of "+string(one.Path))
		out[one.Path] = string(after)
	}
	return out
}
