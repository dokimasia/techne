// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package conformance

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

// Suite is the input of [Run]: the declaration, the grammar, the server, the
// registration and the fixture of one language module.
type Suite struct {
	// Declaration is the declaration the module registers.
	Declaration lang.Declaration

	// Grammar is the grammar the module supplies.
	Grammar treesitter.Grammar

	// Server is the language server the module declares.
	Server lsp.Server

	// Register is the Register function of the module.
	Register func(lang.Workspace, *lang.Registry, *engine.Catalog) error

	// Files is the fixture source, keyed by path relative to the workspace
	// root. Every path has an extension of Declaration.
	Files map[string]string

	// Declares lists every declaration of Files. Run compares it with the
	// outline as a set, so a missing and an extra declaration both fail, and
	// the fixture contains every declaration form of the language.
	Declares []Declared

	// Unclaimed is a path with an extension that the language does not
	// claim, or empty.
	Unclaimed string
}

// Declared is one declaration of a fixture. Run compares Name, Kind and
// Visibility for every declaration, and each other field when it is set.
type Declared struct {
	Name string
	Kind sema.Kind
	// Visibility is the visibility the engine reports. A language that
	// declares visibility with a modifier reports sema.VisibilityUnknown.
	Visibility sema.Visibility
	// Signature is the declaration without its body.
	Signature string
	// Doc is the documentation the fixture writes on the declaration. When
	// any Doc is set, Run compares the documented declarations as a set.
	Doc string
	// Annotations are annotations of the declaration.
	Annotations []Annotated
	// Modifiers are modifier keywords of the declaration.
	Modifiers []string
	// Simple is the name of an import without its qualifier and extension,
	// such as json for encoding/json and stdio for <stdio.h>. Run requires
	// it for every import, and checks that Relate finds the import by it.
	Simple string
}

// Annotated is one annotation of a fixture declaration. Run compares Text
// when it is set.
type Annotated struct {
	Name string
	Text string
}

