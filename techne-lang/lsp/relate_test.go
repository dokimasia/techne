// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// limitLine is the line that [limited] appends to [lsptest.Content] as line 9. The scripted
// server lists no symbol for it.
const limitLine = "const Limit = 8\n"

// limited returns a workspace with [lsptest.Content] and [limitLine] in a.fake.
func limited() map[string]string {
	return map[string]string{"a.fake": lsptest.Content + limitLine}
}

// limit returns the span of the declaration of Limit in the a.fake of [limited].
func limit() source.Span {
	start := len(lsptest.Content)
	return source.Span{
		Path:  "a.fake",
		Start: source.Position{Offset: start, Line: 9},
		End:   source.Position{Offset: start + len(limitLine) - 1, Line: 9, Column: len(limitLine) - 1},
	}
}

// used returns a workspace with a bundle of three functions in a.fake, and a function Use in
// b.fake whose local variable held calls F0.
func used() map[string]string {
	return map[string]string{
		"a.fake": lsptest.Bundle(3),
		"b.fake": "package a\nfunc Use() int {\n\tvar held = F0()\n\treturn held\n}\n",
	}
}

// projected returns a workspace of three projects of tsconfig.json files: [lsptest.Content] in
// a.fake at the root, the type Vault in b/b.fake, which uses Store through a line that binds it
// as an import does, and the type Other in c/c.fake, which uses a Store that it binds nowhere.
func projected() map[string]string {
	return map[string]string{
		"tsconfig.json":   "{}\n",
		"a.fake":          lsptest.Content,
		"b/tsconfig.json": "{}\n",
		"b/b.fake":        "package b\n\n// imports Store\n\ntype Vault struct {\n}\n\nvar _ Store\n",
		"c/tsconfig.json": "{}\n",
		"c/c.fake":        "package c\n\ntype Other struct {\n}\n\nvar _ Store\n",
	}
}

// writing returns an engine of a scoped server over [lsptest.Content] in a.fake and one file
// more than a preload opens, each with the content text.
func writing(t *testing.T, text string) *lsp.Engine {
	t.Helper()
	files := sample()
	for i := range 201 {
		files[fmt.Sprintf("other/f%d.fake", i)] = text
	}
	server := lsptest.Server(lsptest.Scoped)
	server.Scoped = true
	return lsptest.Engine(t, lsptest.Workspace(t, files), server)
}

// requested returns how many requests of method the file log of [lsptest.RecordRequests]
// records.
func requested(t *testing.T, log, method string) int {
	t.Helper()
	recorded, err := os.ReadFile(log)
	assert.NoError(t, err, "the log of the requests")
	return strings.Count(string(recorded), method+"\n")
}

