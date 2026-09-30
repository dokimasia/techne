// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// The paths of the fixtures of a move. A move goes from the package src to the package dst,
// and user imports src.
const (
	movedFrom = "src/moved.go"
	movedTo   = "dst/moved.go"
	stayFile  = "src/stay.go"
	dstFile   = "dst/dst.go"
	userFile  = "user/user.go"
	// tagged is a file of user that the build constraints of the load exclude.
	tagged = "user/tagged.go"
	// sameTo is a destination in a new directory whose package has the name of src.
	sameTo = "other/src/moved.go"
	// extra is the build tag that the load of the checker does not set.
	extra = "extra"
)

// The files of a move of Moved, a function that uses Stay, which remains in src.
const (
	usingStay = "package src\n\n// Moved returns [Stay] plus one.\nfunc Moved() int { return Stay() + 1 }\n"
	stay      = "package src\n\n// Stay returns one.\nfunc Stay() int { return 1 }\n"
	base      = "package dst\n\n// Base returns zero.\nfunc Base() int { return 0 }\n"
	// stayMoved is usingStay after its move to dst.
	stayMoved = "package dst\n\nimport \"example.com/p/src\"\n\n" +
		"// Moved returns [src.Stay] plus one.\nfunc Moved() int { return src.Stay() + 1 }\n"
)

// The files of a move of Moved, a function that Twice, the package dst and user call.
const (
	called = "package src\n\n// Moved returns two.\nfunc Moved() int { return 2 }\n"
	twice  = "package src\n\n// Twice returns [Moved] twice.\nfunc Twice() int { return Moved() * 2 }\n"
	using  = "package dst\n\nimport \"example.com/p/src\"\n\n// Use returns [src.Moved].\n" +
		"func Use() int { return src.Moved() }\n"
	both = "package user\n\nimport \"example.com/p/src\"\n\n// Both returns [src.Moved] and [src.Twice].\n" +
		"func Both() int { return src.Moved() + src.Twice() }\n"
)

// excluded returns a file of the package named pkg that the build constraints of the load
// exclude, with body after its package clause.
func excluded(pkg, body string) string {
	return "//go:build " + extra + "\n\npackage " + pkg + "\n\n" + body
}

// callers returns the files in which Twice, the package dst and user call Moved.
func callers() map[string]string {
	return map[string]string{movedFrom: called, stayFile: twice, dstFile: using, userFile: both}
}

// sameName returns the files of a move of Moved into a package of the name src, with user as
// the file of user.
func sameName(user string) map[string]string {
	return map[string]string{movedFrom: called, stayFile: stay, userFile: user}
}

// moveTarget returns the target and the arguments of the move of from to dest.
func moveTarget(from, dest string) (edit.Target, edit.Args) {
	return edit.Target{Kind: edit.TargetFile, Path: source.Path(from)}, edit.Args{edit.ArgDestination: dest}
}

// planned returns the plan of the move of from to dest in the workspace at root.
func planned(t *testing.T, root, from, dest string) (engine.Result[edit.Change], error) {
	t.Helper()
	target, args := moveTarget(from, dest)
	return over(t, root).Plan(t.Context(), engine.Request{Scope: source.Path(from)}, edit.MoveFile, target, args)
}

// moved plans the move of from to dest in a module of files, applies the plan to the files on
// disk, and returns the directory with the plan.
func moved(t *testing.T, files map[string]string, from, dest string) (string, engine.Result[edit.Change]) {
	t.Helper()
	root := workspace(t, files)
	return root, applied(t, root, from, dest)
}

