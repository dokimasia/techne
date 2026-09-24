// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// hidden reads Store from a call of a function that nothing declares, so the type checker does
// not bind Store on that line.
const hidden = "package p\n\nvar bad = missing().Store\n"

// modules returns the files of two modules, a with Target and b with an error on a line that
// writes Target, and a go.work file of both when worked is true.
func modules(worked bool) map[string]string {
	files := map[string]string{
		"a/go.mod": "module example.com/a\n\ngo 1.24\n",
		"a/x/x.go": "package x\n\nfunc Target() int { return 1 }\n",
		"b/go.mod": "module example.com/b\n\ngo 1.24\n",
		"b/b.go":   "package b\n\nvar B = Target + 1\n",
	}
	if worked {
		files["go.work"] = "go 1.24\n\nuse (\n\t./a\n\t./b\n)\n"
	}
	return files
}

func TestEvidence(t *testing.T) {
	t.Parallel()

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an answer when an error is on a line that writes the name", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["bad.go"] = hidden
			got, err := serving(t, files).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with an error that writes Store")
			assert.Equal(t, got.Lowered, trust.Indexed, "the lowered tier of the answer")
			assert.Equal(t, got.Caveats[1], trust.Caveat{
				Code: trust.CaveatBuildBroken,
				Note: "the type checker reports an error that can hide a use at bad.go:3. Names are bound " +
					"where the type checker could bind them and matched by text elsewhere",
				Paths: []source.Path{"bad.go"},
			}, "the caveat of the error")
		})

		t.Run("keeps the tier when no error line writes the name", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["broken.go"] = broken
			got, err := serving(t, files).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with an error that does not write Store")
			assert.Equal(t, got.Lowered, trust.None, "the lowered tier of the answer")
			assert.False(t, carries(got.Caveats, trust.CaveatBuildBroken), "the build caveat of the answer")
		})

		t.Run("keeps the tier of an answer about a module beside a module with an error", func(t *testing.T) {
			t.Parallel()
			got, err := over(t, written(t, modules(false))).Relate(t.Context(), engine.Request{Scope: "."},
				target, sema.ReferencedBy)
			assert.NoError(t, err, "Relate of Target of module a")
			assert.Equal(t, got.Lowered, trust.None, "the lowered tier of the answer")
		})

		t.Run("lowers an answer when another module of a go.work file writes the name on an error line",
			func(t *testing.T) {
				t.Parallel()
				got, err := over(t, written(t, modules(true))).Relate(t.Context(), engine.Request{Scope: "."},
					target, sema.ReferencedBy)
				assert.NoError(t, err, "Relate of Target of module a")
				assert.Equal(t, got.Lowered, trust.Indexed, "the lowered tier of the answer")
			})

		t.Run("returns the dynamic caveat", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Relate(t.Context(), engine.Request{Scope: "."},
				subject("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.True(t, carries(got.Caveats, trust.CaveatDynamic), "the dynamic caveat of the answer")
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an answer when an error is on the line of the position", func(t *testing.T) {
			t.Parallel()
			line := "package p\n\nvar n = Store{}.missing\n"
			files := whole()
			files["line.go"] = line
			got, err := serving(t, files).Resolve(t.Context(), engine.Request{Scope: "line.go"}, at(t, line, "= Store"))
			assert.NoError(t, err, "Resolve on the line of the error")
			assert.Equal(t, names(got.Items), []string{"Store"}, "the declaration of the use")
			assert.Equal(t, got.Lowered, trust.Indexed, "the lowered tier of the answer")
		})

		t.Run("keeps the tier when the error is on another line", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["broken.go"] = broken
			got, err := serving(t, files).Resolve(t.Context(), engine.Request{Scope: "use.go"},
				at(t, use, "held := Store"))
			assert.NoError(t, err, "Resolve beside the error")
			assert.Equal(t, got.Lowered, trust.None, "the lowered tier of the answer")
		})

		t.Run("lowers an empty answer in a program with an error", func(t *testing.T) {
			t.Parallel()
			files := whole()
			files["broken.go"] = broken
			got, err := serving(t, files).Resolve(t.Context(), engine.Request{Scope: "use.go"}, source.Position{})
			assert.NoError(t, err, "Resolve on the keyword package")
			assert.Empty(t, got.Items, "the declarations at the keyword")
			assert.Equal(t, got.Lowered, trust.Indexed, "the lowered tier of the empty answer")
		})

		t.Run("returns the dynamic caveat", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Resolve(t.Context(), engine.Request{Scope: "use.go"},
				at(t, use, "held := Store"))
			assert.NoError(t, err, "Resolve of a use of Store")
			assert.True(t, carries(got.Caveats, trust.CaveatDynamic), "the dynamic caveat of the answer")
		})
	})
}
