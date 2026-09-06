// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"golang.org/x/tools/go/packages"
)

// view is one type-checked reading of the workspace.
//
// The file set is part of it: a position is an index into the set that
// produced it, so a view and the positions read out of it cannot be
// separated.
type view struct {
	fset *token.FileSet
	pkgs []*packages.Package
	// stamp is what the workspace looked like when this was built.
	stamp string
}

// loading is what a load needs, and everything it needs is the mode.
//
// Every field is asked for because every one is used: the syntax and the
// type information answer what a name denotes, the imports and
// dependencies answer about a name declared in another package, and the
// module tells a file outside the workspace from one inside it.
const loading = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedDeps |
	packages.NeedTypes |
	packages.NeedSyntax |
	packages.NeedTypesInfo |
	packages.NeedModule

// current returns a type-checked view of the workspace, loading one if
// what is cached no longer describes it.
//
// # Why it is cached at all
//
// Type-checking a module is seconds, and a caller asking who calls a
// function then who calls its caller would pay it twice. The load is
// what makes this engine expensive once and cheap afterwards, which is
// the cost it declares.
//
// # What invalidates it
//
// A stamp over every Go file in the workspace: its path, its size and
// when it was last written. That catches a file edited, added or taken
// away, which the loaded file list alone would not — a package that
// gained a file since the last load is one the cache would answer about
// as though the file were not there.
//
// Walking the workspace to build the stamp costs a stat per file, which
// is what one load costs per hundred files.
func (e *Engine) current(ctx context.Context) (*view, error) {
	stamp, err := e.stamp()
	if err != nil {
		return nil, err
	}

	e.loading.Lock()
	defer e.loading.Unlock()

	if e.view != nil && e.view.stamp == stamp {
		return e.view, nil
	}
	held, err := e.load(ctx, nil)
	if err != nil {
		return nil, err
	}
	held.stamp = stamp
	e.view = held
	return held, nil
}

// load type-checks the workspace, optionally over content the workspace
// does not hold.
//
// An overlay is how a gate judges a change before anything is written:
// the loader reads the named paths from memory and everything else from
// disk, so what is checked is the workspace as the change would leave
// it. A load with one is never cached, because it describes a workspace
// that does not exist.
func (e *Engine) load(ctx context.Context, overlay map[string][]byte) (*view, error) {
	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Context: ctx,
		Mode:    loading,
		Dir:     e.root,
		Fset:    fset,
		Overlay: overlay,
		// A declaration used only from a test is used, and a rename that
		// missed it would leave the workspace not building. Loading them
		// doubles the packages and is what makes the answer true.
		Tests: true,
	}, "./...")
	if err != nil {
		return nil, fmt.Errorf("checker: load %s: %w", e.root, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("checker: %s holds no Go packages", e.root)
	}
	return &view{fset: fset, pkgs: pkgs}, nil
}

// stamp is what the workspace's Go files look like now.
//
// Path, size and modification time, which is what a build tool watches
// and enough to notice every edit that is not a rewrite to the same
// length within the same nanosecond.
func (e *Engine) stamp() (string, error) {
	var held []string
	err := fs.WalkDir(os.DirFS(e.root), ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			if skipped(p) {
				return fs.SkipDir
			}
			return nil
		case filepath.Ext(p) != ".go":
			return nil
		}
		// A file that went between the walk and the stat is a workspace
		// that changed, which is exactly what this reports: it is left
		// out, so the stamp differs and the next answer is read again.
		if info, there := d.Info(); there == nil {
			held = append(held, fmt.Sprintf(
				"%s %d %d", p, info.Size(), info.ModTime().UnixNano()))
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("checker: read %s: %w", e.root, err)
	}

	sort.Strings(held)
	sum := sha256.Sum256([]byte(strings.Join(held, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// skipped reports whether a directory holds nothing this engine reads.
//
// A vendor tree and a module cache are Go and are not the workspace's
// to answer about, and a hidden directory is a tool's own storage.
func skipped(p string) bool {
	name := filepath.Base(p)
	return name != "." && (strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata")
}

// forget drops the cached view.
//
// A composition root calls it through Close. Nothing else needs to: the
// stamp is what decides whether the cache still describes the workspace.
func (e *Engine) forget() {
	e.loading.Lock()
	defer e.loading.Unlock()
	e.view = nil
}

// held is the packages a view holds, without the duplicates loading
// tests produces.
//
// Loading with tests gives a package three times: the package, the
// package compiled with its test files, and the external test package.
// The first two declare the same names in the same files, and an answer
// that walked both would name every declaration twice.
func (v *view) held() []*packages.Package {
	seen := map[string]bool{}
	var out []*packages.Package

	for _, pkg := range v.pkgs {
		// The variant compiled with its tests holds a superset of the
		// plain package's files, so it is the one to keep.
		if pkg.ID == pkg.PkgPath || strings.HasSuffix(pkg.ID, ".test]") {
			continue
		}
		seen[pkg.PkgPath] = true
		out = append(out, pkg)
	}
	for _, pkg := range v.pkgs {
		if !seen[pkg.PkgPath] {
			seen[pkg.PkgPath] = true
			out = append(out, pkg)
		}
	}
	slices.SortFunc(out, func(a, b *packages.Package) int {
		return strings.Compare(a.PkgPath, b.PkgPath)
	})
	return out
}

// pathOf is a file where it is, as a path relative to the workspace.
//
// A file outside the root keeps its absolute path: the loader reads a
// standard library and a module cache, and reported relative those would
// climb out of the workspace and read as files in it.
func (e *Engine) pathOf(full string) source.Path {
	relative, err := filepath.Rel(e.root, full)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return source.Path(filepath.ToSlash(full))
	}
	return source.Path(filepath.ToSlash(relative))
}

// inside reports whether a path is one the workspace holds.
func inside(p source.Path) bool {
	return !filepath.IsAbs(filepath.FromSlash(string(p)))
}

// bound is what an answer resting on the type checker is worth: the
// tier it reaches and the limits on it.
//
// Coverage is not among them. The loader read the whole module, so every
// answer covers the scope it was asked about; what the faults change is
// how strong the binding is, not how much was looked at.
//
// A workspace that does not compile is bound where the checker could
// bind it and guessed at everywhere else: an expression whose type is
// invalid resolves to nothing, and every name reached through it is
// matched rather than bound. The answer says so rather than claiming the
// tier this engine reaches over a whole program.
func (*Engine) bound(v *view) (trust.Fidelity, []trust.Caveat) {
	caveats := []trust.Caveat{{
		Code: trust.CaveatDynamic,
		Note: "reflection, string-keyed dispatch and struct tags are invisible here, " +
			"as they are to every engine",
	}}
	if !v.broken() {
		return trust.None, caveats
	}
	return trust.Indexed, append(caveats, trust.Caveat{
		Code: trust.CaveatBuildBroken,
		Note: "the workspace does not compile, so names are bound where the type " +
			"checker could bind them and matched where it could not",
	})
}

// broken reports whether the type checker objected to anything.
func (v *view) broken() bool {
	for _, pkg := range v.pkgs {
		if len(pkg.Errors) > 0 {
			return true
		}
	}
	return false
}
