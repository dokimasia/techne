// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
)

// Plan returns the changes of an operation and writes nothing. The write path of techne
// seals, gates and applies the changes, as it does for every planner. Plan sends these
// requests of LSP 3.17:
//
//   - [edit.RenameSymbol]: textDocument/rename, after textDocument/prepareRename where the
//     server offers it.
//   - [edit.MoveFile]: workspace/willRenameFiles, the edits a move implies, followed by the
//     move itself.
//   - [edit.ExtractFunction]: a code action that extracts the lines, followed by a rename of
//     the function it adds.
//
// Plan returns [engine.ErrDecline] for every other operation, and for a server that does not
// answer within [Server.Answering]. For a target in a file of another language it returns a
// skipped result.
func (e *Engine) Plan(
	ctx context.Context,
	req engine.Request,
	op edit.Operation,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	var out engine.Result[edit.Change]
	var err error
	switch op {
	case edit.RenameSymbol:
		out, err = e.renaming(ctx, req, target, args)
	case edit.MoveFile:
		out, err = e.relocating(ctx, target, args)
	case edit.ExtractFunction:
		out, err = e.extracting(ctx, target, args)
	default:
		err = fmt.Errorf("%w: %s: no request plans %s", engine.ErrDecline, e.server.Name, op)
	}
	return out, e.unanswered(ctx, err)
}

// preloads is the number of files that [Engine.preload] opens for one plan.
const preloads = 200

// preload opens the files of the workspace that writes reports as writing what, for a
// [Server.Scoped] server, so that the server finds the uses in them. It skips the file at skip,
// which the plan has open. preload returns the files that write what, and a caveat when more
// files write what than it opens, nil otherwise.
//
// A [Server.Tsserver] server loads a project whole when it opens one of its files. When more
// files write what than preload opens, preload opens in their place the files that
// [Engine.unprojected] returns and that [Engine.reaching] keeps for the file at declared, the
// file of the declaration or the moved file. An empty declared keeps every file that
// Engine.unprojected returns.
func (e *Engine) preload(
	ctx context.Context,
	held *session,
	what string,
	writes func(p source.Path, content []byte) bool,
	skip, declared source.Path,
) (*trust.Caveat, []source.Path, error) {
	if !e.server.Scoped || what == "" {
		return nil, nil, nil
	}
	files, err := e.walk(engine.Request{Scope: engine.Root})
	if err != nil {
		return nil, nil, err
	}
	var writers []source.Path
	for _, p := range files.Read {
		if p == skip {
			continue
		}
		content, err := os.ReadFile(e.fullPath(p))
		if err == nil && writes(p, content) {
			writers = append(writers, p)
		}
	}
	opened, written := writers, "files that write "+what
	if len(writers) > preloads && e.server.Tsserver {
		loose, projects, err := e.unprojected(ctx, held, writers)
		if err != nil {
			return nil, nil, err
		}
		opened = e.reaching(loose, projects, declared)
		written = "files outside the projects of tsserver that write " + what + " and can refer to it"
	}
	for _, p := range opened[:min(len(opened), preloads)] {
		if _, err := e.open(ctx, held, p); err != nil && !refused(err) {
			return nil, nil, err
		}
	}
	if len(opened) <= preloads {
		return nil, writers, nil
	}
	return &trust.Caveat{
		Code: trust.CaveatIndexWarming,
		Note: fmt.Sprintf("%s loads only the files it has open, and techne opened %d of the %d %s, "+
			"so a use in the others may be missing", e.server.Name, preloads, len(opened), written),
	}, writers, nil
}

// tsProjects are the names of the files that declare a project of tsserver.
var tsProjects = []string{"tsconfig.json", "jsconfig.json"}

// tsserverRequest is the command of typescript-language-server that forwards a request to
// tsserver, and projectInfo the request of tsserver that returns the project of a file.
const (
	tsserverRequest = "typescript.tsserverRequest"
	projectInfo     = "projectInfo"
)

// unprojected opens one file of writers under each file of [tsProjects], the one nearest to
// the file, so that tsserver loads the project that it declares. It returns the files of
// writers that no project of a file of tsProjects contains then, as [Engine.configured]
// reports: the files of a project without a configuration file, and a file that its nearest
// configuration file leaves out. It also returns the directories of the configuration files of
// the projects that it loaded.
func (e *Engine) unprojected(
	ctx context.Context,
	held *session,
	writers []source.Path,
) (loose []source.Path, projects []string, err error) {
	nearest := map[string]string{}
	loaded := map[string]bool{}
	for _, p := range writers {
		config := e.projectOf(path.Dir(string(p)), nearest)
		if config == "" || loaded[config] {
			continue
		}
		loaded[config] = true
		projects = append(projects, path.Dir(config))
		if _, err := e.open(ctx, held, p); err != nil && !refused(err) {
			return nil, nil, err
		}
	}
	for _, p := range writers {
		configured, err := e.configured(ctx, held, p)
		if err != nil {
			return nil, nil, err
		}
		if !configured {
			loose = append(loose, p)
		}
	}
	return loose, projects, nil
}

