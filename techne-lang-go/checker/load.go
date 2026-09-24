// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"golang.org/x/tools/go/packages"
)

// loading is the mode of a load. The loader type-checks the packages of the workspace from
// source. A dependency outside the workspace has the types of the export data that the go
// command writes, and no syntax.
const loading = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedTypes |
	packages.NeedSyntax |
	packages.NeedTypesInfo |
	packages.NeedModule |
	packages.NeedForTest

// concurrent is the number of loads that run at once for a workspace with a load per module.
const concurrent = 4

// manifests are the files, other than Go files, whose change changes what a load returns.
var manifests = []string{"go.mod", "go.sum", "go.work", "go.work.sum"}

// view is one type-checked reading of the workspace: the programs that the engine loaded, in
// one file set. Every position of the packages of a view is an index into its file set.
type view struct {
	fset     *token.FileSet
	programs []program
	// unloaded are the errors of the modules that failed to load, by module directory.
	unloaded map[source.Path]string
	// stamp is the stamp of the walk before the load, and empty for a load with an overlay.
	stamp string
	// goroot is the root of the Go toolchain, as go env GOROOT reports it.
	goroot string

	// keep builds kept on the first call of [view.held].
	keep sync.Once
	kept []*packages.Package
	// index builds files on the first call of [view.file].
	index sync.Once
	files map[*token.File]*ast.File
	// list builds compiled on the first call of [view.compiles].
	list     sync.Once
	compiled map[string]bool
}

// program is the packages of one load, with the directory of the go.mod file of the load as
// module, or [engine.Root] for the load of the modules of a go.work file.
type program struct {
	module source.Path
	pkgs   []*packages.Package
}

// all returns the packages of every program of v.
func (v *view) all() []*packages.Package {
	var out []*packages.Package
	for _, one := range v.programs {
		out = append(out, one.pkgs...)
	}
	return out
}

// program returns the program that type-checks the file at p: the program of the innermost
// module that contains p, or no program for a path outside every module.
func (v *view) program(p source.Path) program {
	var out program
	deepest := -1
	for _, one := range v.programs {
		if depth := depthOf(one.module); lang.Within(p, one.module) && depth > deepest {
			out, deepest = one, depth
		}
	}
	return out
}

// depthOf returns the number of path segments of the module directory m, and 0 for the root.
func depthOf(m source.Path) int {
	if m == engine.Root {
		return 0
	}
	return strings.Count(string(m), "/") + 1
}

// held returns the packages whose files an answer reads, each file once and sorted by ID. The
// test variant of a package, which the go command names p [p.test], replaces the package,
// because it compiles the files of the package and the test files inside it. The external test
// package p_test [p.test] is held. The test main p.test, which the go command generates, is
// not.
func (v *view) held() []*packages.Package {
	v.keep.Do(func() {
		all := v.all()
		tested := map[string]bool{}
		for _, pkg := range all {
			if pkg.ForTest != "" && pkg.PkgPath == pkg.ForTest {
				tested[pkg.PkgPath] = true
			}
		}
		for _, pkg := range all {
			switch {
			case pkg.Types == nil || pkg.TypesInfo == nil:
			case pkg.ForTest == "" && tested[pkg.PkgPath]:
			case pkg.ForTest == "" && pkg.Name == "main" && strings.HasSuffix(pkg.ID, ".test"):
			default:
				v.kept = append(v.kept, pkg)
			}
		}
		slices.SortFunc(v.kept, func(a, b *packages.Package) int { return strings.Compare(a.ID, b.ID) })
	})
	return v.kept
}

// file returns the syntax of the file that contains pos, or nil for a file of which the view
// has no syntax, such as a file of the standard library.
func (v *view) file(pos token.Pos) *ast.File {
	v.index.Do(func() {
		v.files = map[*token.File]*ast.File{}
		for _, pkg := range v.all() {
			for _, one := range pkg.Syntax {
				v.files[v.fset.File(one.FileStart)] = one
			}
		}
	})
	return v.files[v.fset.File(pos)]
}

// expanded returns the name of the file of a position, with the $GOROOT prefix that the export
// data of the standard library writes replaced by the root of the Go toolchain.
func (v *view) expanded(name string) string {
	if rest, cut := strings.CutPrefix(name, "$GOROOT"); cut && v.goroot != "" {
		return filepath.Join(v.goroot, filepath.FromSlash(rest))
	}
	return name
}

// compiles reports whether a package of v compiles the file at full, an absolute path.
func (v *view) compiles(full string) bool {
	v.list.Do(func() {
		v.compiled = map[string]bool{}
		for _, pkg := range v.all() {
			for _, one := range slices.Concat(pkg.GoFiles, pkg.CompiledGoFiles) {
				v.compiled[one] = true
			}
		}
	})
	return v.compiled[full]
}