// Run runs every check against the module that s describes. Each group of
// checks runs as a parallel subtest named after the method it tests.
func Run(t *testing.T, s Suite) {
	t.Helper()

	fsys := fstest.MapFS{}
	for path, content := range s.Files {
		fsys[path] = &fstest.MapFile{Data: []byte(content)}
	}
	if s.Unclaimed != "" {
		fsys[s.Unclaimed] = &fstest.MapFile{Data: []byte("not this language\n")}
	}

	t.Run("Register", func(t *testing.T) {
		t.Parallel()
		registry, catalogue := lang.NewRegistry(), engine.NewCatalog()
		e := build(t, fsys, s)
		assert.NoError(t, registry.Register(catalogue, s.Declaration, e), "Register")

		t.Run("routes every extension to the language", func(t *testing.T) {
			t.Parallel()
			for _, suffix := range s.Declaration.Extensions {
				got, ok := registry.LanguageOf(source.Path("a/b" + suffix))
				assert.True(t, ok, suffix)
				assert.Equal(t, got, s.Declaration.Language, suffix)
			}
		})

		t.Run("adds an engine the catalogue selects for RoleOutline", func(t *testing.T) {
			t.Parallel()
			assert.Length(t, catalogue.For(t.Context(), s.Declaration.Language, engine.RoleOutline), 1, "engines")
		})

		t.Run("names the engine after the language", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, e.Name(), string(s.Declaration.Language), "Name")
		})

		t.Run("registers the language without its server over a tree in memory", func(t *testing.T) {
			t.Parallel()
			r, c := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, s.Register(lang.Workspace{FS: fsys}, r, c), "the Register of the module")
			assert.Equal(t, r.Languages(), []source.Language{s.Declaration.Language}, "the registered languages")
			assert.False(t, catalogued(t, c, s.Server.Name), "the catalogue contains "+s.Server.Name)
		})

		t.Run("registers the server for a workspace on disk", func(t *testing.T) {
			t.Parallel()
			r, c := lang.NewRegistry(), engine.NewCatalog()
			root := t.TempDir()
			assert.NoError(t, s.Register(lang.Workspace{FS: os.DirFS(root), Root: root}, r, c),
				"the Register of the module")
			assert.True(t, catalogued(t, c, s.Server.Name), "the catalogue contains "+s.Server.Name)
		})

		t.Run("refuses a second registration of the language", func(t *testing.T) {
			t.Parallel()
			r, c := lang.NewRegistry(), engine.NewCatalog()
			assert.NoError(t, s.Register(lang.Workspace{FS: fsys}, r, c), "the first Register")
			assert.HasError(t, s.Register(lang.Workspace{FS: fsys}, r, c), "the second Register")
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("passes Server.Valid", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, s.Server.Valid(), "Valid of "+s.Server.Name)
		})

		t.Run("names the program it runs", func(t *testing.T) {
			t.Parallel()
			assert.NotEmpty(t, s.Server.Command, "the command of "+s.Server.Name)
			assert.Equal(t, s.Server.Command[0], s.Server.Name, "the program of "+s.Server.Name)
		})

		t.Run("claims Resolved for resolve", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, s.Server.Fidelity(engine.RoleResolve), trust.Resolved, "the tier of RoleResolve")
		})

		t.Run("claims no tier for outline or search", func(t *testing.T) {
			t.Parallel()
			for _, role := range []engine.Role{engine.RoleOutline, engine.RoleSearch} {
				assert.Equal(t, s.Server.Fidelity(role), trust.None, "the tier of "+role.String())
			}
		})
	})

	t.Run("ProjectOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the directory of each manifest the language declares", func(t *testing.T) {
			t.Parallel()
			assert.NotEmpty(t, s.Declaration.Manifests, "the manifests of the declaration")
			file := source.Path("project/src/a" + s.Declaration.Extensions[0])
			for _, manifest := range s.Declaration.Manifests {
				marker := strings.ReplaceAll(manifest, "*", "App")
				tree := fstest.MapFS{
					"project/" + marker: &fstest.MapFile{},
					string(file):        &fstest.MapFile{},
				}
				assert.Equal(t, lang.ProjectOf(tree, file, s.Declaration.Manifests), source.Path("project"),
					"the project of "+string(file)+" beside "+marker)
			}
		})
	})

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns ErrUnknownCapture for a capture without a kind", func(t *testing.T) {
			t.Parallel()
			spoiled := s.Grammar
			spoiled.Tags = s.Grammar.Tags + "\n((_) @definition.no_such_shape)"
			_, err := treesitter.New(fsys, s.Declaration, spoiled)
			assert.ErrorIs(t, err, treesitter.ErrUnknownCapture, "New")
		})
	})

	t.Run("Outline", func(t *testing.T) {
		t.Parallel()
		e := build(t, fsys, s)
		got := outline(t, e, ".")

		t.Run("returns every declaration of the fixture", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, summarise(got.Items), expected(s.Declares), "declarations")
		})

		t.Run("returns only complete symbols", func(t *testing.T) {
			t.Parallel()
			for _, sym := range got.Items {
				assert.NotEmpty(t, string(sym.ID), "ID of "+sym.Name)
				assert.NotEmpty(t, sym.Name, "Name of "+string(sym.ID))
				assert.NotEqual(t, sym.Kind, sema.KindUnknown, "Kind of "+sym.Name)
				assert.Equal(t, sym.Language, s.Declaration.Language, "Language of "+sym.Name)
				assert.NotEmpty(t, string(sym.Span.Path), "Path of "+sym.Name)
			}
		})

		t.Run("returns the text of each span as its snippet", func(t *testing.T) {
			t.Parallel()
			for _, sym := range got.Items {
				content, ok := s.Files[string(sym.Span.Path)]
				if !ok {
					continue
				}
				assert.True(t, sym.Span.End.Offset <= len(content), "span of "+sym.Name)
				assert.Equal(t, sym.Snippet, content[sym.Span.Start.Offset:sym.Span.End.Offset], "snippet of "+sym.Name)
				assert.Contains(t, sym.Snippet, sym.Name, "snippet of "+sym.Name)
			}
		})

		t.Run("links each declaration to its innermost container", func(t *testing.T) {
			t.Parallel()
			for i, sym := range got.Items {
				assert.Equal(t, sym.Parent, innermost(got.Items, i), "Parent of "+sym.Name)
			}
		})

		t.Run("qualifies each member by the qualified name of its container", func(t *testing.T) {
			t.Parallel()
			for _, sym := range got.Items {
				want := sym.Name
				switch {
				case sym.Kind == sema.KindImport:
				case sym.Parent != "":
					want = sema.Qualify(sym.Parent.Name(), sym.Name)
				case sym.Kind == sema.KindMethod && strings.HasSuffix(sym.ID.Name(), "."+sym.Name):
					// A receiver qualifies a method at the top level of a file.
					continue
				}
				assert.Equal(t, sym.ID.Name(), want, "the qualified name of the "+sym.Kind.String()+" "+sym.Name)
			}
		})

		t.Run("gives two members of one name two identities", func(t *testing.T) {
			t.Parallel()
			pairs := 0
			for i, one := range got.Items {
				for _, other := range got.Items[i+1:] {
					if one.Name == other.Name && one.Kind == other.Kind && one.Span.Path == other.Span.Path &&
						apart(one, other) {
						pairs++
						assert.NotEqual(t, one.ID, other.ID, "the IDs of the two "+one.Kind.String()+"s "+one.Name)
					}
				}
			}
			assert.True(t, pairs > 0, "the fixture declares two members of one name and kind in one file")
		})

		t.Run("reports total coverage", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "completeness")
		})

		t.Run("adds the caveat for matched text", func(t *testing.T) {
			t.Parallel()
			assert.True(t, slices.ContainsFunc(got.Caveats, func(c trust.Caveat) bool {
				return c.Code == trust.CaveatDynamic
			}), "caveats")
		})

		t.Run("publishes syntactic evidence without a negative claim", func(t *testing.T) {
			t.Parallel()
			published := engine.Publish(got, e, engine.RoleOutline, trust.None)
			assert.Equal(t, published.Provenance.Fidelity, trust.Syntactic, "fidelity")
			assert.False(t, published.Provenance.SupportsNegativeClaim(), "negative claim")
			assert.True(t, published.Status.Answered(), "answered")
		})

		t.Run("returns the metadata the fixture states", func(t *testing.T) {
			t.Parallel()
			for _, want := range s.Declares {
				// A name and kind can repeat, as an interface and the class
				// that implements it both declare a method, so the check
				// passes when one declaration of the name and kind has the
				// metadata.
				matching := every(got.Items, want)
				if len(matching) == 0 {
					continue
				}
				for _, annotation := range want.Annotations {
					assert.True(t, anyAnnotated(matching, annotation), want.Name+" @"+annotation.Name)
				}
				if want.Signature != "" {
					assert.True(t, anySigned(matching, want.Signature), "signature of "+want.Name)
				}
				for _, keyword := range want.Modifiers {
					assert.True(t, anyModified(matching, keyword), want.Name+" "+keyword)
				}
			}
		})

		t.Run("returns the documentation the fixture writes", func(t *testing.T) {
			t.Parallel()
			if !documents(s.Declares) {
				t.Skip("the fixture writes no documentation")
			}
			assert.Equal(t, documented(got.Items), expectedDocs(s.Declares), "documentation")
		})

		t.Run("returns identical items for identical requests", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, outline(t, e, ".").Items, got.Items, "items")
		})

		t.Run("opens the root .gitignore once", func(t *testing.T) {
			t.Parallel()
			ignoring := maps.Clone(fsys)
			ignoring[".gitignore"] = &fstest.MapFile{Data: []byte("generated/\n")}
			counted := &counting{FS: ignoring, opens: map[string]int{}}
			outline(t, build(t, counted, s), ".")
			assert.Equal(t, counted.opened(".gitignore"), 1, ".gitignore opens")
		})

		t.Run("returns no items for a file of another language", func(t *testing.T) {
			t.Parallel()
			if s.Unclaimed == "" {
				t.Skip("the module supplies no unclaimed path")
			}
			assert.Empty(t, outline(t, e, source.Path(s.Unclaimed)).Items, "items")
		})

		grown := enlarged(fsys, s)
		large := outline(t, build(t, grown, s), ".")

		t.Run("returns every declaration beside a file larger than Largest", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, summarise(large.Items), expected(s.Declares), "declarations")
		})

		t.Run("reports a file larger than Largest as unread", func(t *testing.T) {
			t.Parallel()
			var unread []source.Path
			for _, c := range large.Caveats {
				if c.Code == trust.CaveatUnread {
					unread = append(unread, c.Paths...)
				}
			}
			assert.Equal(t, unread, []source.Path{source.Path(oversized(s))}, "unread")
			assert.Equal(t, large.Completeness, trust.ScopePartial, "completeness")
		})

		t.Run("does not skip a scope with only a file larger than Largest", func(t *testing.T) {
			t.Parallel()
			alone := outline(t, build(t, grown, s), source.Path(oversized(s)))
			assert.False(t, alone.Skipped, "Skipped")
		})
	})

	t.Run("Search", func(t *testing.T) {
		t.Parallel()
		e := build(t, fsys, s)
		everything := outline(t, e, ".")

		t.Run("returns a declaration by its exact name", func(t *testing.T) {
			t.Parallel()
			if len(s.Declares) == 0 {
				t.Skip("the fixture declares nothing")
			}
			wanted := s.Declares[0]
			got := search(t, e, engine.Query{Text: wanted.Name, Private: true})
			assert.True(t, found(got.Items, wanted), wanted.Name)
		})

		t.Run("returns an exact match first", func(t *testing.T) {
			t.Parallel()
			if len(s.Declares) == 0 {
				t.Skip("the fixture declares nothing")
			}
			wanted := s.Declares[0]
			got := search(t, e, engine.Query{Text: wanted.Name, Private: true})
			assert.NotEmpty(t, got.Items, "items")
			assert.Equal(t, got.Items[0].Name, wanted.Name, "first item")
		})

		t.Run("leaves out an unexported declaration without Private", func(t *testing.T) {
			t.Parallel()
			hidden := slices.IndexFunc(s.Declares, func(d Declared) bool { return d.Visibility == sema.Unexported })
			if hidden < 0 {
				t.Skip("the fixture declares nothing unexported")
			}
			wanted := s.Declares[hidden]
			got := search(t, e, engine.Query{Text: wanted.Name})
			assert.Empty(t, every(got.Items, wanted), "the unexported "+wanted.Kind.String()+" "+wanted.Name)
		})

		t.Run("returns no items for a name no declaration has", func(t *testing.T) {
			t.Parallel()
			got := search(t, e, engine.Query{Text: "aNameNoModuleWouldDeclare", Private: true})
			assert.Empty(t, got.Items, "items")
			assert.False(t, engine.Publish(got, e, engine.RoleSearch, trust.None).Provenance.SupportsNegativeClaim(),
				"negative claim")
		})

		t.Run("returns identical items for identical requests", func(t *testing.T) {
			t.Parallel()
			q := engine.Query{Text: "e", Private: true}
			assert.Equal(t, search(t, e, q).Items, search(t, e, q).Items, "items")
		})

		t.Run("returns a truncation caveat at the limit", func(t *testing.T) {
			t.Parallel()
			if len(everything.Items) < 2 {
				t.Skip("the fixture declares fewer than two declarations")
			}
			got := search(t, e, engine.Query{Private: true, Limit: 1})
			assert.Length(t, got.Items, 1, "items")
			assert.Contains(t, got.Caveats, trust.Caveat{
				Code: trust.CaveatTruncated,
				Note: fmt.Sprintf("1 of %d matches returned", len(everything.Items)),
			}, "caveats")
		})

		missing := engine.Query{Text: "aNameNoModuleWouldDeclare", Private: true}
		first := slices.Sorted(maps.Keys(s.Files))[0]

		t.Run("skips an unchanged file without a match in a second search", func(t *testing.T) {
			t.Parallel()
			counted := &counting{FS: maps.Clone(fsys), opens: map[string]int{}}
			searched := build(t, counted, s)
			search(t, searched, missing)
			before := sources(counted, s)
			search(t, searched, missing)
			assert.Equal(t, sources(counted, s), before, "the reads of the fixture files")
		})

		t.Run("reads a file again after its size changes", func(t *testing.T) {
			t.Parallel()
			files := maps.Clone(fsys)
			counted := &counting{FS: files, opens: map[string]int{}}
			searched := build(t, counted, s)
			search(t, searched, missing)
			before := counted.opened(first)
			files[first] = &fstest.MapFile{Data: []byte(s.Files[first] + "\n")}
			search(t, searched, missing)
			assert.Equal(t, counted.opened(first), before+1, "the reads of "+first)
		})

		t.Run("reads a file again after its modification time changes", func(t *testing.T) {
			t.Parallel()
			files := maps.Clone(fsys)
			counted := &counting{FS: files, opens: map[string]int{}}
			searched := build(t, counted, s)
			search(t, searched, missing)
			before := counted.opened(first)
			files[first] = &fstest.MapFile{Data: []byte(s.Files[first]), ModTime: time.Unix(1, 0)}
			search(t, searched, missing)
			assert.Equal(t, counted.opened(first), before+1, "the reads of "+first)
		})
	})

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("returns Skipped for a scope without a file of the language", func(t *testing.T) {
			t.Parallel()
			elsewhere := fstest.MapFS{"docs/notes.none": {Data: []byte("text\n")}}
			got, err := build(t, elsewhere, s).Relate(t.Context(), engine.Request{Scope: "docs"}, "", sema.Calls)
			assert.NoError(t, err, "Relate")
			assert.True(t, got.Skipped, "Skipped")
		})

		t.Run("returns ErrDecline for a relation that needs name binding", func(t *testing.T) {
			t.Parallel()
			_, err := build(t, fsys, s).Relate(t.Context(), engine.Request{Scope: "."}, "", sema.Calls)
			assert.ErrorIs(t, err, engine.ErrDecline, "Relate")
		})

		e := build(t, fsys, s)
		declared := outline(t, e, engine.Root).Items
		imported := importsOf(s.Declares)

		t.Run("requires a simple name for every import", func(t *testing.T) {
			t.Parallel()
			assert.NotEmpty(t, imported, "the imports of the fixture")
			for _, d := range imported {
				assert.NotEmpty(t, d.Simple, "the simple name of "+d.Name)
				assert.Contains(t, d.Name, d.Simple, "the name of the import "+d.Simple)
			}
		})

		t.Run("returns the files of an import by its name", func(t *testing.T) {
			t.Parallel()
			for _, d := range imported {
				finds(t, e, declared, d.Name, d)
			}
		})

		t.Run("returns the files of an import by its simple name", func(t *testing.T) {
			t.Parallel()
			for _, d := range imported {
				finds(t, e, declared, d.Simple, d)
			}
		})

		t.Run("returns no file for an import with another simple name", func(t *testing.T) {
			t.Parallel()
			for _, d := range imported {
				at := strings.LastIndex(d.Name, d.Simple)
				if at < 0 {
					continue
				}
				other := d.Name[:at] + unrelated + d.Name[at+len(d.Simple):]
				assert.Empty(t, importedBy(t, e, other), "the files that import "+other)
			}
		})

		t.Run("returns no file for an import with another qualifier", func(t *testing.T) {
			t.Parallel()
			for _, d := range imported {
				other := unrelated + "/" + d.Simple
				assert.Empty(t, importedBy(t, e, other), "the files that import "+other)
			}
		})

		t.Run("returns one relation for each line that imports a name", func(t *testing.T) {
			t.Parallel()
			for _, d := range imported {
				lines := map[string]int{}
				for _, edge := range importedBy(t, e, d.Simple) {
					lines[fmt.Sprintf("%s:%d", edge.At.Path, edge.At.Start.Line+1)]++
				}
				for line, count := range lines {
					assert.Equal(t, count, 1, "the relations of "+d.Simple+" at "+line)
				}
			}
		})

		t.Run("returns each importing file with the line of its import", func(t *testing.T) {
			t.Parallel()
			for _, d := range imported {
				for _, edge := range importedBy(t, e, d.Name) {
					assert.Equal(t, edge.To.Kind, sema.KindFile, "the kind of "+edge.To.Name)
					assert.Equal(t, edge.To.Name, string(edge.At.Path), "the file of the import at "+edge.Via)
					written := lineOf(s.Files[string(edge.At.Path)], edge.At.Start.Line)
					assert.Equal(t, edge.Via, strings.TrimSpace(written),
						"the line of the import of "+d.Name+" in "+edge.To.Name)
				}
			}
		})

		t.Run("returns the imports of the file that declares a name", func(t *testing.T) {
			t.Parallel()
			subject, ok := unique(declared)
			assert.True(t, ok, "a name the fixture declares once in a file with an import")
			got, err := e.Relate(t.Context(), engine.Request{Scope: engine.Root}, subject.ID, sema.Imports)
			assert.NoError(t, err, "Relate of the imports of "+subject.Name)
			assert.Equal(t, sortedList(farNames(got.Items)), sortedList(importsIn(declared, subject.Span.Path)),
				"the imports of "+string(subject.Span.Path))
		})
	})

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()
		// One engine over one mutable workspace, restored after each
		// declaration, because compiling a query per declaration costs more
		// than the rest of the suite.
		files := fstest.MapFS{}
		for path, content := range s.Files {
			files[path] = &fstest.MapFile{Data: []byte(content)}
		}
		e := build(t, files, s)
		before := outline(t, e, ".")

		t.Run("writes documentation that Outline reads back", func(t *testing.T) {
			for _, sym := range before.Items {
				documenting(t, e, files, s, before, sym)
			}
		})
	})

	t.Run("Check", func(t *testing.T) {
		t.Parallel()
		e := build(t, fsys, s)

		t.Run("returns no findings for the fixture", func(t *testing.T) {
			t.Parallel()
			got, err := e.Check(t.Context(), content(s))
			assert.NoError(t, err, "Check")
			assert.Empty(t, got.Items, "findings")
		})

		t.Run("returns an error finding for content that stops parsing", func(t *testing.T) {
			t.Parallel()
			spoiled := content(s)
			for path := range spoiled {
				spoiled[path] = append(spoiled[path], []byte(garbage)...)
			}
			got, err := e.Check(t.Context(), spoiled)
			assert.NoError(t, err, "Check")
			assert.NotEmpty(t, got.Items, "findings")
			for _, one := range got.Items {
				assert.Equal(t, one.Diagnostic.Severity, diag.SeverityError, "severity")
				assert.NotEmpty(t, one.Diagnostic.Message, "message")
				assert.NotEmpty(t, string(one.Diagnostic.Span.Path), "path")
				assert.NotEmpty(t, one.Diagnostic.Snippet, "snippet")
				assert.Empty(t, one.Fix, "fix")
			}
		})

		t.Run("returns ErrDecline for content of another language", func(t *testing.T) {
			t.Parallel()
			_, err := e.Check(t.Context(), map[source.Path][]byte{"a.no-language-claims-this": []byte("x")})
			assert.ErrorIs(t, err, engine.ErrDecline, "Check")
		})
	})

	t.Run("Index", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declarations of one file", func(t *testing.T) {
			t.Parallel()
			first := source.Path(slices.Sorted(maps.Keys(s.Files))[0])
			e := build(t, fsys, s)
			got, err := e.Index(t.Context(), first)
			assert.NoError(t, err, "Index of "+string(first))
			assert.False(t, got.Skipped, "Skipped of the index of "+string(first))
			assert.Equal(t, summarise(got.Items), summarise(outline(t, e, first).Items),
				"the declarations of "+string(first))
		})

		t.Run("skips a file of another language", func(t *testing.T) {
			t.Parallel()
			if s.Unclaimed == "" {
				t.Skip("the module supplies no unclaimed path")
			}
			got, err := build(t, fsys, s).Index(t.Context(), source.Path(s.Unclaimed))
			assert.NoError(t, err, "Index of "+s.Unclaimed)
			assert.True(t, got.Skipped, "Skipped of the index of "+s.Unclaimed)
			assert.Empty(t, got.Items, "the declarations of "+s.Unclaimed)
		})

		t.Run("returns LargeError for a file larger than Largest", func(t *testing.T) {
			t.Parallel()
			_, err := build(t, enlarged(fsys, s), s).Index(t.Context(), source.Path(oversized(s)))
			_, ok := errors.AsType[lang.LargeError](err)
			assert.True(t, ok, "LargeError")
		})

		t.Run("returns GeneratedError for a file the .gitignore excludes", func(t *testing.T) {
			t.Parallel()
			excluded := maps.Clone(fsys)
			excluded[".gitignore"] = &fstest.MapFile{Data: []byte("generated\n")}
			buried := "generated/" + oversized(s)
			excluded[buried] = &fstest.MapFile{Data: []byte(anyFile(s))}
			_, err := build(t, excluded, s).Index(t.Context(), source.Path(buried))
			_, ok := errors.AsType[lang.GeneratedError](err)
			assert.True(t, ok, "GeneratedError")
		})
	})
}