// applied plans the move of from to dest in the workspace at root, applies the plan to the
// files on disk, and returns the plan.
func applied(t *testing.T, root, from, dest string) engine.Result[edit.Change] {
	t.Helper()
	got, err := planned(t, root, from, dest)
	assert.NoError(t, err, "Plan of the move of "+from)
	for _, c := range got.Items {
		full := filepath.Join(root, filepath.FromSlash(string(c.Path)))
		switch c.Kind {
		case edit.ChangeEdit:
			content, err := os.ReadFile(full)
			assert.NoError(t, err, "ReadFile "+string(c.Path))
			applied, err := edit.Apply(content, c.Edits)
			assert.NoError(t, err, "Apply to "+string(c.Path))
			assert.NoError(t, os.WriteFile(full, applied, 0o644), "WriteFile "+string(c.Path))
		case edit.ChangeMove:
			arrived := filepath.Join(root, filepath.FromSlash(string(c.To)))
			assert.NoError(t, os.MkdirAll(filepath.Dir(arrived), 0o755), "MkdirAll "+string(c.To))
			assert.NoError(t, os.Rename(full, arrived), "Rename to "+string(c.To))
		default:
			t.Fatalf("the plan contains a change of kind %v", c.Kind)
		}
	}
	return got
}

// builds runs go vet over the module at root with tags, and fails the test with its output
// when the module does not build.
func builds(t *testing.T, root string, tags ...string) {
	t.Helper()
	args := []string{"vet"}
	if len(tags) > 0 {
		args = append(args, "-tags="+strings.Join(tags, ","))
	}
	vet := exec.CommandContext(t.Context(), "go", append(args, "./...")...)
	vet.Dir = root
	out, err := vet.CombinedOutput()
	assert.NoError(t, err, "go vet of the moved module: "+string(out))
}

// content returns the content of the file at name under root.
func content(t *testing.T, root, name string) string {
	t.Helper()
	held, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	assert.NoError(t, err, "ReadFile "+name)
	return string(held)
}

// changed returns the paths of the changes of a plan.
func changed(got engine.Result[edit.Change]) []source.Path {
	out := make([]source.Path, 0, len(got.Items))
	for _, c := range got.Items {
		out = append(out, c.Path)
	}
	return out
}