// walked is what one walk of the workspace found.
type walked struct {
	// stamp is a digest of the path, the size and the modification time of every Go file, every
	// file that manifests names and the list of vendored modules of every module.
	stamp string
	// modules are the directories that contain a go.mod file.
	modules []source.Path
	// files are the Go files.
	files []source.Path
}

// claims reports whether scope contains a Go file of w.
func (w walked) claims(scope source.Path) bool {
	return slices.ContainsFunc(w.files, func(p source.Path) bool { return lang.Within(p, scope) })
}

// walk returns the Go files and the modules of the workspace, and their stamp.
//
// It skips what the go command leaves out of a pattern that ends in /...: a file or a
// directory whose name starts with a dot or an underscore, a testdata directory and a vendor
// directory. The stamp includes the list of vendored modules of each module, which a load
// reads. It changes when a file changes, appears or goes.
func (e *Engine) walk() (walked, error) {
	var out walked
	var stamps []string
	fsys := os.DirFS(e.root)
	stamp := func(p string) (fs.FileInfo, bool) {
		info, err := fs.Stat(fsys, p)
		if err == nil {
			stamps = append(stamps, fmt.Sprintf("%s %d %d", p, info.Size(), info.ModTime().UnixNano()))
		}
		return info, err == nil
	}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case p != "." && skipped(d.Name()) && d.IsDir():
			return fs.SkipDir
		case p == "." || skipped(d.Name()) || d.IsDir():
			return nil
		case path.Ext(p) == ".go":
			// A file removed between the read of its directory and its stat is left out.
			if _, there := stamp(p); there {
				out.files = append(out.files, source.Path(p))
			}
		case d.Name() == "go.mod":
			if _, there := stamp(p); there {
				out.modules = append(out.modules, source.Path(path.Dir(p)))
				stamp(path.Join(path.Dir(p), "vendor", "modules.txt"))
			}
		case slices.Contains(manifests, d.Name()):
			stamp(p)
		}
		return nil
	})
	if err != nil {
		return walked{}, fmt.Errorf("checker: walk %s: %w", e.root, err)
	}
	slices.Sort(stamps)
	sum := sha256.Sum256([]byte(strings.Join(stamps, "\n")))
	out.stamp = hex.EncodeToString(sum[:])
	return out, nil
}

// skipped reports whether the go command leaves a file or a directory named name out of a
// pattern that ends in /....
func skipped(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor"
}

// current returns the view of the workspace that w describes: the cached view when its stamp
// is the stamp of w, and a view loaded now otherwise. One lock guards the cached view, so two
// questions that arrive together load the workspace once.
func (e *Engine) current(ctx context.Context, w walked) (*view, error) {
	e.loading.Lock()
	defer e.loading.Unlock()

	if e.view != nil && e.view.stamp == w.stamp {
		return e.view, nil
	}
	v, err := e.load(ctx, w.modules, nil)
	if err != nil {
		return nil, err
	}
	v.stamp = w.stamp
	e.view = v
	return v, nil
}

// load type-checks the modules of the workspace, with the content of overlay in place of the
// files that it names. A view with an overlay is not cached, because it describes content that
// is not on disk. A module that fails to load is in the unloaded modules of the view, and load
// returns an error when no module loads.
func (e *Engine) load(ctx context.Context, modules []source.Path, overlay map[string][]byte) (*view, error) {
	env, err := e.goCommand(ctx, "env", "GOWORK", "GOROOT")
	if err != nil {
		return nil, err
	}
	values := append(strings.Split(env, "\n"), "", "")
	plans, err := e.plans(ctx, strings.TrimSpace(values[0]), modules)
	if err != nil {
		return nil, err
	}

	v := &view{fset: token.NewFileSet(), unloaded: map[source.Path]string{}, goroot: strings.TrimSpace(values[1])}
	loaded := make([][]*packages.Package, len(plans))
	failed := make([]error, len(plans))
	slots := make(chan struct{}, concurrent)
	var wg sync.WaitGroup
	for i, one := range plans {
		var env []string
		if one.alone {
			env = append(os.Environ(), "GOWORK=off")
		}
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			loaded[i], failed[i] = packages.Load(&packages.Config{
				Context: ctx,
				Mode:    loading,
				Dir:     one.dir,
				Env:     env,
				Fset:    v.fset,
				Overlay: overlay,
				Tests:   true,
			}, one.patterns...)
		})
	}
	wg.Wait()

	for i, one := range plans {
		if failed[i] != nil {
			v.unloaded[one.module] = firstLine(failed[i].Error())
			continue
		}
		for _, pkg := range loaded[i] {
			absolute(pkg, one.dir)
		}
		v.programs = append(v.programs, program{module: one.module, pkgs: loaded[i]})
	}
	if len(v.programs) == 0 {
		return nil, fmt.Errorf("checker: no module of %s loads: %s", e.root, failures(v.unloaded))
	}
	return v, nil
}