// enlarged returns fsys with a file larger than [lang.Largest] added under
// an extension of the language. The file contains newlines only, because no
// engine reads it.
func enlarged(fsys fstest.MapFS, s Suite) fstest.MapFS {
	out := maps.Clone(fsys)
	out[oversized(s)] = &fstest.MapFile{Data: bytes.Repeat([]byte("\n"), lang.Largest+1)}
	return out
}

// oversized returns the path of the file that enlarged adds.
func oversized(s Suite) string {
	return "toobig" + s.Declaration.Extensions[0]
}

// anyFile returns the content of the first fixture file in path order.
func anyFile(s Suite) string {
	for _, path := range slices.Sorted(maps.Keys(s.Files)) {
		return s.Files[path]
	}
	return ""
}

// documenting writes documentation onto one declaration through Plan,
// checks the result, and reads the documentation back through Outline. It
// skips a declaration that Plan refuses, such as a parameter, which no
// language documents on its own.
func documenting(
	t *testing.T,
	e *treesitter.Engine,
	files fstest.MapFS,
	s Suite,
	before engine.Result[sema.Symbol],
	sym sema.Symbol,
) {
	t.Helper()

	t.Run(fmt.Sprintf("%s %s", sym.Kind, sym.Name), func(t *testing.T) {
		original := s.Files[string(sym.Span.Path)]
		defer func() { files[string(sym.Span.Path)] = &fstest.MapFile{Data: []byte(original)} }()

		planned, err := e.Plan(t.Context(), engine.Request{Scope: sym.Span.Path},
			edit.DocumentSymbol,
			edit.Target{Kind: edit.TargetSpan, Span: sym.Span},
			edit.Args{edit.ArgDoc: written})
		if errors.Is(err, engine.ErrRefuse) {
			t.Skipf("the language does not document a %s here: %v", sym.Kind, err)
		}
		assert.NoError(t, err, "Plan")
		assert.Length(t, planned.Items, 1, "changes")

		changed := applied(t, original, planned.Items[0])
		files[string(sym.Span.Path)] = &fstest.MapFile{Data: []byte(changed)}

		faults, err := e.Check(t.Context(), map[source.Path][]byte{sym.Span.Path: []byte(changed)})
		assert.NoError(t, err, "Check")
		assert.Empty(t, faults.Items, "findings")

		after := outline(t, e, ".")
		assert.Equal(t, summarise(after.Items), summarise(before.Items), "declarations")
		assert.True(t, reads(after.Items, sym, written), "documentation of "+sym.Name)
	})
}

