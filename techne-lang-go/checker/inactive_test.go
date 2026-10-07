// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/go/checker"
)

// never is the build constraint of the excluded files of [gated], which excludes a file under
// every build configuration that sets no tag never.
const never = "//go:build never\n\n"

// gated returns a module whose packages have files that the build constraints exclude from the
// default build:
//
//   - clock declares Now and the interface Clock, and clock/never.go uses Now
//   - use imports clock and declares C, a Clock, and use/never.go and use/never_test.go import
//     clock too
//   - via imports use and declares C, the C of use, and via/never.go uses the method Now of C
//     without an import
//   - other/never.go imports via, so it can use clock through it, and names nothing of clock
//   - apart imports nothing of the module, and apart/never.go imports strings and uses Apart
//   - lone imports nothing, and no excluded file can use it
//   - ported has files of plan9, a port that the default build is not: ported_plan9.go, which
//     every build of plan9 includes, cgo_plan9.go, which imports C, tagged_plan9.go, which
//     needs cgo, nocgo_plan9.go, which needs cgo off, and nul_plan9.go, whose header contains
//     a NUL byte
func gated() map[string]string {
	return map[string]string{
		"clock/clock.go": "package clock\n\n// Clock reads the time.\ntype Clock interface{ Now() int }\n\n" +
			"func Now() int { return 0 }\n",
		"clock/never.go": never + "package clock\n\nfunc Other() int { return Now() }\n",
		"use/use.go": "package use\n\nimport \"example.com/p/clock\"\n\nfunc Use() int { return clock.Now() }\n\n" +
			"// C is the clock of the package.\nvar C clock.Clock\n",
		"use/never.go": never + "package use\n\nimport \"example.com/p/clock\"\n\nvar _ = clock.Now\n",
		"use/never_test.go": never + "package use\n\nimport (\n\t\"testing\"\n\n\t\"example.com/p/clock\"\n)\n\n" +
			"func TestNever(t *testing.T) { _ = clock.Now() }\n",
		"via/via.go": "package via\n\nimport \"example.com/p/use\"\n\nvar V = use.Use\n\n" +
			"// C is the clock of use.\nvar C = use.C\n",
		"via/never.go":           never + "package via\n\nvar _ = C.Now\n",
		"other/other.go":         "package other\n\nfunc Other() int { return 2 }\n",
		"other/never.go":         never + "package other\n\nimport \"example.com/p/via\"\n\nvar _ = via.V\n",
		"apart/apart.go":         "package apart\n\nfunc Apart() int { return 1 }\n",
		"apart/never.go":         never + "package apart\n\nimport \"strings\"\n\nvar _ = strings.ToUpper\n\nvar _ = Apart\n",
		"lone/lone.go":           "package lone\n\nfunc Lone() int { return 3 }\n",
		"ported/ported.go":       "package ported\n\nfunc Ported() int { return 4 }\n",
		"ported/ported_plan9.go": "package ported\n\nfunc Plan() int { return Ported() }\n",
		"ported/cgo_plan9.go":    "package ported\n\nimport \"C\"\n\nvar _ = Plan\n",
		"ported/tagged_plan9.go": "//go:build cgo\n\npackage ported\n\nvar _ = Plan\n",
		"ported/nocgo_plan9.go":  "//go:build !cgo\n\npackage ported\n\nvar _ = Plan\n",
		"ported/nul_plan9.go":    "package ported\x00\n",
	}
}

// lockedFile is the file of [locked] that nobody can read.
const lockedFile = "ported/locked_plan9.go"

// walking is the start of the error of a walk of the workspace that fails, which the checker
// returns before it runs the go command.
const walking = "checker: walk "