// reaching returns the files of loose that can refer to a declaration of the file at declared.
// The files of loose are in no project of tsserver, and projects are the directories of the
// projects that tsserver loaded. A file of no project sees what it imports, and a module offers
// its declarations only to a file that imports it, so reaching keeps a file of loose whose
// imports lead to declared or to a loaded project, as [reach.leads] reports. It returns loose
// unchanged for an empty declared, for a declared file without a line that [moduleStatement]
// matches, which is no module, and for a module with a line that [globalBlock] matches, because
// every file of a program sees a global declaration.
func (e *Engine) reaching(loose []source.Path, projects []string, declared source.Path) []source.Path {
	if declared == "" {
		return loose
	}
	content, err := os.ReadFile(e.fullPath(declared))
	if err != nil || !moduleStatement.Match(content) || globalBlock.Match(content) {
		return loose
	}
	r := reach{engine: e, projects: projects, declared: declared, known: map[source.Path]bool{}}
	var out []source.Path
	for _, p := range loose {
		if r.leads(p) {
			out = append(out, p)
		}
	}
	return out
}

// moduleStatement matches a line that starts with import or export. Such a statement makes a
// file of JavaScript or TypeScript a module.
var moduleStatement = regexp.MustCompile(`(?m)^[ \t]*(?:import|export)\b`)

// globalBlock matches a line that opens a block of global declarations: declare global in a
// module, or global in the declaration of a module.
var globalBlock = regexp.MustCompile(`(?m)^[ \t]*(?:declare\s+)?global\s*\{`)

// specifiers matches the specifier of a module in its first group. An import, an export from a
// module, a call of require and a call of import each write one. It matches the path of a
// reference directive in its second group.
var specifiers = regexp.MustCompile(
	`(?:\bfrom|\bimport\s*\(?|\brequire\s*\()\s*["']([^"'\n]+)["']|<reference\s+path\s*=\s*["']([^"'\n]+)["']`)

// resolvable are the extensions that tsserver appends to a module specifier, in the order in
// which it tries them: those of TypeScript, then those of JavaScript for a project that allows
// JavaScript.
var resolvable = []string{".ts", ".tsx", ".d.ts", ".mts", ".cts", ".d.mts", ".d.cts", ".js", ".jsx", ".mjs", ".cjs"}

// compiled maps the extension of a JavaScript file to the extensions of the TypeScript files that
// compile to it. tsserver resolves a specifier of the JavaScript file to such a file.
var compiled = map[string][]string{
	".js":  {".ts", ".tsx", ".d.ts"},
	".jsx": {".tsx"},
	".mjs": {".mts", ".d.mts"},
	".cjs": {".cts", ".d.cts"},
}

// The names with which tsserver resolves a module specifier.
const (
	// nodeModules is the directory of the packages that the files of its parent directory and
	// of the directories under it import.
	nodeModules = "node_modules"

	// indexStem is the stem of the file that a specifier of a directory resolves to.
	indexStem = "index"

	// subpathPrefix starts a subpath import, which the imports field of a package.json
	// resolves.
	subpathPrefix = "#"

	// scopePrefix starts the scope of a scoped package, whose name is its first two segments.
	scopePrefix = "@"
)

// reach finds the files of a workspace whose imports lead to a declaration, for one call of
// [Engine.reaching]. It reads each file at most once.
type reach struct {
	engine   *Engine
	projects []string             // the directories of the projects that tsserver loaded
	declared source.Path          // the file of the declaration
	known    map[source.Path]bool // whether the imports of a file lead to the declaration
}

// leads reports whether the imports of the file at p lead to the declaration. They do when p, or
// a file that p imports directly or through other files, meets one of these conditions:
//
//   - It is the file of the declaration.
//   - It is under a directory of a loaded project.
//   - It has an import that [reach.imported] cannot follow.
//
// A walk that ends without such a file has read every file that p leads to, so leads records
// false for each file that it visited.
func (r *reach) leads(p source.Path) bool {
	visited := map[source.Path]bool{}
	for pending := []source.Path{p}; len(pending) > 0; {
		next := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if visited[next] {
			continue
		}
		visited[next] = true
		imported, found := r.step(next)
		if found {
			r.known[p] = true
			return true
		}
		pending = append(pending, imported...)
	}
	for one := range visited {
		r.known[one] = false
	}
	return false
}