// written is the documentation the round trip writes. It has two
// paragraphs, so a form that keeps only the first line fails.
const written = "Documented by the suite.\n\nA second paragraph, so the form spans more than one line."

// applied returns content with the edits of c applied by [edit.Apply], as
// the write path applies them.
func applied(t *testing.T, content string, c edit.Change) string {
	t.Helper()
	assert.Equal(t, c.Kind, edit.ChangeEdit, "change kind")
	out, err := edit.Apply([]byte(content), c.Edits)
	assert.NoError(t, err, "Apply")
	return string(out)
}

// reads reports whether some declaration with the name and kind of sym has
// doc as its documentation. Writing documentation moves a declaration, so
// reads matches by name and kind, not by span.
func reads(found []sema.Symbol, sym sema.Symbol, doc string) bool {
	return slices.ContainsFunc(found, func(one sema.Symbol) bool {
		return one.Name == sym.Name && one.Kind == sym.Kind && one.Doc == doc
	})
}

// content returns the fixture as the files a gate receives.
func content(s Suite) map[source.Path][]byte {
	out := map[source.Path][]byte{}
	for path, text := range s.Files {
		out[source.Path(path)] = []byte(text)
	}
	return out
}

// garbage is punctuation from which none of the ten grammars parses a
// declaration.
const garbage = "\n)]}%\n"