func TestInactive(t *testing.T) {
	t.Parallel()

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the excluded files that can use a declaration and name it as unread", func(t *testing.T) {
			t.Parallel()
			got, err := gatedRelate(t, "clock", "Now", sema.KindFunction, sema.ReferencedBy, false)
			assert.NoError(t, err, "Relate of the uses of Now")
			assert.Equal(t, places(got.Items), []string{"use/use.go:4"}, "the uses of Now")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.Equal(t, unreadIn(got.Caveats), []source.Path{"clock/never.go", "use/never.go", "via/never.go"},
				"the files of the caveat")
		})

		t.Run("reports an excluded test file as unread for a request with tests", func(t *testing.T) {
			t.Parallel()
			got, err := gatedRelate(t, "clock", "Now", sema.KindFunction, sema.ReferencedBy, true)
			assert.NoError(t, err, "Relate of the uses of Now")
			assert.Contains(t, unreadIn(got.Caveats), source.Path("use/never_test.go"), "the files of the caveat")
		})

		t.Run("names the one excluded file that can use a declaration", func(t *testing.T) {
			t.Parallel()
			got, err := gatedRelate(t, "apart", "Apart", sema.KindFunction, sema.ReferencedBy, true)
			assert.NoError(t, err, "Relate of the uses of Apart")
			assert.Contains(t, got.Caveats, trust.Caveat{
				Code:  trust.CaveatInactiveBuild,
				Note:  "the build constraints exclude apart/never.go from the build that the answer reads",
				Paths: []source.Path{"apart/never.go"},
			}, "the caveats of the answer")
		})

		t.Run("counts the excluded files that can use a declaration in the note", func(t *testing.T) {
			t.Parallel()
			got, err := gatedRelate(t, "clock", "Now", sema.KindFunction, sema.ReferencedBy, false)
			assert.NoError(t, err, "Relate of the uses of Now")
			assert.Contains(t, notes(got.Caveats), "the build constraints exclude 3 files from the build that "+
				"the answer reads, such as clock/never.go", "the notes of the caveats")
		})

		t.Run("returns a total answer when no excluded file can use the declaration", func(t *testing.T) {
			t.Parallel()
			got, err := gatedRelate(t, "lone", "Lone", sema.KindFunction, sema.ReferencedBy, true)
			assert.NoError(t, err, "Relate of the uses of Lone")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
			assert.Empty(t, unreadIn(got.Caveats), "the files of the caveat")
		})

		t.Run("returns a total answer for the calls of a function", func(t *testing.T) {
			t.Parallel()
			got, err := gatedRelate(t, "use", "Use", sema.KindFunction, sema.Calls, true)
			assert.NoError(t, err, "Relate of the calls of Use")
			assert.Equal(t, edges(got.Items), []string{"Now"}, "the calls of Use")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})

		t.Run("reports every excluded file as unread for the implementations of an interface", func(t *testing.T) {
			t.Parallel()
			got, err := gatedRelate(t, "clock", "Clock", sema.KindInterface, sema.ImplementedBy, false)
			assert.NoError(t, err, "Relate of the implementations of Clock")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.Equal(t, unreadIn(got.Caveats), []source.Path{
				"apart/never.go", "clock/never.go", "other/never.go", "ported/cgo_plan9.go", "ported/nocgo_plan9.go",
				"ported/nul_plan9.go", "ported/ported_plan9.go", "ported/tagged_plan9.go", "use/never.go",
				"via/never.go",
			}, "the files of the caveat")
		})

		t.Run("returns ErrDecline that names an excluded file of the scope", func(t *testing.T) {
			t.Parallel()
			_, err := gatedRelate(t, "clock", "Other", sema.KindFunction, sema.ReferencedBy, true)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
			assert.Contains(t, err.Error(), "the build constraints exclude clock/never.go", "the error of Relate")
		})

		t.Run("returns ErrDecline for a workspace whose module does not load", func(t *testing.T) {
			t.Parallel()
			root := written(t, map[string]string{"go.mod": "this is no go.mod\n", "p/p.go": "package p\n"})
			_, err := over(t, root).Relate(t.Context(), engine.Request{Scope: "p"},
				sema.NewID(golang.Language, "p", "P", sema.KindFunction), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
		})

		t.Run("returns ErrDecline for a workspace without a Go module", func(t *testing.T) {
			t.Parallel()
			_, err := moduleless(t).Relate(t.Context(), engine.Request{Scope: "p"},
				sema.NewID(golang.Language, "p", "P", sema.KindFunction), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
			assert.Contains(t, err.Error(), "contains no Go module", "the error of Relate")
		})

		t.Run("returns ErrDecline that leaves out an excluded test file for a request without tests",
			func(t *testing.T) {
				t.Parallel()
				_, err := gatedRelate(t, "use", "Missing", sema.KindFunction, sema.ReferencedBy, false)
				assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
				assert.Contains(t, err.Error(), "the build constraints exclude use/never.go from the build that "+
					"the answer reads", "the error of Relate")
			})

		t.Run("returns the imports of an excluded file of the importers of a package", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Relate(t.Context(), engine.Request{Scope: engine.Root, Tests: true},
				sema.NewID(golang.Language, "", "example.com/p/clock", sema.KindUnknown), sema.ImportedBy)
			assert.NoError(t, err, "Relate of the importers of example.com/p/clock")
			assert.Equal(t, places(got.Items), []string{"use/never.go:4", "use/never_test.go:7", "use/use.go:2"},
				"the sites of the imports")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})
	})

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the excluded files of the scope as unread", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Verify(t.Context(), engine.Request{Scope: "use", Tests: true}, nil)
			assert.NoError(t, err, "Verify of use")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.Equal(t, unreadIn(got.Caveats), []source.Path{"use/never.go", "use/never_test.go"},
				"the files of the caveat")
		})

		t.Run("leaves out an excluded test file for a request without tests", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Verify(t.Context(), engine.Request{Scope: "use"}, nil)
			assert.NoError(t, err, "Verify of use")
			assert.Equal(t, unreadIn(got.Caveats), []source.Path{"use/never.go"}, "the files of the caveat")
		})

		t.Run("reports an excluded file of another port as unread", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Verify(t.Context(), engine.Request{Scope: "ported/ported_plan9.go"}, nil)
			assert.NoError(t, err, "Verify of ported/ported_plan9.go")
			assert.Equal(t, unreadIn(got.Caveats), []source.Path{"ported/ported_plan9.go"}, "the files of the caveat")
		})

		t.Run("returns a total answer for a scope without an excluded file", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Verify(t.Context(), engine.Request{Scope: "lone", Tests: true}, nil)
			assert.NoError(t, err, "Verify of lone")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
		})
	})

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the excluded files that can use a changed package as unchecked", func(t *testing.T) {
			t.Parallel()
			changed := map[source.Path][]byte{"clock/clock.go": []byte(gated()["clock/clock.go"] + "\n// Changed.\n")}
			got, err := serving(t, gated()).Check(t.Context(), changed)
			assert.NoError(t, err, "Check of a change of clock")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the check")
			assert.Equal(t, dependentsIn(got.Caveats), []source.Path{
				"clock/never.go", "other/never.go", "use/never.go", "use/never_test.go", "via/never.go",
			}, "the files of the caveat")
			assert.Contains(t, notes(got.Caveats), "the build constraints exclude 5 files that can use a changed "+
				"package from the build that the check reads, such as clock/never.go", "the notes of the caveats")
		})

		t.Run("names the one excluded file that can use a changed package", func(t *testing.T) {
			t.Parallel()
			changed := map[source.Path][]byte{"apart/apart.go": []byte(gated()["apart/apart.go"] + "\n// Changed.\n")}
			got, err := serving(t, gated()).Check(t.Context(), changed)
			assert.NoError(t, err, "Check of a change of apart")
			assert.Contains(t, got.Caveats, trust.Caveat{
				Code: trust.CaveatDependents,
				Note: "the build constraints exclude apart/never.go, which can use a changed package, from the " +
					"build that the check reads",
				Paths: []source.Path{"apart/never.go"},
			}, "the caveats of the check")
		})

		t.Run("leaves out an excluded file that the change deletes", func(t *testing.T) {
			t.Parallel()
			changed := map[source.Path][]byte{
				"apart/apart.go": []byte(gated()["apart/apart.go"] + "\n// Changed.\n"),
				"apart/never.go": nil,
			}
			got, err := serving(t, gated()).Check(t.Context(), changed)
			assert.NoError(t, err, "Check of a change of apart that deletes apart/never.go")
			assert.Empty(t, dependentsIn(got.Caveats), "the files of the caveat")
		})

		t.Run("adds no caveat when no excluded file can use a changed package", func(t *testing.T) {
			t.Parallel()
			changed := map[source.Path][]byte{"lone/lone.go": []byte(gated()["lone/lone.go"] + "\n// Changed.\n")}
			got, err := serving(t, gated()).Check(t.Context(), changed)
			assert.NoError(t, err, "Check of a change of lone")
			assert.Empty(t, dependentsIn(got.Caveats), "the files of the caveat")
		})
	})

	t.Run("Unread", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the excluded files that can use a declaration of the unit of an ID", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Unread(t.Context(), engine.Request{Scope: "clock"},
				sema.NewID(golang.Language, "clock", "Now", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Unread of the uses of Now")
			assert.Equal(t, unreadIn(got), []source.Path{"clock/never.go", "use/never.go", "via/never.go"},
				"the files of the caveat")
		})

		t.Run("returns the excluded files that can use a declaration of the file of the request", func(t *testing.T) {
			t.Parallel()
			req := engine.Request{Scope: "apart", Declared: source.Span{Path: "apart/apart.go"}}
			got, err := serving(t, gated()).Unread(t.Context(), req,
				sema.NewID(golang.Language, "clock", "Apart", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Unread of the uses of Apart")
			assert.Equal(t, unreadIn(got), []source.Path{"apart/never.go"}, "the files of the caveat")
		})

		t.Run("returns an excluded file of a directory whose files the build constraints all exclude",
			func(t *testing.T) {
				t.Parallel()
				files := gated()
				files["remote/remote_plan9.go"] = "package remote\n\nimport \"example.com/p/clock\"\n\nvar _ = clock.Now\n"
				got, err := serving(t, files).Unread(t.Context(), engine.Request{Scope: "clock"},
					sema.NewID(golang.Language, "clock", "Now", sema.KindFunction), sema.ReferencedBy)
				assert.NoError(t, err, "Unread of the uses of Now")
				assert.Equal(t, unreadIn(got),
					[]source.Path{"clock/never.go", "remote/remote_plan9.go", "use/never.go", "via/never.go"},
					"the files of the caveat")
			})

		t.Run("returns a caveat without paths for a declaration in a file of another port", func(t *testing.T) {
			t.Parallel()
			req := engine.Request{Scope: "ported", Declared: source.Span{Path: "ported/ported_plan9.go"}}
			got, err := serving(t, gated()).Unread(t.Context(), req,
				sema.NewID(golang.Language, "ported", "Plan", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Unread of the uses of Plan")
			assert.Equal(t, got, []trust.Caveat{{
				Code: trust.CaveatInactiveBuild,
				Note: "the server reads ported/ported_plan9.go in the build of another port, and the build " +
					"constraints can exclude other files from that build",
			}}, "the caveats")
		})

		t.Run("returns the excluded files for a declaration in a file that no port builds", func(t *testing.T) {
			t.Parallel()
			req := engine.Request{Scope: "clock", Declared: source.Span{Path: "clock/never.go"}}
			got, err := serving(t, gated()).Unread(t.Context(), req,
				sema.NewID(golang.Language, "clock", "Other", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Unread of the uses of Other")
			assert.Equal(t, unreadIn(got), []source.Path{"clock/never.go"}, "the files of the caveat")
		})

		t.Run("returns no caveat for the calls of a function", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Unread(t.Context(), engine.Request{Scope: "use"},
				sema.NewID(golang.Language, "use", "Use", sema.KindFunction), sema.Calls)
			assert.NoError(t, err, "Unread of the calls of Use")
			assert.Empty(t, got, "the caveats")
		})

		t.Run("returns an excluded file that cannot be read as a file that names the declaration",
			func(t *testing.T) {
				t.Parallel()
				req := engine.Request{Scope: "ported", Declared: source.Span{Path: "ported/ported.go"}}
				got, err := locked(t).Unread(t.Context(), req,
					sema.NewID(golang.Language, "ported", "Ported", sema.KindFunction), sema.ReferencedBy)
				assert.NoError(t, err, "Unread of the uses of Ported")
				assert.Equal(t, unreadIn(got), []source.Path{lockedFile, "ported/ported_plan9.go"},
					"the files of the caveat")
			})

		t.Run("returns the error of the walk of a removed workspace", func(t *testing.T) {
			t.Parallel()
			_, err := removed(t).Unread(t.Context(), engine.Request{Scope: "clock"},
				sema.NewID(golang.Language, "clock", "Now", sema.KindFunction), sema.ReferencedBy)
			assert.ErrorIs(t, err, fs.ErrNotExist, "the error of Unread over a removed workspace")
			assert.HasPrefix(t, err.Error(), walking, "the error of Unread over a removed workspace")
		})

		t.Run("returns no caveat for the calls of a function in a file of another port", func(t *testing.T) {
			t.Parallel()
			req := engine.Request{Scope: "ported", Declared: source.Span{Path: "ported/ported_plan9.go"}}
			got, err := serving(t, gated()).Unread(t.Context(), req,
				sema.NewID(golang.Language, "ported", "Plan", sema.KindFunction), sema.Calls)
			assert.NoError(t, err, "Unread of the calls of Plan")
			assert.Empty(t, got, "the caveats")
		})
	})

	t.Run("Unverified", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the excluded files of the scope", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Unverified(t.Context(), engine.Request{Scope: "use", Tests: true})
			assert.NoError(t, err, "Unverified of use")
			assert.Equal(t, unreadIn(got), []source.Path{"use/never.go", "use/never_test.go"},
				"the files of the caveat")
			assert.Equal(t, notes(got), []string{"the build constraints can exclude 2 files from every build " +
				"that the server checks, such as use/never.go"}, "the notes of the caveats")
		})

		t.Run("leaves out an excluded file that every build of another port includes", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Unverified(t.Context(),
				engine.Request{Scope: "ported/ported_plan9.go", Tests: true})
			assert.NoError(t, err, "Unverified of ported/ported_plan9.go")
			assert.Empty(t, got, "the caveats")
		})

		t.Run("returns an excluded file that imports C", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Unverified(t.Context(), engine.Request{Scope: "ported/cgo_plan9.go"})
			assert.NoError(t, err, "Unverified of ported/cgo_plan9.go")
			assert.Equal(t, got, []trust.Caveat{{
				Code:  trust.CaveatInactiveBuild,
				Note:  "the build constraints can exclude ported/cgo_plan9.go from every build that the server checks",
				Paths: []source.Path{"ported/cgo_plan9.go"},
			}}, "the caveats")
		})

		t.Run("returns an excluded file whose build constraints need cgo", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Unverified(t.Context(), engine.Request{Scope: "ported/tagged_plan9.go"})
			assert.NoError(t, err, "Unverified of ported/tagged_plan9.go")
			assert.Equal(t, unreadIn(got), []source.Path{"ported/tagged_plan9.go"}, "the files of the caveat")
		})

		t.Run("returns an excluded file whose build constraints need cgo off", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Unverified(t.Context(), engine.Request{Scope: "ported/nocgo_plan9.go"})
			assert.NoError(t, err, "Unverified of ported/nocgo_plan9.go")
			assert.Equal(t, unreadIn(got), []source.Path{"ported/nocgo_plan9.go"}, "the files of the caveat")
		})

		t.Run("returns an excluded file whose header contains a NUL byte", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Unverified(t.Context(), engine.Request{Scope: "ported/nul_plan9.go"})
			assert.NoError(t, err, "Unverified of ported/nul_plan9.go")
			assert.Equal(t, unreadIn(got), []source.Path{"ported/nul_plan9.go"}, "the files of the caveat")
		})

		t.Run("returns an excluded file that cannot be read", func(t *testing.T) {
			t.Parallel()
			got, err := locked(t).Unverified(t.Context(), engine.Request{Scope: lockedFile})
			assert.NoError(t, err, "Unverified of "+lockedFile)
			assert.Equal(t, unreadIn(got), []source.Path{lockedFile}, "the files of the caveat")
		})

		t.Run("leaves out a file of each port of the go command", func(t *testing.T) {
			t.Parallel()
			listed, err := exec.CommandContext(t.Context(), "go", "tool", "dist", "list").Output()
			assert.NoError(t, err, "go tool dist list")
			ported := strings.Fields(string(listed))
			assert.NotEmpty(t, ported, "the ports of go tool dist list")
			files := map[string]string{"every/every.go": "package every\n"}
			for _, one := range ported {
				files["every/every_"+strings.ReplaceAll(one, "/", "_")+".go"] = "package every\n"
			}
			got, err := serving(t, files).Unverified(t.Context(), engine.Request{Scope: "every", Tests: true})
			assert.NoError(t, err, "Unverified of every")
			assert.Empty(t, got, "the caveats")
		})

		t.Run("returns no caveat for a scope without an excluded file", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Unverified(t.Context(), engine.Request{Scope: "lone", Tests: true})
			assert.NoError(t, err, "Unverified of lone")
			assert.Empty(t, got, "the caveats")
		})

		t.Run("returns the error of the walk of a removed workspace", func(t *testing.T) {
			t.Parallel()
			_, err := removed(t).Unverified(t.Context(), engine.Request{Scope: "use"})
			assert.ErrorIs(t, err, fs.ErrNotExist, "the error of Unverified over a removed workspace")
			assert.HasPrefix(t, err.Error(), walking, "the error of Unverified over a removed workspace")
		})

		t.Run("returns the error of a workspace without a Go module", func(t *testing.T) {
			t.Parallel()
			_, err := moduleless(t).Unverified(t.Context(), engine.Request{Scope: "p"})
			assert.HasError(t, err, "Unverified over a workspace without a Go module")
			assert.Contains(t, err.Error(), "contains no Go module", "the error of Unverified")
		})
	})

	t.Run("Unchecked", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the excluded files that can use a changed package", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Unchecked(t.Context(), map[source.Path][]byte{"via/via.go": nil})
			assert.NoError(t, err, "Unchecked of a change of via")
			assert.Equal(t, dependentsIn(got), []source.Path{"other/never.go", "via/never.go"},
				"the files of the caveat")
		})

		t.Run("returns no caveat when no excluded file can use a changed package", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, gated()).Unchecked(t.Context(), map[source.Path][]byte{"lone/lone.go": nil})
			assert.NoError(t, err, "Unchecked of a change of lone")
			assert.Empty(t, got, "the caveats")
		})

		t.Run("returns the error of the walk of a removed workspace", func(t *testing.T) {
			t.Parallel()
			_, err := removed(t).Unchecked(t.Context(), map[source.Path][]byte{"lone/lone.go": nil})
			assert.ErrorIs(t, err, fs.ErrNotExist, "the error of Unchecked over a removed workspace")
			assert.HasPrefix(t, err.Error(), walking, "the error of Unchecked over a removed workspace")
		})
	})
}