// failures returns the modules of unloaded with their errors, in module order, for a message.
func failures(unloaded map[source.Path]string) string {
	out := make([]string, 0, len(unloaded))
	for _, m := range slices.Sorted(maps.Keys(unloaded)) {
		out = append(out, fmt.Sprintf("%s (%s)", m, unloaded[m]))
	}
	return strings.Join(out, ", ")
}

// plan is one load: the directory that the go command runs in, the patterns, and the module
// whose failure the load reports. alone reports whether the go command loads the module with
// GOWORK=off, without the go.work file that does not list it.
type plan struct {
	dir      string
	patterns []string
	module   source.Path
	alone    bool
}

// plans returns the loads of the modules of the workspace.
//
// Under the go.work file work, which go env GOWORK reports, one load covers the modules of the
// workspace that go list -m lists under the root, because the go command type-checks them as
// one program. A module under the root that the go.work file does not list is a program of its
// own, loaded with GOWORK=off, as the go command builds it. Without a go.work file each module
// is a program of its own. Either way the packages of the module that contains the root, when
// the root is below the directory of its go.mod file, are loaded from the root.
func (e *Engine) plans(ctx context.Context, work string, modules []source.Path) ([]plan, error) {
	var out []plan
	listed := map[string]bool{}
	enclosing := false
	if work != "" && work != "off" {
		dirs, err := e.goCommand(ctx, "list", "-m", "-f", "{{.Dir}}")
		if err != nil {
			return nil, err
		}
		var patterns []string
		for dir := range strings.SplitSeq(strings.TrimSpace(dirs), "\n") {
			listed[dir] = true
			switch p := e.pathOf(dir); {
			case e.encloses(dir):
				patterns, enclosing = append(patterns, "./..."), true
			case lang.Within(p, engine.Root):
				patterns = append(patterns, "./"+string(p)+"/...")
			}
		}
		if len(patterns) > 0 {
			slices.Sort(patterns)
			out = append(out, plan{dir: e.root, patterns: slices.Compact(patterns), module: engine.Root})
		}
	}
	alone := len(listed) > 0
	for _, one := range modules {
		if !listed[e.fullPath(one)] {
			out = append(out, plan{dir: e.fullPath(one), patterns: []string{"./..."}, module: one, alone: alone})
		}
	}
	if !enclosing && !slices.Contains(modules, engine.Root) && enclosed(e.root) {
		out = append(out, plan{dir: e.root, patterns: []string{"./..."}, module: engine.Root, alone: alone})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("checker: %s contains no Go module", e.root)
	}
	return out, nil
}

// encloses reports whether the directory dir, an absolute path, is the root or contains it.
func (e *Engine) encloses(dir string) bool { return below(e.root, dir) }

// enclosed reports whether a directory above dir contains a go.mod file.
func enclosed(dir string) bool {
	for at := filepath.Dir(dir); at != filepath.Dir(at); at = filepath.Dir(at) {
		if _, err := os.Stat(filepath.Join(at, "go.mod")); err == nil {
			return true
		}
	}
	return false
}

// below reports whether the absolute path p is dir or a path under it.
func below(p, dir string) bool {
	relative, err := filepath.Rel(dir, p)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// goCommand runs the go command with args in the root, and returns its output. The error of a
// failed command contains the first line that the command wrote to stderr.
func (e *Engine) goCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = e.root
	out, err := cmd.Output()
	var failed *exec.ExitError
	switch {
	case errors.As(err, &failed) && len(failed.Stderr) > 0:
		return "", fmt.Errorf("checker: go %s: %s", strings.Join(args, " "), firstLine(string(failed.Stderr)))
	case err != nil:
		return "", fmt.Errorf("checker: go %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// absolute makes the file of the position of each error of pkg absolute. The go command writes
// the file of a list error relative to dir, the directory that it ran in.
func absolute(pkg *packages.Package, dir string) {
	for i, one := range pkg.Errors {
		if file, _, _ := placed(one.Pos); file != "" && !filepath.IsAbs(file) {
			pkg.Errors[i].Pos = filepath.Join(dir, file) + one.Pos[len(file):]
		}
	}
}

// forget drops the cached view.
func (e *Engine) forget() {
	e.loading.Lock()
	defer e.loading.Unlock()
	e.view = nil
}

// fullPath returns the absolute path of the workspace path p.
func (e *Engine) fullPath(p source.Path) string {
	return filepath.Join(e.root, filepath.FromSlash(string(p)))
}

// pathOf returns the workspace path of the file at full, an absolute path. A file outside the
// workspace, such as a file of the standard library or the module cache, keeps its absolute
// path in slash form.
func (e *Engine) pathOf(full string) source.Path {
	if !below(full, e.root) {
		return source.Path(filepath.ToSlash(full))
	}
	relative, _ := filepath.Rel(e.root, full)
	return source.Path(filepath.ToSlash(relative))
}

// firstLine returns the first line of text, without the white space around it.
func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(line)
}