// build returns the engine of s over fsys and closes it when the test ends.
func build(t *testing.T, fsys fs.FS, s Suite) *treesitter.Engine {
	t.Helper()
	e, err := treesitter.New(fsys, s.Declaration, s.Grammar)
	assert.NoError(t, err, "New")
	t.Cleanup(e.Close)
	return e
}

// outline returns the outline of a scope.
func outline(t *testing.T, e *treesitter.Engine, scope source.Path) engine.Result[sema.Symbol] {
	t.Helper()
	got, err := e.Outline(t.Context(), engine.Request{Scope: scope})
	assert.NoError(t, err, "Outline")
	return got
}

// search returns the result of a query over the workspace root.
func search(t *testing.T, e *treesitter.Engine, q engine.Query) engine.Result[sema.Symbol] {
	t.Helper()
	got, err := e.Search(t.Context(), engine.Request{Scope: "."}, q)
	assert.NoError(t, err, "Search")
	return got
}

// counting is a filesystem that counts the opens of each path. A stat opens
// nothing, so the opens of a file count its reads.
type counting struct {
	fs.FS
	mu    sync.Mutex
	opens map[string]int
}

func (c *counting) Open(name string) (fs.File, error) {
	c.mu.Lock()
	c.opens[name]++
	c.mu.Unlock()
	return c.FS.Open(name)
}