func TestMove(t *testing.T) {
	t.Parallel()

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("qualifies a declaration of the source in the moved file", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{movedFrom: usingStay, stayFile: stay, dstFile: base}
			root, _ := moved(t, files, movedFrom, movedTo)
			builds(t, root)
			assert.Equal(t, content(t, root, movedTo), stayMoved, "the moved file")
		})

		t.Run("drops the qualifier of the destination in the moved file", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{
				movedFrom: "package src\n\nimport \"example.com/p/dst\"\n\n// Moved returns [dst.Base].\n" +
					"func Moved() int { return dst.Base() }\n",
				stayFile: stay, dstFile: base,
			}
			root, _ := moved(t, files, movedFrom, movedTo)
			builds(t, root)
			assert.Equal(t, content(t, root, movedTo),
				"package dst\n\n// Moved returns [Base].\nfunc Moved() int { return Base() }\n", "the moved file")
		})

		t.Run("links a declaration of the source by its import path in the moved file", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{
				movedFrom: "package src\n\n// Moved returns two, where [Stay] returns one.\nfunc Moved() int { return 2 }\n",
				stayFile:  stay,
			}
			root, _ := moved(t, files, movedFrom, movedTo)
			assert.Contains(t, content(t, root, movedTo), "where [example.com/p/src.Stay] returns one",
				"the moved file")
		})

		t.Run("links a moved declaration by its import path in another file of the source", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{
				movedFrom: called,
				stayFile:  "package src\n\n// Stay returns one, and [Moved] two.\nfunc Stay() int { return 1 }\n",
			}
			root, _ := moved(t, files, movedFrom, movedTo)
			assert.Contains(
				t,
				content(t, root, stayFile),
				"and [example.com/p/dst.Moved] two",
				"the file of the source",
			)
		})

		t.Run("links a moved declaration by its import path in an importer without the import", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files[userFile] = "package user\n\nimport \"example.com/p/src\"\n\n// One returns [src.Twice], not [src.Moved].\n" +
				"func One() int { return src.Twice() }\n"
			root, _ := moved(t, files, movedFrom, movedTo)
			assert.Contains(t, content(t, root, userFile), "not [example.com/p/dst.Moved]", "the file of user")
		})

		t.Run("rewrites a link by the import path of the source", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["other/other.go"] = "package other\n\n// X is one, as [example.com/p/src.Moved] is two, " +
				"[example.com/p/src.Twice] is four and [example.com/q.Moved] is none.\nvar X = 1\n"
			files["dst/link.go"] = "package dst\n\n// Y is one, as [example.com/p/src.Moved] is two.\nvar Y = 1\n"
			root, _ := moved(t, files, movedFrom, movedTo)
			assert.Contains(t, content(t, root, "other/other.go"), "as [example.com/p/dst.Moved] is two, "+
				"[example.com/p/src.Twice] is four and [example.com/q.Moved] is none", "the links of another package")
			assert.Contains(t, content(t, root, "dst/link.go"), "as [Moved] is two", "the link of the destination")
		})

		t.Run("qualifies a moved declaration in another file of the source", func(t *testing.T) {
			t.Parallel()
			root, _ := moved(t, callers(), movedFrom, movedTo)
			builds(t, root)
			assert.Equal(t, content(t, root, stayFile), "package src\n\nimport \"example.com/p/dst\"\n\n"+
				"// Twice returns [dst.Moved] twice.\nfunc Twice() int { return dst.Moved() * 2 }\n",
				"the other file of the source")
		})

		t.Run("qualifies a moved type and a moved variable in another file of the source", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{
				movedFrom: "package src\n\n// Kind is a kind.\ntype Kind int\n\n// Default is the default kind.\n" +
					"var Default Kind = 1\n\nvar _ Kind = 2\n\n// String returns kind.\n" +
					"func (k Kind) String() string { return \"kind\" }\n",
				stayFile: "package src\n\nfunc Name() string { return Default.String() }\n\nfunc Of() Kind { return Default }\n",
			}
			root, _ := moved(t, files, movedFrom, movedTo)
			builds(t, root)
			assert.Equal(t, content(t, root, stayFile), "package src\n\nimport \"example.com/p/dst\"\n\n"+
				"func Name() string { return dst.Default.String() }\n\nfunc Of() dst.Kind { return dst.Default }\n",
				"the other file of the source")
		})

		t.Run("drops the qualifier in the destination package", func(t *testing.T) {
			t.Parallel()
			root, _ := moved(t, callers(), movedFrom, movedTo)
			assert.Equal(t, content(t, root, dstFile),
				"package dst\n\n// Use returns [Moved].\nfunc Use() int { return Moved() }\n", "the file of dst")
		})

		t.Run("imports the destination in a file that imports the source", func(t *testing.T) {
			t.Parallel()
			root, _ := moved(t, callers(), movedFrom, movedTo)
			assert.Equal(t, content(t, root, userFile), "package user\n\nimport (\n\t\"example.com/p/dst\"\n"+
				"\t\"example.com/p/src\"\n)\n\n// Both returns [dst.Moved] and [src.Twice].\n"+
				"func Both() int { return dst.Moved() + src.Twice() }\n", "the file of user")
		})

		t.Run("writes the name under which a file imports the destination", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files[userFile] = "package user\n\nimport (\n\td \"example.com/p/dst\"\n\t\"example.com/p/src\"\n)\n\n" +
				"func Both() int { return src.Moved() + d.Use() }\n"
			root, _ := moved(t, files, movedFrom, movedTo)
			builds(t, root)
			assert.Contains(t, content(t, root, userFile), "return d.Moved() + d.Use()", "the file of user")
		})

		t.Run("imports the destination beside a blank import of it", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files[userFile] = "package user\n\nimport (\n\t_ \"example.com/p/dst\"\n\t\"example.com/p/src\"\n)\n\n" +
				"func One() int { return src.Moved() }\n"
			root, _ := moved(t, files, movedFrom, movedTo)
			builds(t, root)
			assert.Contains(t, content(t, root, userFile), "return dst.Moved()", "the file of user")
		})

		t.Run("deletes a named import of the source that the file no longer uses", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files[userFile] = "package user\n\nimport s \"example.com/p/src\"\n\nfunc One() int { return s.Moved() }\n"
			root, _ := moved(t, files, movedFrom, movedTo)
			builds(t, root)
			assert.Equal(
				t,
				content(t, root, userFile),
				"package user\n\nimport \"example.com/p/dst\"\n\nfunc One() int { return dst.Moved() }\n",
				"the file of user",
			)
		})

		t.Run("swaps the import of a package of the same name", func(t *testing.T) {
			t.Parallel()
			files := sameName("package user\n\nimport \"example.com/p/src\"\n\n// One returns [src.Moved].\n" +
				"func One() int { return src.Moved() }\n")
			files["user/doc.go"] = "// Package user calls [src.Moved].\npackage user\n"
			root, _ := moved(t, files, movedFrom, sameTo)
			builds(t, root)
			assert.Equal(t, content(t, root, userFile), "package user\n\nimport \"example.com/p/other/src\"\n\n"+
				"// One returns [src.Moved].\nfunc One() int { return src.Moved() }\n", "the file of user")
			assert.Equal(
				t,
				content(t, root, "user/doc.go"),
				"// Package user calls [example.com/p/other/src.Moved].\npackage user\n",
				"the link of a file without imports",
			)
		})

		t.Run("aliases a package of the same name beside the source", func(t *testing.T) {
			t.Parallel()
			files := sameName(
				"package user\n\nimport \"example.com/p/src\"\n\n// Two adds [src.Moved] to [src.Stay].\n" +
					"func Two() int { return src.Moved() + src.Stay() }\n",
			)
			root, _ := moved(t, files, movedFrom, sameTo)
			builds(t, root)
			assert.Equal(t, content(t, root, userFile), "package user\n\nimport (\n"+
				"\tothersrc \"example.com/p/other/src\"\n\t\"example.com/p/src\"\n)\n\n"+
				"// Two adds [othersrc.Moved] to [src.Stay].\n"+
				"func Two() int { return othersrc.Moved() + src.Stay() }\n", "the file of user")
		})

		t.Run("numbers an alias that the file already uses", func(t *testing.T) {
			t.Parallel()
			files := sameName("package user\n\nimport \"example.com/p/src\"\n\nvar othersrc = 1\n\n" +
				"var point struct{ N int }\n\nfunc Two() int { return src.Moved() + src.Stay() + othersrc + point.N }\n")
			root, _ := moved(t, files, movedFrom, sameTo)
			builds(t, root)
			assert.Contains(t, content(t, root, userFile), "return othersrc2.Moved() + src.Stay() + othersrc + point.N",
				"the file of user")
		})

		t.Run("aliases by the package name alone under a directory that names no package", func(t *testing.T) {
			t.Parallel()
			files := sameName("package user\n\nimport \"example.com/p/src\"\n\n" +
				"func Two() int { return src.Moved() + src.Stay() }\n")
			root, _ := moved(t, files, movedFrom, "odd-name/src/moved.go")
			builds(t, root)
			assert.Contains(t, content(t, root, userFile), "return src2.Moved() + src.Stay()", "the file of user")
		})

		t.Run("keeps the test suffix of an external test", func(t *testing.T) {
			t.Parallel()
			external := "package src_test\n\nimport (\n\t\"testing\"\n\n" +
				"\t\"example.com/p/dst\"\n\t\"example.com/p/src\"\n)\n\n" +
				"// TestStay adds [src.Stay] to [dst.Base].\nfunc TestStay(t *testing.T) { _ = src.Stay() + dst.Base() }\n"
			files := map[string]string{stayFile: stay, dstFile: base, "src/stay_test.go": external}
			root, _ := moved(t, files, "src/stay_test.go", "dst/stay_test.go")
			builds(t, root)
			assert.Equal(t, content(t, root, "dst/stay_test.go"),
				strings.Replace(external, "package src_test", "package dst_test", 1), "the moved test")
		})

		t.Run("refuses a move of an external test that uses another file of its package", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{
				stayFile:             stay,
				dstFile:              base,
				"src/shared_test.go": "package src_test\n\nfunc Shared() int { return 1 }\n",
				"src/stay_test.go":   "package src_test\n\nimport \"testing\"\n\nfunc TestStay(t *testing.T) { _ = Shared() }\n",
			}
			_, err := planned(t, workspace(t, files), "src/stay_test.go", "dst/stay_test.go")
			assert.ErrorIs(t, err, engine.ErrRefuse, "Plan of a move of an external test that uses Shared")
		})

		t.Run("moves a test file of the package", func(t *testing.T) {
			t.Parallel()
			internal := "package src\n\nimport \"testing\"\n\nfunc TestStay(t *testing.T) { _ = Stay() }\n"
			files := map[string]string{stayFile: stay, dstFile: base, "src/stay_test.go": internal}
			root, _ := moved(t, files, "src/stay_test.go", "dst/stay_test.go")
			builds(t, root)
			assert.Equal(t, content(t, root, "dst/stay_test.go"), "package dst\n\nimport (\n\t\"testing\"\n\n"+
				"\t\"example.com/p/src\"\n)\n\nfunc TestStay(t *testing.T) { _ = src.Stay() }\n", "the moved test")
		})

		t.Run("names the package of the destination by its package clause", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files[dstFile] = "package target\n"
			root, _ := moved(t, files, movedFrom, movedTo)
			builds(t, root)
			assert.HasPrefix(t, content(t, root, movedTo), "package target\n", "the moved file")
		})

		t.Run("names the package of a directory of external tests alone", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{
				movedFrom:         called,
				stayFile:          stay,
				userFile:          "package user\n\nimport \"example.com/p/src\"\n\nfunc One() int { return src.Moved() }\n",
				"dst/dst_test.go": "package base_test\n\nimport \"testing\"\n\nfunc TestBase(t *testing.T) {}\n",
			}
			root, _ := moved(t, files, movedFrom, movedTo)
			builds(t, root)
			assert.HasPrefix(t, content(t, root, movedTo), "package base\n", "the moved file")
			assert.Contains(t, content(t, root, userFile), "base \"example.com/p/dst\"", "the file of user")
		})

		t.Run("names the package of a module root without Go files after its directory", func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "top")
			user := "package user\n\nimport \"example.com/p/src\"\n\nfunc One() int { return src.Moved() }\n"
			for name, body := range map[string]string{"go.mod": module, movedFrom: called, userFile: user} {
				full := filepath.Join(root, filepath.FromSlash(name))
				assert.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755), "MkdirAll "+name)
				assert.NoError(t, os.WriteFile(full, []byte(body), 0o644), "WriteFile "+name)
			}
			applied(t, root, movedFrom, "moved.go")
			builds(t, root)
			assert.Equal(
				t,
				content(t, root, userFile),
				"package user\n\nimport top \"example.com/p\"\n\nfunc One() int { return top.Moved() }\n",
				"the file of user",
			)
		})

		t.Run("names a new package after the innermost module of its directory", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["inner/go.mod"] = "module example.com/inner\n\ngo 1.24\n"
			files["inner/in.go"] = "package inner\n"
			root, _ := moved(t, files, movedFrom, "inner/fresh/moved.go")
			assert.Contains(
				t,
				content(t, root, stayFile),
				"\"example.com/inner/fresh\"",
				"the other file of the source",
			)
		})

		t.Run("keeps the line endings of a CRLF file", func(t *testing.T) {
			t.Parallel()
			crlf := strings.ReplaceAll(usingStay, "\n", "\r\n")
			root, _ := moved(t, map[string]string{movedFrom: crlf, stayFile: stay, dstFile: base}, movedFrom, movedTo)
			builds(t, root)
			assert.Equal(t, content(t, root, movedTo), strings.ReplaceAll(stayMoved, "\n", "\r\n"), "the moved file")
		})

		t.Run("edits only the lines that change in a CRLF file", func(t *testing.T) {
			t.Parallel()
			crlf := strings.ReplaceAll(usingStay, "\n", "\r\n")
			got, err := planned(t, workspace(t, map[string]string{movedFrom: crlf, stayFile: stay, dstFile: base}),
				movedFrom, movedTo)
			assert.NoError(t, err, "Plan of the move of a CRLF file")
			for _, one := range got.Items[0].Edits {
				assert.NotContains(t, crlf[one.Span.Start.Offset:one.Span.End.Offset], "\r\n\r\n",
					"the blank line after the package clause")
			}
		})

		t.Run("edits only the lines that change", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files[userFile] = "package user\n\nimport \"example.com/p/src\"\n\nfunc A() int { return src.Moved() }\n\n" +
				"func B() int { return src.Twice() }\n\nfunc C() int { return src.Moved() }\n"
			got, err := planned(t, workspace(t, files), movedFrom, movedTo)
			assert.NoError(t, err, "Plan of the move")
			var edits []edit.TextEdit
			for _, c := range got.Items {
				if c.Path == userFile {
					edits = c.Edits
				}
			}
			assert.Length(t, edits, 3, "the edits of the imports, A and C")
			for _, one := range edits {
				before := files[userFile][:one.Span.Start.Offset]
				assert.Equal(t, one.Span.Start.Line, strings.Count(before, "\n"), "the line of an edit")
				assert.NotContains(t, files[userFile][one.Span.Start.Offset:one.Span.End.Offset], "func B()",
					"the lines that an edit replaces")
			}
		})

		t.Run("rewrites a file that differs in more lines than the script covers", func(t *testing.T) {
			t.Parallel()
			var body strings.Builder
			body.WriteString(usingStay)
			for i := range 1100 {
				body.WriteString("var  V" + strconv.Itoa(i) + "  =  1\n")
			}
			root, _ := moved(t, map[string]string{movedFrom: body.String(), stayFile: stay, dstFile: base},
				movedFrom, movedTo)
			builds(t, root)
			assert.NotContains(t, content(t, root, movedTo), "  =  ", "the moved file as gofmt prints it")
		})

		t.Run("leaves a file without a use of the moved declarations unchanged", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["other/other.go"] = "package other\n\nvar  X  =  1\n"
			got, err := planned(t, workspace(t, files), movedFrom, movedTo)
			assert.NoError(t, err, "Plan of the move")
			assert.NotContains(t, changed(got), source.Path("other/other.go"), "the changes of the plan")
		})

		t.Run("rewrites a file that the build constraints exclude by name", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files[tagged] = excluded(
				"user",
				"import \"example.com/p/src\"\n\nfunc Tagged() int { return src.Moved() }\n",
			)
			files["user/user_test.go"] = "package user_test\n"
			root, got := moved(t, files, movedFrom, movedTo)
			builds(t, root, extra)
			assert.True(t, carries(got.Caveats, trust.CaveatInactiveBuild), "the caveat of the file rewritten by name")
			assert.Equal(t, content(t, root, tagged),
				excluded("user", "import \"example.com/p/dst\"\n\nfunc Tagged() int { return dst.Moved() }\n"),
				"the file rewritten by name")
		})

		t.Run("writes by name the name under which a file imports the destination", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files[tagged] = excluded("user", "import (\n\td \"example.com/p/dst\"\n\t\"example.com/p/src\"\n)\n\n"+
				"func Tagged() int { return src.Moved() + d.Use() }\n")
			root, _ := moved(t, files, movedFrom, movedTo)
			builds(t, root, extra)
			assert.Contains(t, content(t, root, tagged), "return d.Moved() + d.Use()", "the file rewritten by name")
		})

		t.Run("drops the qualifier by name in an excluded file of the destination", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["dst/tagged.go"] = excluded("dst",
				"import \"example.com/p/src\"\n\nfunc Tagged() int { return src.Moved() }\n")
			root, _ := moved(t, files, movedFrom, movedTo)
			builds(t, root, extra)
			assert.Equal(
				t,
				content(t, root, "dst/tagged.go"),
				excluded("dst", "func Tagged() int { return Moved() }\n"),
				"the file rewritten by name",
			)
		})

		t.Run("swaps the import by name in an excluded file", func(t *testing.T) {
			t.Parallel()
			files := sameName("package user\n")
			files[tagged] = excluded("user", "import \"example.com/p/src\"\n\nfunc One() int { return src.Moved() }\n")
			root, _ := moved(t, files, movedFrom, sameTo)
			builds(t, root, extra)
			assert.Equal(t, content(t, root, tagged),
				excluded("user", "import \"example.com/p/other/src\"\n\nfunc One() int { return src.Moved() }\n"),
				"the file rewritten by name")
		})

		t.Run("aliases by name in an excluded file", func(t *testing.T) {
			t.Parallel()
			files := sameName("package user\n")
			files[tagged] = excluded("user", "import \"example.com/p/src\"\n\n"+
				"func Two() int { return src.Moved() + src.Stay() }\n")
			root, _ := moved(t, files, movedFrom, sameTo)
			builds(t, root, extra)
			assert.Contains(
				t,
				content(t, root, tagged),
				"return othersrc.Moved() + src.Stay()",
				"the file rewritten by name",
			)
		})

		t.Run(
			"returns a partial plan for an excluded file of the source that names a moved declaration",
			func(t *testing.T) {
				t.Parallel()
				files := callers()
				files["src/tagged.go"] = excluded("src", "func Tagged() int { return Moved() }\n")
				got, err := planned(t, workspace(t, files), movedFrom, movedTo)
				assert.NoError(t, err, "Plan with an excluded file of the source")
				assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the plan")
				assert.True(t, carries(got.Caveats, trust.CaveatUnrewritten), "the caveat of the file not rewritten")
			},
		)

		t.Run("returns a whole plan for an excluded file of the source that declares a moved name", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["src/tagged.go"] = excluded("src", "func Moved() int { return 3 }\n")
			got, err := planned(t, workspace(t, files), movedFrom, movedTo)
			assert.NoError(t, err, "Plan with an excluded file of the source")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the plan")
		})

		t.Run("returns a whole plan for an excluded file with a method of a moved name", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{
				movedFrom: "package src\n\n// Kind is a kind.\ntype Kind int\n\n// String returns kind.\n" +
					"func (k Kind) String() string { return \"kind\" }\n",
				stayFile: stay,
				"src/tagged.go": excluded(
					"src",
					"type Named struct{}\n\nfunc (Named) String() string { return \"named\" }\n\n"+
						"func Show(n Named) string { return n.String() }\n",
				),
			}
			got, err := planned(t, workspace(t, files), movedFrom, movedTo)
			assert.NoError(t, err, "Plan with an excluded file of the source")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the plan")
		})

		t.Run("leaves an excluded file without a use of a moved declaration unchanged", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["user/alone.go"] = excluded("user", "func Alone() int { return 1 }\n")
			files[tagged] = excluded(
				"user",
				"import \"example.com/p/src\"\n\nfunc Tagged() int { return src.Twice() }\n",
			)
			got, err := planned(t, workspace(t, files), movedFrom, movedTo)
			assert.NoError(t, err, "Plan with excluded files")
			assert.NotContains(t, changed(got), source.Path("user/alone.go"), "the changes of the plan")
			assert.NotContains(t, changed(got), source.Path(tagged), "the changes of the plan")
		})

		t.Run("skips an excluded file that does not parse", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["src/tagged.go"] = excluded("src", "func Tagged() int { return Moved( }\n")
			got, err := planned(t, workspace(t, files), movedFrom, movedTo)
			assert.NoError(t, err, "Plan with an excluded file that does not parse")
			assert.NotContains(t, changed(got), source.Path("src/tagged.go"), "the changes of the plan")
		})

		t.Run("lowers a plan in a module with an error on a line that names the source", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["user/broken.go"] = "package user\n\nimport \"example.com/p/src\"\n\nvar _ = src.Missing\n"
			got, err := planned(t, workspace(t, files), movedFrom, movedTo)
			assert.NoError(t, err, "Plan in a module with an error")
			assert.Equal(t, got.Lowered, trust.Indexed, "the tier of the plan")
		})

		t.Run("lowers a plan in a module with an error on a line that names a moved declaration", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["other/broken.go"] = "package other\n\nfunc Moved() int { return \"text\" }\n"
			got, err := planned(t, workspace(t, files), movedFrom, movedTo)
			assert.NoError(t, err, "Plan in a module with an error")
			assert.Equal(t, got.Lowered, trust.Indexed, "the tier of the plan")
		})

		t.Run("keeps the tier of a plan in a module with an error elsewhere", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["other/broken.go"] = "package other\n\nvar _ int = \"text\"\n"
			got, err := planned(t, workspace(t, files), movedFrom, movedTo)
			assert.NoError(t, err, "Plan in a module with an error")
			assert.Equal(t, got.Lowered, trust.None, "the tier of the plan")
		})

		t.Run("moves the file alone within its directory", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{movedFrom: usingStay, stayFile: stay}
			got, err := planned(t, workspace(t, files), movedFrom, "src/renamed.go")
			assert.NoError(t, err, "Plan of a move within the directory")
			assert.Equal(t, got.Items, []edit.Change{{Kind: edit.ChangeMove, Path: movedFrom, To: "src/renamed.go"}},
				"the plan of a move within the directory")
		})

		t.Run("refuses a move that uses an unexported name across the packages", func(t *testing.T) {
			t.Parallel()
			files := map[string]string{
				movedFrom: "package src\n\nfunc Moved() int { return helper() }\n",
				stayFile:  "package src\n\nfunc helper() int { return 1 }\n",
				dstFile:   base,
			}
			_, err := planned(t, workspace(t, files), movedFrom, movedTo)
			assert.ErrorIs(t, err, engine.ErrRefuse, "Plan of a move that uses helper")
			assert.Contains(t, err.Error(), "helper", "the refusal names the unexported declaration")
		})

		t.Run("refuses a destination that is not a Go file", func(t *testing.T) {
			t.Parallel()
			_, err := planned(t, workspace(t, callers()), movedFrom, "dst/moved.txt")
			assert.ErrorIs(t, err, engine.ErrRefuse, "Plan of a move to a text file")
		})

		t.Run("refuses a move that makes a file a test file", func(t *testing.T) {
			t.Parallel()
			_, err := planned(t, workspace(t, callers()), movedFrom, "dst/moved_test.go")
			assert.ErrorIs(t, err, engine.ErrRefuse, "Plan of a move to a test file")
		})

		t.Run("refuses a destination in a directory that names no package", func(t *testing.T) {
			t.Parallel()
			_, err := planned(t, workspace(t, callers()), movedFrom, "not-a-name/moved.go")
			assert.ErrorIs(t, err, engine.ErrRefuse, "Plan of a move to not-a-name")
		})

		t.Run("refuses a destination outside every module", func(t *testing.T) {
			t.Parallel()
			root := written(t, map[string]string{
				"mod/go.mod": module, "mod/src/moved.go": called, "mod/src/stay.go": stay,
			})
			_, err := planned(t, root, "mod/src/moved.go", "elsewhere/moved.go")
			assert.ErrorIs(t, err, engine.ErrRefuse, "Plan of a move out of the module")
		})

		t.Run("declines a file that the build constraints exclude", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["src/tagged.go"] = excluded("src", "func Tagged() int { return 1 }\n")
			_, err := planned(t, workspace(t, files), "src/tagged.go", "dst/tagged.go")
			assert.ErrorIs(t, err, engine.ErrDecline, "Plan of the move of an excluded file")
		})

		t.Run("skips a file of another language", func(t *testing.T) {
			t.Parallel()
			files := callers()
			files["notes.py"] = "x = 1\n"
			got, err := planned(t, workspace(t, files), "notes.py", "src/notes.py")
			assert.NoError(t, err, "Plan of the move of a Python file")
			assert.True(t, got.Skipped, "the plan of the move of a Python file is skipped")
		})

		t.Run("declines an operation other than a move", func(t *testing.T) {
			t.Parallel()
			target, args := moveTarget(movedFrom, movedTo)
			_, err := serving(t, callers()).Plan(t.Context(), engine.Request{}, edit.RenameSymbol, target, args)
			assert.ErrorIs(t, err, engine.ErrDecline, "Plan of a rename")
		})
	})
}