// step returns the files that the file at p imports, and reports true when p leads to the
// declaration without them. It returns no import of a file that an earlier walk found to lead
// nowhere.
func (r *reach) step(p source.Path) ([]source.Path, bool) {
	if known, ok := r.known[p]; ok {
		return nil, known
	}
	loaded := slices.ContainsFunc(r.projects, func(dir string) bool { return lang.Within(p, source.Path(dir)) })
	if p == r.declared || loaded {
		return nil, true
	}
	return r.imported(p)
}

// imported returns the files of the workspace that the file at p imports with a relative
// specifier or a reference directive, and reports true for a file that does not read and for an
// import that it cannot follow: a specifier that [reach.resolved] cannot follow, a subpath
// import, an absolute specifier, and a package that [reach.linked] reports.
func (r *reach) imported(p source.Path) ([]source.Path, bool) {
	content, err := os.ReadFile(r.engine.fullPath(p))
	if err != nil {
		return nil, true
	}
	var out []source.Path
	for _, match := range specifiers.FindAllSubmatch(content, -1) {
		specifier := string(match[1])
		if directive := string(match[2]); directive != "" {
			// The path of a reference directive is relative to its file, with or without ./.
			specifier = "./" + directive
		}
		switch {
		case strings.HasPrefix(specifier, "."):
			files, followed := r.resolved(source.Path(path.Join(path.Dir(string(p)), specifier)))
			if !followed {
				return nil, true
			}
			out = append(out, files...)
		case strings.HasPrefix(specifier, subpathPrefix), path.IsAbs(specifier), r.linked(p, specifier):
			return nil, true
		}
	}
	return out, false
}

// resolved returns the files of the workspace to which tsserver can resolve the module at base:
//
//   - base itself
//   - base with an extension of [resolvable]
//   - base with its JavaScript extension replaced by an extension of [compiled]
//   - the index file of base as a directory
//
// It reports false, for a module that it cannot follow, when base is outside the workspace and
// when base is a directory without an index file. The package.json of such a directory can
// point to any file.
func (r *reach) resolved(base source.Path) ([]source.Path, bool) {
	if lang.Outside(base) {
		return nil, false
	}
	stem := strings.TrimSuffix(string(base), path.Ext(string(base)))
	candidates := []string{string(base)}
	for _, ext := range resolvable {
		candidates = append(candidates, string(base)+ext, path.Join(string(base), indexStem+ext))
	}
	for _, ext := range compiled[path.Ext(string(base))] {
		candidates = append(candidates, stem+ext)
	}
	var out []source.Path
	for _, one := range candidates {
		if fileExists(r.engine.fullPath(source.Path(one))) {
			out = append(out, source.Path(one))
		}
	}
	if info, err := os.Stat(r.engine.fullPath(base)); len(out) == 0 && err == nil && info.IsDir() {
		return nil, false
	}
	return out, true
}

// linked reports whether the package of a bare specifier, as node resolves it from the file at
// p, is a symbolic link into the workspace, as a workspace of npm, yarn or pnpm links its
// packages. The package is the first segment of the specifier, or the first two of a scoped
// package. linked reads the [nodeModules] directories from the directory of p up to the root of
// the file system, and stops at the first that contains the package.
func (r *reach) linked(p source.Path, specifier string) bool {
	name, rest, _ := strings.Cut(specifier, "/")
	if strings.HasPrefix(name, scopePrefix) {
		second, _, _ := strings.Cut(rest, "/")
		name = path.Join(name, second)
	}
	for dir := filepath.Dir(r.engine.fullPath(p)); ; dir = filepath.Dir(dir) {
		link := filepath.Join(dir, nodeModules, filepath.FromSlash(name))
		if info, err := os.Lstat(link); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				return false
			}
			target, err := filepath.EvalSymlinks(link)
			if err != nil {
				return false
			}
			relative, err := filepath.Rel(r.engine.root, target)
			return err == nil && !lang.Outside(source.Path(filepath.ToSlash(relative)))
		}
		if dir == filepath.Dir(dir) {
			return false
		}
	}
}