// Stat returns the file information of name without an open.
func (c *counting) Stat(name string) (fs.FileInfo, error) { return fs.Stat(c.FS, name) }

// opened returns the number of opens of name.
func (c *counting) opened(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.opens[name]
}

// sources returns the number of opens of the fixture files of s.
func sources(c *counting, s Suite) int {
	total := 0
	for path := range s.Files {
		total += c.opened(path)
	}
	return total
}

// innermost returns the ID of the smallest other symbol in the file of
// symbols[i] whose span contains it and is wider, the first of equal width.
// It returns an empty ID when there is none, and when that symbol has the
// ID of symbols[i]. It compares every pair.
func innermost(symbols []sema.Symbol, i int) sema.ID {
	best := -1
	for j, outer := range symbols {
		if j == i || !encloses(outer.Span, symbols[i].Span) {
			continue
		}
		if best < 0 || width(outer.Span) < width(symbols[best].Span) {
			best = j
		}
	}
	if best < 0 || symbols[best].ID == symbols[i].ID {
		return ""
	}
	return symbols[best].ID
}

// apart reports whether two declarations of one name and kind are members of
// different containers: their parents differ, or both are methods at the top
// level of a file. A method at the top level is a method of its receiver, and
// Go allows one method of a name for each receiver type.
func apart(one, other sema.Symbol) bool {
	return one.Parent != other.Parent || one.Parent == "" && one.Kind == sema.KindMethod
}