// TestInactiveEnv sets the environment of the process, so none of its cases runs in parallel.
func TestInactiveEnv(t *testing.T) {
	t.Run("Unverified", func(t *testing.T) {
		t.Run("returns an excluded file that the default port includes without the tags of GOFLAGS",
			func(t *testing.T) {
				t.Setenv("GOFLAGS", "-tags=flagged")
				files := gated()
				files["ported/flagged.go"] = "//go:build !flagged\n\npackage ported\n"
				got, err := serving(t, files).Unverified(t.Context(), engine.Request{Scope: "ported/flagged.go"})
				assert.NoError(t, err, "Unverified of ported/flagged.go")
				assert.Equal(t, unreadIn(got), []source.Path{"ported/flagged.go"}, "the files of the caveat")
			})
	})
}

// locked returns the checker over [gated] with lockedFile, a file of plan9 that nobody can read.
// It skips the test for root, which reads a file without read permission.
func locked(t *testing.T) *checker.Engine {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads a file without read permission")
	}
	files := gated()
	files[lockedFile] = "package ported\n"
	root := workspace(t, files)
	assert.NoError(t, os.Chmod(filepath.Join(root, filepath.FromSlash(lockedFile)), 0), "Chmod of "+lockedFile)
	return over(t, root)
}

