// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// The notes of the caveats of an answer about the imports of [imported], as the test pins them:
// byNameNote ends the caveat of the match by name that every such answer has, undefinedNote is
// the caveat that counts the imports of gone/store in missing.fake on line 3, counted from one,
// in prefix.fake and in twice.fake, and loadingNote starts the caveat of a server that had not
// settled.
const (
	byNameNote    = "an import under another name, such as an import of a directory, is not read"
	undefinedNote = "the server resolved 3 of the imports that write the name to no other file, such as the " +
		"import at missing.fake:3"
	loadingNote = "the server was still loading the workspace"
)

// atStore, atPort, atGone and atValue are the requests of the imports of the variables store of
// store.fake, port of port.fake, store of gone.fake and value of values.fake in [imported], over
// the whole workspace.
var (
	atStore = engine.Request{Scope: engine.Root, Declared: source.Span{Path: "store.fake"}}
	atPort  = engine.Request{Scope: engine.Root, Declared: source.Span{Path: "port.fake"}}
	atGone  = engine.Request{Scope: engine.Root, Declared: source.Span{Path: "gone.fake"}}
	atValue = engine.Request{Scope: engine.Root, Declared: source.Span{Path: "values.fake"}}
)

func TestImport(t *testing.T) {
	t.Parallel()

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each import with a name whose definition is in the file of the declaration",
			func(t *testing.T) {
				t.Parallel()
				got, err := importers(t, lsptest.Server(lsptest.Resolves), "store", ".", atStore)
				assert.NoError(t, err, "Relate of the importers of store")
				assert.Equal(t, edges(got.Items),
					[]string{"a-use.fake", "a/use.fake", "mixed.fake", "twice.fake", "use.fake"},
					"the files that import store")
			})

		t.Run("returns each import with a name whose definition is under the directory of the scope",
			func(t *testing.T) {
				t.Parallel()
				got, err := importers(t, lsptest.Server(lsptest.Resolves), "store", "a", engine.Request{Scope: "a"})
				assert.NoError(t, err, "Relate of the importers of a/store")
				assert.Equal(t, edges(got.Items), []string{"elsewhere.fake"}, "the files that import a/store")
			})

		t.Run("asks at the last occurrence of the name of an import in its line", func(t *testing.T) {
			t.Parallel()
			got, err := importers(t, lsptest.Server(lsptest.Resolves), "port", ".", atPort)
			assert.NoError(t, err, "Relate of the importers of port")
			assert.Equal(t, edges(got.Items), []string{"keyword.fake"}, "the files that import port")
		})

		t.Run("asks at the names of the import that the outline engine found alone", func(t *testing.T) {
			t.Parallel()
			got, err := importers(t, lsptest.Server(lsptest.Resolves), "store", ".", atGone)
			assert.NoError(t, err, "Relate of the importers of the store of gone.fake")
			assert.Empty(t, got.Items, "the files that import the store of gone.fake")
		})

		t.Run("asks at the module of an import that the server defines as the import itself", func(t *testing.T) {
			t.Parallel()
			got, err := importers(t, lsptest.Server(lsptest.Resolves), "value", ".", atValue)
			assert.NoError(t, err, "Relate of the importers of value")
			assert.Equal(t, edges(got.Items), []string{"named.fake"}, "the files that import value")
		})

		t.Run("counts an import that the server resolves to itself alone in a caveat", func(t *testing.T) {
			t.Parallel()
			got, err := importers(t, lsptest.Server(lsptest.Resolves), "value", ".", atValue)
			assert.NoError(t, err, "Relate of the importers of value")
			assert.True(t, caveated(got.Caveats, trust.CaveatUnsupported,
				"resolved 2 of the imports that write the name to no other file, such as the import at lost.fake:3"),
				"the answer has the caveat of the imports without a definition")
		})

		t.Run("asks again at an import that a starting server resolves to no file", func(t *testing.T) {
			t.Parallel()
			got, err := importers(t, lsptest.Server(lsptest.ResolvesLate), "store", ".", atStore)
			assert.NoError(t, err, "Relate of the importers of store")
			assert.Equal(t, edges(got.Items),
				[]string{"a-use.fake", "a/use.fake", "mixed.fake", "twice.fake", "use.fake"},
				"the files that import store")
		})

		t.Run("asks once at an import of a server that has run past its Resolving", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.ResolvesLate)
			server.Resolving = 0
			got, err := importers(t, server, "store", ".", atStore)
			assert.NoError(t, err, "Relate of the importers of store")
			assert.Empty(t, got.Items, "the files that import store")
			assert.True(t, caveated(got.Caveats, trust.CaveatUnsupported, "resolved 9 of the imports"),
				"the answer has the caveat of the imports without a definition")
		})

		t.Run("asks the outline engine for every import whatever the limit of the request", func(t *testing.T) {
			t.Parallel()
			limited := atStore
			limited.Limit = 1
			got, err := importers(t, lsptest.Server(lsptest.Resolves), "store", ".", limited)
			assert.NoError(t, err, "Relate of the importers of store")
			assert.Length(t, got.Items, 5, "the imports of store")
		})

		t.Run("returns the site of each import at the line that imports", func(t *testing.T) {
			t.Parallel()
			got, err := importers(t, lsptest.Server(lsptest.Resolves), "store", ".", atStore)
			assert.NoError(t, err, "Relate of the importers of store")
			var sites []string
			for _, one := range got.Items {
				sites = append(sites, fmt.Sprintf("%s:%d", one.At.Path, one.At.Start.Line))
			}
			assert.Equal(t, sites,
				[]string{"a-use.fake:2", "a/use.fake:2", "mixed.fake:2", "twice.fake:2", "use.fake:2"},
				"the sites of the imports")
		})

		t.Run("returns a partial answer with the caveat of the match by name", func(t *testing.T) {
			t.Parallel()
			got, err := importers(t, lsptest.Server(lsptest.Resolves), "store", ".", atStore)
			assert.NoError(t, err, "Relate of the importers of store")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.True(t, caveated(got.Caveats, trust.CaveatUnsupported, byNameNote),
				"the answer has the caveat of the match by name")
		})

		t.Run("counts the imports without a definition in a caveat", func(t *testing.T) {
			t.Parallel()
			got, err := importers(t, lsptest.Server(lsptest.Resolves), "store", ".", atStore)
			assert.NoError(t, err, "Relate of the importers of store")
			assert.True(t, caveated(got.Caveats, trust.CaveatUnsupported, undefinedNote),
				"the answer has the caveat of the import without a definition")
		})

		t.Run("counts a single import without a definition in a caveat", func(t *testing.T) {
			t.Parallel()
			got, err := importers(t, lsptest.Server(lsptest.Resolves), "port", ".", atPort)
			assert.NoError(t, err, "Relate of the importers of port")
			assert.True(t, caveated(got.Caveats, trust.CaveatUnsupported, "resolved 1 of the imports"),
				"the answer has the caveat of the import without a definition")
		})

		t.Run("counts an import whose definition request fails as an import without a definition",
			func(t *testing.T) {
				t.Parallel()
				server := lsptest.Server(lsptest.Cancels)
				server.Imports = true
				got, err := importers(t, server, "store", ".", atStore)
				assert.NoError(t, err, "Relate of the importers of store")
				assert.Empty(t, got.Items, "the files that import store")
				assert.True(t, caveated(got.Caveats, trust.CaveatUnsupported, "resolved 9 of the imports"),
					"the answer has the caveat of the imports without a definition")
			})

		t.Run("returns the caveat of a server that has not settled", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Stuck)
			server.Imports = true
			got, err := importers(t, server, "store", ".", atStore)
			assert.NoError(t, err, "Relate of the importers of store")
			assert.True(t, caveated(got.Caveats, trust.CaveatIndexWarming, loadingNote),
				"the answer has the caveat of a server that was loading")
		})

		t.Run("returns ErrDecline for a server that does not declare Imports", func(t *testing.T) {
			t.Parallel()
			_, err := importers(t, lsptest.Server(lsptest.Default), "store", ".", atStore)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
		})

		t.Run("returns ErrDecline for an engine whose outline engine does not relate imports", func(t *testing.T) {
			t.Parallel()
			e := lsptest.Engine(t, lsptest.Workspace(t, imported()), lsptest.Server(lsptest.Resolves))
			_, err := e.Relate(t.Context(), atStore, variableIn(".", "store"), sema.ImportedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
		})
	})
}