// encloses reports whether outer contains inner and is wider.
func encloses(outer, inner source.Span) bool {
	return outer.Path == inner.Path &&
		outer.Start.Offset <= inner.Start.Offset &&
		inner.End.Offset <= outer.End.Offset &&
		width(outer) > width(inner)
}

// width returns the number of bytes a span covers.
func width(s source.Span) int { return s.End.Offset - s.Start.Offset }

// summarise returns the kind, name and visibility of each symbol, sorted,
// so a failure shows both sets side by side.
func summarise(found []sema.Symbol) string {
	out := make([]string, 0, len(found))
	for _, sym := range found {
		out = append(out, fmt.Sprintf("%s %s/%s", sym.Kind, sym.Name, sym.Visibility))
	}
	return sortedList(out)
}

// expected returns the declarations of a fixture in the form of summarise.
func expected(want []Declared) string {
	out := make([]string, 0, len(want))
	for _, d := range want {
		out = append(out, fmt.Sprintf("%s %s/%s", d.Kind, d.Name, d.Visibility))
	}
	return sortedList(out)
}

// sortedList returns items sorted and joined in brackets.
func sortedList(items []string) string {
	sort.Strings(items)
	return "[" + strings.Join(items, ", ") + "]"
}

// every returns each symbol with the name and kind of want.
func every(in []sema.Symbol, want Declared) []sema.Symbol {
	var out []sema.Symbol
	for _, sym := range in {
		if sym.Name == want.Name && sym.Kind == want.Kind {
			out = append(out, sym)
		}
	}
	return out
}