// moduleless returns the checker over a workspace with the Go file p/p.go and no go.mod file.
func moduleless(t *testing.T) *checker.Engine {
	t.Helper()
	return over(t, written(t, map[string]string{"p/p.go": "package p\n\nfunc P() int { return 1 }\n"}))
}

// removed returns the checker over a directory that was removed after the checker was built, so
// each walk of it fails.
func removed(t *testing.T) *checker.Engine {
	t.Helper()
	root := t.TempDir()
	e := over(t, root)
	assert.NoError(t, os.Remove(root), "Remove of "+root)
	return e
}

// gatedRelate returns the answer of the checker over [gated] to the relations of kind of the
// declaration name of kind declared in the package at unit, with or without the test files.
func gatedRelate(
	t *testing.T,
	unit, name string,
	declared sema.Kind,
	kind sema.RelationKind,
	tests bool,
) (engine.Result[sema.Relation], error) {
	t.Helper()
	of := sema.NewID(golang.Language, source.Path(unit), name, declared)
	return serving(t, gated()).Relate(t.Context(), engine.Request{Scope: source.Path(unit), Tests: tests}, of, kind)
}

// unreadIn returns the paths of the [trust.CaveatInactiveBuild] caveats of caveats, in order.
func unreadIn(caveats []trust.Caveat) []source.Path {
	return pathsOf(caveats, trust.CaveatInactiveBuild)
}

// dependentsIn returns the paths of the [trust.CaveatDependents] caveats of caveats, in order.
func dependentsIn(caveats []trust.Caveat) []source.Path {
	return pathsOf(caveats, trust.CaveatDependents)
}

// pathsOf returns the paths of the caveats of code in caveats, in order.
func pathsOf(caveats []trust.Caveat, code trust.CaveatCode) []source.Path {
	var out []source.Path
	for _, one := range caveats {
		if one.Code == code {
			out = append(out, one.Paths...)
		}
	}
	return out
}

// notes returns the note of each caveat of caveats, in order.
func notes(caveats []trust.Caveat) []string {
	out := make([]string, 0, len(caveats))
	for _, one := range caveats {
		out = append(out, one.Note)
	}
	return out
}