// projectOf returns the workspace path of the file of [tsProjects] in dir or in the nearest
// directory above it, or the empty string for none up to the root. nearest keeps the answer of
// each directory for the call.
func (e *Engine) projectOf(dir string, nearest map[string]string) string {
	if found, known := nearest[dir]; known {
		return found
	}
	found := ""
	for _, name := range tsProjects {
		if candidate := path.Join(dir, name); fileExists(e.fullPath(source.Path(candidate))) {
			found = candidate
			break
		}
	}
	if found == "" && dir != "." && dir != "/" {
		found = e.projectOf(path.Dir(dir), nearest)
	}
	nearest[dir] = found
	return found
}

// fileExists reports whether a file is at full.
func fileExists(full string) bool {
	info, err := os.Stat(full)
	return err == nil && !info.IsDir()
}

// configured reports whether tsserver has loaded a project of a file of [tsProjects] that
// contains the file at p, as projectInfo returns it. tsserver responds to projectInfo for a file
// of no loaded project with the error No Project, and configured reports false for it and for
// a file of a project without a configuration file.
func (e *Engine) configured(ctx context.Context, held *session, p source.Path) (bool, error) {
	args, err := json.Marshal(map[string]any{"file": e.fullPath(p), "needFileNameList": false})
	if err != nil {
		return false, fmt.Errorf("lsp: %s: projectInfo of %s: %w", e.server.Name, p, err)
	}
	var reply struct {
		Body struct {
			ConfigFileName string `json:"configFileName"`
		} `json:"body"`
	}
	answered := protocol.Call(ctx, held.conn, protocol.MethodWorkspaceExecuteCommand, &protocol.ExecuteCommandParams{
		Command:   tsserverRequest,
		Arguments: []protocol.LSPAny{protocol.LSPAny(`"` + projectInfo + `"`), protocol.LSPAny(args)},
	}, &reply) == nil
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	return answered && slices.Contains(tsProjects, path.Base(filepath.ToSlash(reply.Body.ConfigFileName))), nil
}

// preloaded runs [Engine.preload] for the declaration whose name is at the protocol position at
// of doc, over the files that write the name, and returns what it returns. A declaration that
// its file does not offer to the rest of a program, such as a parameter or a local, has no use
// in another file, so preloaded opens no file for it.
func (e *Engine) preloaded(
	ctx context.Context,
	held *session,
	found *finder,
	doc document,
	at protocol.Position,
) (*trust.Caveat, []source.Path, error) {
	if !e.server.Scoped {
		return nil, nil, nil
	}
	offered, named, err := found.offers(ctx, doc.path, at)
	if err != nil || !offered {
		return nil, nil, err
	}
	declared := doc.path
	if !named {
		declared = ""
	}
	word := doc.word(at)
	return e.preload(ctx, held, word, uncommented(word), doc.path, declared)
}

// wording returns the rule of [Engine.preload] for a file that writes word as a word.
func wording(word string) func(source.Path, []byte) bool {
	return func(_ source.Path, content []byte) bool {
		return bytes.Contains(content, []byte(word)) && lang.Worded(string(content), word) >= 0
	}
}

// lineComment starts a line comment of JavaScript and TypeScript, the languages of every
// [Server.Scoped] server.
const lineComment = "//"

// uncommented returns the rule of [Engine.preload] for a file that writes word as a word on a
// line that does not start with [lineComment], by the rule of [occurrences].
func uncommented(word string) func(source.Path, []byte) bool {
	return func(_ source.Path, content []byte) bool {
		for range occurrences(content, word) {
			return true
		}
		return false
	}
}

// occurrences returns the byte offset of each occurrence of word as a word in content, by the
// rule of [lang.Worded], on a line that does not start with [lineComment]. Such a line has no
// use of a declaration, because a rename and a search for references leave a comment as it is.
// A reference directive is such a line, and it refers to a file, not to a declaration.
func occurrences(content []byte, word string) iter.Seq[int] {
	return func(yield func(int) bool) {
		text := string(content)
		from := 0
		// Each occurrence starts after the end of the one before it, so a text of n bytes has at
		// most n occurrences.
		for range len(text) {
			at := lang.Worded(text[from:], word)
			if at < 0 {
				return
			}
			at += from
			from = at + len(word)
			start := strings.LastIndexByte(text[:at], '\n') + 1
			if !strings.HasPrefix(strings.TrimLeft(text[start:], " \t"), lineComment) && !yield(at) {
				return
			}
		}
	}
}

// beyond returns the first path of changes that is outside the workspace, or the empty string
// when every path is inside. techne writes under its root only, so a plan with such a path
// cannot be applied as the server described it.
func beyond(changes []edit.Change) string {
	for _, one := range changes {
		for _, p := range []source.Path{one.Path, one.To} {
			if p != "" && filepath.IsAbs(filepath.FromSlash(string(p))) {
				return string(p)
			}
		}
	}
	return ""
}