// anyAnnotated reports whether a symbol of in has an annotation that
// matches want.
func anyAnnotated(in []sema.Symbol, want Annotated) bool {
	for _, sym := range in {
		for _, a := range sym.Annotations {
			if a.Name == want.Name && (want.Text == "" || a.Text == want.Text) {
				return true
			}
		}
	}
	return false
}

// anySigned reports whether a symbol of in has the signature.
func anySigned(in []sema.Symbol, signature string) bool {
	return slices.ContainsFunc(in, func(sym sema.Symbol) bool { return sym.Signature == signature })
}

// anyModified reports whether a symbol of in has the modifier keyword.
func anyModified(in []sema.Symbol, keyword string) bool {
	return slices.ContainsFunc(in, func(sym sema.Symbol) bool { return sym.Modified(keyword) })
}

// documents reports whether a fixture writes any documentation.
func documents(want []Declared) bool {
	return slices.ContainsFunc(want, func(d Declared) bool { return d.Doc != "" })
}

// documented returns each documented symbol with its documentation, sorted.
func documented(found []sema.Symbol) string {
	out := make([]string, 0, len(found))
	for _, sym := range found {
		if sym.Doc != "" {
			out = append(out, fmt.Sprintf("%s %s: %q", sym.Kind, sym.Name, sym.Doc))
		}
	}
	return sortedList(out)
}

// expectedDocs returns the documented declarations of a fixture in the form
// of documented.
func expectedDocs(want []Declared) string {
	out := make([]string, 0, len(want))
	for _, d := range want {
		if d.Doc != "" {
			out = append(out, fmt.Sprintf("%s %s: %q", d.Kind, d.Name, d.Doc))
		}
	}
	return sortedList(out)
}

// found reports whether a symbol with the name, kind and visibility of want
// is among the symbols.
func found(symbols []sema.Symbol, want Declared) bool {
	return slices.ContainsFunc(symbols, func(sym sema.Symbol) bool {
		return sym.Name == want.Name && sym.Kind == want.Kind && sym.Visibility == want.Visibility
	})
}

// catalogued reports whether an engine named name serves a role in c. It
// counts an engine that cannot run.
func catalogued(t *testing.T, c *engine.Catalog, name string) bool {
	t.Helper()
	return slices.ContainsFunc(c.Capabilities(t.Context()), func(one engine.Capability) bool {
		return one.Engine == name
	})
}

// unrelated is a segment that the fixtures do not write in an import.
const unrelated = "Unrelated"

// importsOf returns the imports of a fixture.
func importsOf(want []Declared) []Declared {
	var out []Declared
	for _, d := range want {
		if d.Kind == sema.KindImport {
			out = append(out, d)
		}
	}
	return out
}

// importedBy returns the relations of the files that import name, over the
// workspace root.
func importedBy(t *testing.T, e *treesitter.Engine, name string) []sema.Relation {
	t.Helper()
	of := sema.NewID(e.Language(), engine.Root, name, sema.KindImport)
	got, err := e.Relate(t.Context(), engine.Request{Scope: engine.Root}, of, sema.ImportedBy)
	assert.NoError(t, err, "Relate of the files that import "+name)
	return got.Items
}

// finds checks that the files that import asked include every file that
// declares the import d.
func finds(t *testing.T, e *treesitter.Engine, declared []sema.Symbol, asked string, d Declared) {
	t.Helper()
	got := farNames(importedBy(t, e, asked))
	for _, sym := range declared {
		if sym.Kind == sema.KindImport && sym.Name == d.Name {
			assert.Contains(t, got, string(sym.Span.Path), "the files that import "+asked)
		}
	}
}

// farNames returns the distinct names of the far ends of relations.
func farNames(relations []sema.Relation) []string {
	var out []string
	for _, edge := range relations {
		if !slices.Contains(out, edge.To.Name) {
			out = append(out, edge.To.Name)
		}
	}
	return out
}

// importsIn returns the distinct names of the imports of the file at p.
func importsIn(declared []sema.Symbol, p source.Path) []string {
	var out []string
	for _, sym := range declared {
		if sym.Kind == sema.KindImport && sym.Span.Path == p && !slices.Contains(out, sym.Name) {
			out = append(out, sym.Name)
		}
	}
	return out
}

// unique returns a declaration that is not an import, in a file with an
// import, whose name occurs once in declared.
func unique(declared []sema.Symbol) (sema.Symbol, bool) {
	count := map[string]int{}
	for _, sym := range declared {
		count[sym.Name]++
	}
	for _, sym := range declared {
		if sym.Kind != sema.KindImport && count[sym.Name] == 1 && len(importsIn(declared, sym.Span.Path)) > 0 {
			return sym, true
		}
	}
	return sema.Symbol{}, false
}

// lineOf returns the zero-based line n of content.
func lineOf(content string, n int) string {
	lines := strings.Split(content, "\n")
	if n < 0 || n >= len(lines) {
		return ""
	}
	return lines[n]
}