// imported returns a workspace of files that import the names store and port. store.fake,
// a/store.fake and gone.fake each declare a variable store, other.fake the variable other, and
// port.fake the variable port. use.fake, a-use.fake and a/use.fake import store, elsewhere.fake
// imports a/store, mixed.fake imports other and store on one line, and missing.fake imports
// gone/store, whose file gone/store.fake does not exist. twice.fake imports store on line 2 and
// gone/store on line 3, and prefix.fake imports gone/store on line 2 and gone on line 3.
// keyword.fake imports port, which its keyword import contains too, and portless.fake imports
// gone/port. values.fake declares the variable value, which named.fake imports from values,
// elsenamed.fake from other, lost.fake from vanished, whose file vanished.fake does not exist,
// and self.fake from itself. A walk of the directories reads a/use.fake before a-use.fake,
// against the order of their paths.
func imported() map[string]string {
	return map[string]string{
		"store.fake":     "package a\n\nvar store = 1\n",
		"a/store.fake":   "package a\n\nvar store = 2\n",
		"gone.fake":      "package a\n\nvar store = 5\n",
		"other.fake":     "package a\n\nvar other = 3\n",
		"port.fake":      "package a\n\nvar port = 4\n",
		"prefix.fake":    "package b\n\nimport gone/store\nimport gone\n",
		"use.fake":       "package b\n\nimport store\n",
		"a-use.fake":     "package b\n\nimport store\n",
		"a/use.fake":     "package b\n\nimport store\n",
		"elsewhere.fake": "package b\n\nimport a/store\n",
		"mixed.fake":     "package b\n\nimport other, store\n",
		"missing.fake":   "package b\n\nimport gone/store\n",
		"twice.fake":     "package b\n\nimport store\nimport gone/store\n",
		"keyword.fake":   "package b\n\nimport port\n",
		"portless.fake":  "package b\n\nimport gone/port\n",
		"values.fake":    "package a\n\nvar value = 6\n",
		"named.fake":     "package b\n\nfrom values import value\n",
		"elsenamed.fake": "package b\n\nfrom other import value\n",
		"lost.fake":      "package b\n\nfrom vanished import value\n",
		"self.fake":      "package b\n\nfrom self import value\n",
	}
}

// importers returns the answer of [sema.ImportedBy] for the variable name of the directory unit
// of [imported], from an engine that runs server and reads the declarations of a file through
// [lsptest.Parser], asked with req.
func importers(
	t *testing.T,
	server lsp.Server,
	name string,
	unit source.Path,
	req engine.Request,
) (engine.Result[sema.Relation], error) {
	t.Helper()
	e := lsptest.Parsing(t, lsptest.Workspace(t, imported()), server)
	return e.Relate(t.Context(), req, variableIn(unit, name), sema.ImportedBy)
}

// variableIn returns the ID of the variable name of [imported] in the directory unit.
func variableIn(unit source.Path, name string) sema.ID {
	return sema.NewID(lsptest.Language, unit, name, sema.KindVariable)
}

// caveated reports whether caveats has a caveat of code whose note contains text.
func caveated(caveats []trust.Caveat, code trust.CaveatCode, text string) bool {
	return slices.ContainsFunc(caveats, func(one trust.Caveat) bool {
		return one.Code == code && strings.Contains(one.Note, text)
	})
}