func TestRelate(t *testing.T) {
	t.Parallel()

	request := engine.Request{Scope: "a.fake"}

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("relates each use to the declaration that contains it", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
		})

		t.Run("returns the relations of a new server when the server has stopped answering", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "starts")
			e := lsptest.Parsing(t, lsptest.Workspace(t, sample()),
				lsptest.Server(lsptest.Mutes, lsptest.RecordStarts(log)))
			got, err := e.Relate(t.Context(), request, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
			assert.Equal(t, starts(t, log), 2, "the starts of the server")
		})

		t.Run("returns the relations of a new server after the server exits", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "starts")
			e := serving(t, lsptest.Exits, sample(), lsptest.RecordStarts(log))
			_, err := e.Relate(t.Context(), request, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.HasError(t, err, "the error of the Relate that the server exits during")
			got, err := e.Relate(t.Context(), request, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "the error of the Relate after the exit")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
			assert.Equal(t, starts(t, log), 2, "the starts of the server")
		})

		t.Run("returns ErrDecline when a new server has stopped answering too", func(t *testing.T) {
			t.Parallel()
			e := lsptest.Parsing(t, lsptest.Workspace(t, sample()), lsptest.Server(lsptest.Mutes))
			_, err := e.Relate(t.Context(), request, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
			assert.Contains(t, err.Error(), "fake returned no symbol of a.fake", "the error of Relate")
		})

		t.Run("returns the source line of each use", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.Equal(t, got.Items[0].Via, "func (s *Store) Get() int { return s.size }",
				"the line of the first use")
		})

		t.Run("cuts the source line of a use on a long line", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Minified, map[string]string{"a.fake": lsptest.Bundle(100)}).
				Relate(t.Context(), request, declared("F0", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of F0 in a bundle")
			assert.Equal(t, edges(got.Items), []string{"After"}, "the declarations that use F0")
			via := got.Items[0].Via
			assert.That(t, via).
				HasPrefix("…", "the start of the line of the use of F0").
				HasSuffix("return F0() }", "the end of the line of the use of F0")
			assert.InRange(t, len(via), -1<<63, float64(lang.LineLimit+len("…")),
				"the line of the use of F0 is at most lang.LineLimit bytes and an ellipsis: "+via)
		})

		t.Run("finds a use in a file that a scoped server had not opened", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Scoped)
			server.Scoped = true
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Content, "b.fake": "var _ Store\n"})
			got, err := lsptest.Engine(t, root, server).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			inB := func(one sema.Relation) bool { return one.At.Path == "b.fake" }
			assert.True(t, slices.ContainsFunc(got.Items, inB), "the uses of Store include b.fake")
		})

		t.Run("returns a partial answer past the files that a scoped server opens", func(t *testing.T) {
			t.Parallel()
			store := declared("Store", sema.KindStruct)
			got, err := writing(t, useOfStore).Relate(t.Context(), request, store, sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.True(t, cutShort(got.Caveats), "the answer has the caveat of a short preload")
		})

		t.Run("opens no file that writes the name only on comment lines", func(t *testing.T) {
			t.Parallel()
			store := declared("Store", sema.KindStruct)
			got, err := writing(t, "// Store\n\t// uses Store\n").Relate(t.Context(), request, store, sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.False(t, cutShort(got.Caveats), "the answer has the caveat of a short preload")
		})

		t.Run("opens a file that writes the name at the start of a line after a comment line", func(t *testing.T) {
			t.Parallel()
			store := declared("Store", sema.KindStruct)
			got, err := writing(t, "// Store\nStore{}\n").Relate(t.Context(), request, store, sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.True(t, cutShort(got.Caveats), "the answer has the caveat of a short preload")
		})

		t.Run("returns a whole answer past the preload when tsserver loads the project of every file",
			func(t *testing.T) {
				t.Parallel()
				files := sample()
				files["tsconfig.json"] = "{}"
				for i := range 201 {
					files[fmt.Sprintf("other/f%d.fake", i)] = "var _ Store\n"
				}
				server := lsptest.Server(lsptest.Projects)
				server.Scoped, server.Tsserver = true, true
				got, err := lsptest.Engine(t, lsptest.Workspace(t, files), server).Relate(t.Context(), request,
					declared("Store", sema.KindStruct), sema.ReferencedBy)
				assert.NoError(t, err, "Relate of the uses of Store")
				assert.False(t, cutShort(got.Caveats), "the answer has the caveat of a short preload")
			})

		loose := []struct {
			name      string
			declaring string            // the content of a.fake, moduleContent when empty
			give      string            // the text before the use of Store in each file outside a project
			files     map[string]string // the other files of the workspace
			links     map[string]string // the symbolic links of the workspace and the paths they point to
			want      bool              // whether the answer has the caveat of a short preload
		}{
			{
				name: "returns a whole answer past the preload when no file outside a project imports the module",
			},
			{
				name: "returns a partial answer past the preload when the files outside a project import the module",
				give: `import { Store } from "../a.fake"` + "\n",
				want: true,
			},
			{
				name: "returns a partial answer past the preload for the files that name the module in a " +
					"reference directive",
				give: `/// <reference path="../a.fake" />` + "\n",
				want: true,
			},
			{
				name:  "returns a partial answer past the preload for the files that import the module through a file",
				give:  `import "./helper"` + "\n",
				files: map[string]string{"other/helper.ts": `export * from "../a.fake"` + "\n"},
				want:  true,
			},
			{
				name: "returns a partial answer past the preload for the files that import the module through the " +
					"TypeScript file of a JavaScript specifier",
				give:  `import "./helper.js"` + "\n",
				files: map[string]string{"other/helper.ts": `export * from "../a.fake"` + "\n"},
				want:  true,
			},
			{
				name: "returns a partial answer past the preload for the files that import the module through the " +
					"index file of a directory",
				give:  `import "./lib"` + "\n",
				files: map[string]string{"other/lib/index.ts": `export * from "../../a.fake"` + "\n"},
				want:  true,
			},
			{
				name: "returns a partial answer past the preload for the files that import the module through a " +
					"file that imports them",
				give: `import "./helper"` + "\n",
				files: map[string]string{
					"other/helper.ts": `import "./f0.fake"` + "\n" + `export * from "../a.fake"` + "\n",
				},
				want: true,
			},
			{
				name:  "returns a whole answer past the preload for the files that import a file that leads nowhere",
				give:  `import "./helper"` + "\n",
				files: map[string]string{"other/helper.ts": `import "./f0.fake"` + "\n"},
			},
			{
				name: "returns a partial answer past the preload for the files that import a directory without " +
					"an index file",
				give:  `import "./lib"` + "\n",
				files: map[string]string{"other/lib/notes.md": "notes\n"},
				want:  true,
			},
			{
				name:  "returns a partial answer past the preload for the files that import a file of a loaded project",
				give:  `import "../project/x.fake"` + "\n",
				files: map[string]string{"project/tsconfig.json": "{}\n", "project/x.fake": useOfStore},
				want:  true,
			},
			{
				name: "returns a partial answer past the preload for the files that import a file outside the " +
					"workspace",
				give: `import "../../x"` + "\n",
				want: true,
			},
			{
				name: "returns a partial answer past the preload for the files that import a subpath of a package",
				give: `import { Store } from "#store"` + "\n",
				want: true,
			},
			{
				name:  "returns a partial answer past the preload for the files that import a linked package",
				give:  `import { Store } from "shared/store"` + "\n",
				files: map[string]string{"shared/index.ts": "export {}\n"},
				links: map[string]string{"node_modules/shared": "shared"},
				want:  true,
			},
			{
				name:  "returns a partial answer past the preload for the files that import a linked scoped package",
				give:  `import { Store } from "@team/shared/store"` + "\n",
				files: map[string]string{"shared/index.ts": "export {}\n"},
				links: map[string]string{"node_modules/@team/shared": "shared"},
				want:  true,
			},
			{
				name:  "returns a whole answer past the preload for the files that import an installed package",
				give:  `import { Store } from "shared"` + "\n",
				files: map[string]string{"node_modules/shared/index.ts": "export {}\n"},
			},
			{
				name:      "returns a partial answer past the preload for a module that opens a global block",
				declaring: moduleContent + "declare global {\n}\n",
				want:      true,
			},
			{
				name:      "returns a partial answer past the preload for a file without an import or an export",
				declaring: lsptest.Content,
				want:      true,
			},
		}
		for _, tt := range loose {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				files := map[string]string{"a.fake": cmp.Or(tt.declaring, moduleContent)}
				maps.Copy(files, tt.files)
				got, err := projectless(t, files, tt.give+useOfStore, tt.links).Relate(t.Context(), request,
					declared("Store", sema.KindStruct), sema.ReferencedBy)
				assert.NoError(t, err, "Relate of the uses of Store")
				assert.Equal(t, cutShort(got.Caveats), tt.want, "the answer has the caveat of a short preload")
			})
		}

		t.Run("preloads the files that write the name of a method inside a function", func(t *testing.T) {
			t.Parallel()
			nested := "package a\n\ntype Store struct {\n\tsize int\n}\n\nfunc Outer() int {\n" +
				"func (s *Store) Inner() int { return s.size }\n\treturn 1\n}\n"
			files := map[string]string{"a.fake": nested}
			for i := range 201 {
				files[fmt.Sprintf("other/f%d.fake", i)] = "var _ = Inner\n"
			}
			root := lsptest.Workspace(t, files)
			outlined, err := lsptest.Parser(root).Outline(t.Context(), engine.Request{Scope: "a.fake"})
			assert.NoError(t, err, "the outline of a.fake")
			at := slices.IndexFunc(outlined.Items, func(s sema.Symbol) bool { return s.Name == "Inner" })
			assert.InRange(t, at, 0, 1<<63, "the outline of a.fake declares Inner")
			server := lsptest.Server(lsptest.Scoped)
			server.Scoped = true
			got, err := lsptest.Parsing(t, root, server).Relate(t.Context(),
				engine.Request{
					Scope:    "a.fake",
					Declared: outlined.Items[at].Span,
				}, outlined.Items[at].ID, sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Inner")
			assert.True(t, cutShort(got.Caveats), "the answer has the caveat of a short preload")
		})

		t.Run("leaves out a use whose definition is another declaration from a related server", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Redeclares)
			server.Related = true
			got, err := lsptest.Engine(t, lsptest.Workspace(t, sample()), server).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store")
			assert.Equal(t, edges(got.Items), []string{"Get"}, "the declarations that use Store")
		})

		t.Run("keeps a use whose definition is another declaration from a server that is not related",
			func(t *testing.T) {
				t.Parallel()
				got, err := serving(t, lsptest.Redeclares, sample()).Relate(t.Context(), request,
					declared("Store", sema.KindStruct), sema.ReferencedBy)
				assert.NoError(t, err, "Relate of the uses of Store")
				assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
			})

		t.Run("opens no other file for a relation from the declaration", func(t *testing.T) {
			t.Parallel()
			store := declared("Store", sema.KindStruct)
			got, err := writing(t, useOfStore).Relate(t.Context(), request, store, sema.References)
			assert.NoError(t, err, "Relate of what Store refers to")
			assert.False(t, cutShort(got.Caveats), "the answer has the caveat of a short preload")
		})

		t.Run("returns the relations up to the limit of the request", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake", Limit: 1}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store with a limit of 1")
			assert.Equal(t, edges(got.Items), []string{"Get"}, "the declarations that use Store")
		})

		t.Run("counts the relations past the limit in a caveat", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake", Limit: 1}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Store with a limit of 1")
			var notes []string
			for _, one := range got.Caveats {
				if one.Code == trust.CaveatTruncated {
					notes = append(notes, one.Note)
				}
			}
			assert.Equal(t, notes, []string{"1 of 2 relations returned"}, "the notes of the truncation caveats")
		})

		t.Run("reads no file of a relation past the limit", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "requests")
			e := serving(t, lsptest.Minified, map[string]string{
				"a.fake": lsptest.Bundle(3), "b.fake": "package a\nfunc Caller() int { return F0() }\n",
			}, lsptest.RecordRequests(log))
			got, err := e.Relate(t.Context(), engine.Request{Scope: "a.fake", Limit: 1},
				declared("F0", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of F0 with a limit of 1")
			assert.Equal(t, edges(got.Items), []string{"After"}, "the declarations that use F0")
			requests, err := os.ReadFile(log)
			assert.NoError(t, err, "the log of the requests")
			assert.Equal(t, strings.Count(string(requests), "textDocument/documentSymbol\n"), 1,
				"the requests for symbols, of a.fake only")
		})

		t.Run("returns the callers from the call hierarchy", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store.Get", sema.KindMethod), sema.CalledBy)
			assert.NoError(t, err, "Relate of the callers of Get")
			assert.Equal(t, edges(got.Items), []string{"After"}, "the callers of Get")
		})

		t.Run("leaves out a caller whose definition is another declaration from a related server", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Redeclares)
			server.Related = true
			got, err := lsptest.Engine(t, lsptest.Workspace(t, sample()), server).Relate(t.Context(), request,
				declared("Store.Get", sema.KindMethod), sema.CalledBy)
			assert.NoError(t, err, "Relate of the callers of Get")
			assert.Empty(t, got.Items, "the callers of Get")
		})

		t.Run("returns the callers that the outline engine reads from a server without the call hierarchy",
			func(t *testing.T) {
				t.Parallel()
				e := lsptest.Parsing(t, lsptest.Workspace(t, map[string]string{
					"a.fake": lsptest.Bundle(3),
					"b.fake": "package a\nvar Held = F0\nfunc Caller() int { return F0() }\n",
				}), lsptest.Server(lsptest.Uncalled))
				got, err := e.Relate(t.Context(), request, declared("F0", sema.KindFunction), sema.CalledBy)
				assert.NoError(t, err, "Relate of the callers of F0")
				assert.Equal(t, edges(got.Items), []string{"After", "Caller"}, "the callers of F0")
			})

		t.Run("returns the calls that the outline engine reads from a server without the call hierarchy",
			func(t *testing.T) {
				t.Parallel()
				e := lsptest.Parsing(t, lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Bundle(3)}),
					lsptest.Server(lsptest.Uncalled))
				got, err := e.Relate(t.Context(), request, declared("After", sema.KindFunction), sema.Calls)
				assert.NoError(t, err, "Relate of the calls of After")
				assert.Equal(t, edges(got.Items), []string{"F0"}, "the declarations that After calls")
			})

		t.Run("leaves out a call outside the declaration from a server without the call hierarchy",
			func(t *testing.T) {
				t.Parallel()
				other := lsptest.Bundle(3) + "func Other() int { return F1() }\n"
				e := lsptest.Parsing(t, lsptest.Workspace(t, map[string]string{"a.fake": other}),
					lsptest.Server(lsptest.Uncalled))
				got, err := e.Relate(t.Context(), request, declared("After", sema.KindFunction), sema.Calls)
				assert.NoError(t, err, "Relate of the calls of After")
				assert.Length(t, got.Items, 1, "the calls of After")
			})

		t.Run("returns ErrDecline for callers from a server without the call hierarchy and no outline engine",
			func(t *testing.T) {
				t.Parallel()
				e := serving(t, lsptest.Uncalled, map[string]string{"a.fake": lsptest.Bundle(3)})
				_, err := e.Relate(t.Context(), request, declared("F0", sema.KindFunction), sema.CalledBy)
				assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
			})

		t.Run("returns the call site of an outgoing call", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("After", sema.KindFunction), sema.Calls)
			assert.NoError(t, err, "Relate of the calls of After")
			assert.Equal(t, edges(got.Items), []string{"Get"}, "the declarations that After calls")
			assert.Equal(t, got.Items[0].At.Path, source.Path("a.fake"), "the file of the call site")
			assert.Equal(t, got.Items[0].At.Start.Line, 8, "the line of the call site")
			assert.Equal(t, got.Items[0].At.Start.Column, 37, "the column of the call site")
			assert.Equal(t, got.Items[0].Via, "func After() int { return (&Store{}).Get() }",
				"the line of the call site")
		})

		t.Run("returns the call site in the file of the caller", func(t *testing.T) {
			t.Parallel()
			far := filepath.Join(lsptest.Workspace(t, map[string]string{"far.fake": lsptest.Content}), "far.fake")
			got, err := serving(t, lsptest.Default, sample(), lsptest.Outside(far)).Relate(t.Context(), request,
				declared("After", sema.KindFunction), sema.Calls)
			assert.NoError(t, err, "Relate of the calls of After into "+far)
			assert.Equal(t, got.Items[0].At.Path, source.Path("a.fake"), "the file of the call site")
			assert.Equal(t, got.Items[0].At.Start.Line, 8, "the line of the call site")
		})

		t.Run("returns the implementations of a type", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ImplementedBy)
			assert.NoError(t, err, "Relate of the implementations of Store")
			assert.Equal(t, edges(got.Items), []string{"Store"}, "the implementations of Store")
		})

		t.Run("returns only the implementations at the name of a class from a contextual server", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Contextual)
			server.Contextual = true
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Getter})
			got, err := lsptest.Parsing(t, root, server).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ImplementedBy)
			assert.NoError(t, err, "Relate of the implementations of Store")
			assert.Equal(t, edges(got.Items), []string{"Store"}, "the implementations of Store")
		})

		t.Run("returns the implementations in the other projects that use the declaration from tsserver",
			func(t *testing.T) {
				t.Parallel()
				server := lsptest.Server(lsptest.Projected)
				server.Scoped, server.Tsserver = true, true
				got, err := lsptest.Parsing(t, lsptest.Workspace(t, projected()), server).Relate(t.Context(), request,
					declared("Store", sema.KindStruct), sema.ImplementedBy)
				assert.NoError(t, err, "Relate of the implementations of Store")
				assert.Equal(t, edges(got.Items), []string{"Store", "Vault"}, "the implementations of Store")
			})

		t.Run("returns the implementations in the project of the declaration from a server that is not tsserver",
			func(t *testing.T) {
				t.Parallel()
				server := lsptest.Server(lsptest.Projected)
				server.Scoped = true
				got, err := lsptest.Parsing(t, lsptest.Workspace(t, projected()), server).Relate(t.Context(), request,
					declared("Store", sema.KindStruct), sema.ImplementedBy)
				assert.NoError(t, err, "Relate of the implementations of Store")
				assert.Equal(t, edges(got.Items), []string{"Store"}, "the implementations of Store")
			})

		t.Run("returns every implementation from a server that is not contextual", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Getter})
			got, err := lsptest.Parsing(t, root, lsptest.Server(lsptest.Contextual)).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ImplementedBy)
			assert.NoError(t, err, "Relate of the implementations of Store")
			assert.Equal(t, edges(got.Items), []string{"Store", "After", "Getter"}, "the implementations of Store")
		})

		t.Run("returns the supertypes of a type", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.Embeds)
			assert.NoError(t, err, "Relate of the supertypes of Store")
			assert.Equal(t, edges(got.Items), []string{"Store"}, "the supertypes of Store")
		})

		t.Run("declines a relation that the server answers with other relations", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Default)
			server.Unrelated = []sema.RelationKind{sema.Embeds}
			_, err := lsptest.Engine(t, lsptest.Workspace(t, sample()), server).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.Embeds)
			assert.ErrorIs(t, err, engine.ErrDecline, "Relate of the supertypes of Store")
		})

		t.Run("returns the subtypes of a type", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.EmbeddedBy)
			assert.NoError(t, err, "Relate of the subtypes of Store")
			assert.Equal(t, edges(got.Items), []string{"After"}, "the subtypes of Store")
		})

		t.Run("relates a use outside every declaration to its file", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Unenclosed, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of a use on line 0")
			assert.Equal(t, got.Items[0].To.Kind, sema.KindFile, "the kind of the far end")
			assert.Contains(t, sema.Kinds(), got.Items[0].To.Kind, "sema.Kinds contains the kind")
		})

		t.Run("relates a use in a file outside the workspace", func(t *testing.T) {
			t.Parallel()
			far := filepath.Join(lsptest.Workspace(t, map[string]string{"far.fake": lsptest.Content}), "far.fake")
			got, err := serving(t, lsptest.Default, sample(), lsptest.Outside(far)).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of uses in "+far)
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
		})

		t.Run("declines imports", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.Imports)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
		})

		t.Run("declines a declaration the server cannot call", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Uncallable, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.CalledBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
			assert.Contains(t, err.Error(), "not a function", "the error of Relate")
		})

		t.Run("refuses a question that the server responds to with an error", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Untyped, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ImplementedBy)
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of Relate")
			assert.Contains(t, err.Error(), lsptest.NotAType, "the error of Relate")
		})

		t.Run("declines a question that the server does not answer in time", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Hangs)
			server.Answering = 300 * time.Millisecond
			e := lsptest.Engine(t, lsptest.Workspace(t, sample()), server)
			_, err := e.Relate(t.Context(), request, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
			assert.Contains(t, err.Error(), "did not answer within 300ms", "the error of Relate")
		})

		t.Run("declines a question whose content the server reports modified", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Cancels, sample()).Relate(t.Context(), request,
				declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
			assert.Contains(t, err.Error(), "content was modified", "the error of Relate")
		})

		t.Run("declines a declaration that no file declares", func(t *testing.T) {
			t.Parallel()
			_, err := serving(t, lsptest.Default, sample()).Relate(t.Context(), request,
				declared("Missing", sema.KindStruct), sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of Relate")
		})

		t.Run("asks the server at the declared span when no symbol matches the ID", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, limited()).Relate(t.Context(),
				engine.Request{Scope: "a.fake", Declared: limit()},
				declared("Limit", sema.KindConstant), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Limit")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that the server names")
		})

		t.Run("reads the symbols of the declared file alone", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "requests")
			files := limited()
			files["b.fake"], files["c.fake"] = lsptest.Content, lsptest.Content
			e := serving(t, lsptest.Default, files, lsptest.RecordRequests(log))
			_, err := e.Relate(t.Context(), engine.Request{Scope: ".", Declared: limit()},
				declared("Limit", sema.KindConstant), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of Limit")
			assert.Equal(t, requested(t, log, "textDocument/documentSymbol"), 1, "the requests for symbols")
		})

		t.Run("reads the declaration at a site through the outline engine", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "requests")
			e := lsptest.Parsing(t, lsptest.Workspace(t, used()),
				lsptest.Server(lsptest.Minified, lsptest.RecordRequests(log)))
			_, err := e.Relate(t.Context(), request, declared("F0", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of F0")
			assert.Equal(t, requested(t, log, "textDocument/documentSymbol"), 1,
				"the requests for symbols, of a.fake only")
		})

		t.Run("leaves out a second declaration of the declaration", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{
				"a.fake": lsptest.Bundle(3), "c.fake": "package a\nfunc F0() int { return 1 }\n",
			}
			e := lsptest.Parsing(t, lsptest.Workspace(t, files), lsptest.Server(lsptest.Minified))
			got, err := e.Relate(t.Context(), engine.Request{Scope: "."}, declared("F0", sema.KindFunction),
				sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of F0")
			assert.Equal(t, edges(got.Items), []string{"After"}, "the declarations that use F0")
		})

		t.Run("relates a use inside a local declaration to the function", func(t *testing.T) {
			t.Parallel()
			e := lsptest.Parsing(t, lsptest.Workspace(t, used()), lsptest.Server(lsptest.Minified))
			got, err := e.Relate(t.Context(), request, declared("F0", sema.KindFunction), sema.ReferencedBy)
			assert.NoError(t, err, "Relate of the uses of F0")
			assert.Equal(t, edges(got.Items), []string{"After", "Use"}, "the declarations that use F0")
		})

		t.Run("skips a scope without a file of the language", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Default, map[string]string{"notes.md": "# notes\n"}).
				Relate(t.Context(), engine.Request{Scope: "."}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate in a scope without a file of the language")
			assert.True(t, got.Skipped, "Skipped of the answer")
		})
	})
}
