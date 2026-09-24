//go:build corpus

// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package corpus_test

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/test/corpus"
	"go.dokimi.dev/techne/tool"
)

// The limits of the phases of one repository.
const (
	cloning   = time.Hour
	preparing = 2 * time.Hour
	building  = 2 * time.Hour
	calling   = 5 * time.Minute
)

// budgeted are the read tools whose warm calls the budget of the manifest
// bounds.
var budgeted = []string{"outline", "search", "resolve", "relations"}

// probes is the number of declarations that each read probe checks.
const probes = 8

// extracted is the name of an extracted function in the naming convention
// of each language, which the analyzers of a build such as StyleCop enforce.
var extracted = map[string]string{
	"c": "extracted_corpus", "csharp": "ExtractedCorpus", "go": "extractedCorpus", "java": "extractedCorpus",
	"javascript": "extractedCorpus", "python": "extracted_corpus", "ruby": "extracted_corpus",
	"rust": "extracted_corpus", "scala": "extractedCorpus", "typescript": "extractedCorpus",
}

func TestCorpus(t *testing.T) {
	m, err := corpus.Load("corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	root, err := corpus.ModuleRoot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	dir := directory(t)
	binary, err := corpus.Binary(t.Context(), root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	report := &corpus.Report{Budget: time.Duration(m.Budget)}
	for _, r := range m.Select(os.Getenv("TECHNE_CORPUS_ONLY")) {
		t.Run(r.Name, func(t *testing.T) {
			section := &corpus.Section{Repository: r}
			defer func() {
				report.Add(*section)
				write(t, dir, report)
			}()
			drive(t, m, r, dir, root, binary, section)
		})
	}
}

// write writes the report to report.md in dir. A run writes it after each
// repository, so a run that stops early keeps the repositories it finished.
func write(t *testing.T, dir string, report *corpus.Report) {
	t.Helper()
	path := filepath.Join(dir, "report.md")
	if err := os.WriteFile(path, []byte(report.Markdown()), 0o644); err != nil {
		t.Errorf("write the report: %v", err)
		return
	}
	t.Logf("the report is at %s", path)
}

// directory returns the directory of the clones: TECHNE_CORPUS, or
// techne/corpus under the user cache directory.
func directory(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("TECHNE_CORPUS")
	if dir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		dir = filepath.Join(cache, "techne", "corpus")
	}
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// probe is one declaration that a read probe checks, with the file that
// declares it.
type probe struct {
	path string
	// parent is the name of the declaration that contains item, and empty
	// for a declaration at the top level of its file.
	parent string
	item   tool.Declaration
	// sites are the uses of the declaration that relations returned.
	sites []tool.Connected
	// form is the name by which relations found the declaration, and empty
	// when relations refused every form.
	form string
}

// drive runs every check over one repository and fills section.
func drive(t *testing.T, m corpus.Manifest, r corpus.Repository, dir, root, binary string, section *corpus.Section) {
	t.Helper()
	declared, known := declarations[r.Language]
	if !known {
		t.Fatalf("techne declares no language %s", r.Language)
	}
	log := logFile(t, dir, r.Name)

	w := within(t, cloning, func(ctx context.Context) (*corpus.Workspace, error) {
		return corpus.Open(ctx, dir, root, r, log)
	})
	t.Logf("%s is at %s", r.Name, w.Root)
	within(t, preparing, func(ctx context.Context) (struct{}, error) { return struct{}{}, w.Prepare(ctx) })

	baseline := 0
	if r.Writable() {
		t.Logf("building %s at %s", r.Name, r.Tag)
		output, err := build(t, w)
		count, counted := r.Count(output)
		switch {
		case r.Errors != "" && !counted:
			t.Fatalf("the build of %s at %s reports no error count:\n%s", r.Name, r.Tag, corpus.Tail(output, 30))
		case r.Errors == "" && err != nil:
			t.Fatalf("the build of %s fails at %s: %v", r.Name, r.Tag, err)
		}
		baseline = count
	}

	session := start(t, w, binary, log)
	defer func() { section.Calls = session.Calls() }()
	available(t, session, r.Language)

	files, err := w.Files(t.Context(), declared.Extensions)
	if err != nil {
		t.Fatal(err)
	}
	sampled := sample(t, w.Root, files, m.Sample, m.Seed)
	t.Logf("sampled %d of %d files: %s", len(sampled), len(files), strings.Join(sampled, ", "))

	budget := time.Duration(m.Budget)
	pool := outline(t, w, session, sampled, budget)
	section.Settled = warm(t, session, w, pool, time.Duration(r.Warmup))
	session.Warm()

	t.Run("Outline", func(t *testing.T) {
		t.Run("returns declarations that match the bytes of each file", func(t *testing.T) {
			pool = outline(t, w, session, sampled, budget)
		})
	})
	candidates := chosen(pool)

	t.Run("Search", func(t *testing.T) {
		t.Run("returns an exact match first", func(t *testing.T) {
			for _, p := range candidates[:min(probes, len(candidates))] {
				var found tool.Matches
				ask(t, session, budget, "search", map[string]any{
					"text": p.item.Name, "private": true, "limit": 20, "language": r.Language,
				}, &found)
				for _, problem := range corpus.Ranked(p.item.Name, found.Items) {
					t.Errorf("%s: %s", p.path, problem)
				}
			}
		})
	})

	t.Run("Relations", func(t *testing.T) {
		t.Run("returns sites that show the name", func(t *testing.T) {
			for i := range candidates[:min(probes, len(candidates))] {
				p := &candidates[i]
				related, form, refusal := relate(t, session, budget, r.Language, *p)
				if refusal != "" {
					t.Errorf("relations of %s in %s: %s", p.item.Name, p.path, refusal)
					continue
				}
				p.sites, p.form = related.Items, form
				for _, problem := range corpus.Sites(p.item.Name, related.Items, reader(w)) {
					t.Errorf("referenced-by %s: %s", p.item.Name, problem)
				}
			}
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Run("returns the declaration that a use names", func(t *testing.T) {
			asked := 0
			for _, p := range candidates {
				site, column, found := use(w, p)
				if !found || asked == probes {
					continue
				}
				asked++
				var resolved tool.Answer
				ask(t, session, budget, "resolve", map[string]any{
					"scope": site.Path, "line": site.Line, "column": column, "language": r.Language,
				}, &resolved)
				if !slices.ContainsFunc(resolved.Items, called(p.item.Name)) {
					t.Errorf("resolve at %s:%d:%d returned %s, not %s", site.Path, site.Line, column,
						names(resolved.Items), p.item.Name)
				}
			}
		})
	})

	t.Run("Verify", func(t *testing.T) {
		t.Run("returns the findings of a file without an error", func(t *testing.T) {
			var verified tool.VerifyOutput
			ask(t, session, budget, "verify", map[string]any{"scope": sampled[0], "language": r.Language}, &verified)
			if verified.Error != nil {
				t.Errorf("verify %s: %s: %s", sampled[0], verified.Error.Code, verified.Error.Reason)
			}
		})
	})

	if !r.Writable() {
		return
	}
	changes := changer{w: w, session: session, section: section, baseline: baseline}

	t.Run("Rename", func(t *testing.T) {
		t.Run("applies a rename that the build accepts", func(t *testing.T) {
			target, found := renamable(candidates)
			if !found {
				t.Skip("no sampled declaration has uses in two files")
			}
			renamed := target.item.Name + "Corpus"
			changes.apply(t, "rename.symbol", target.form, map[string]any{
				"scope": target.path, "name": target.form, "new_name": renamed,
				"kind": target.item.Kind.String(), "language": r.Language,
			}, func(t *testing.T) {
				t.Helper()
				for _, site := range target.sites {
					if filepath.IsAbs(site.Path) {
						continue
					}
					content, err := os.ReadFile(filepath.Join(w.Root, site.Path))
					written, _ := corpus.Line(content, site.Line)
					if err != nil || !strings.Contains(written, renamed) {
						t.Errorf("%s:%d does not show %s after the rename", site.Path, site.Line, renamed)
					}
				}
			})
		})
	})

	t.Run("Move", func(t *testing.T) {
		t.Run("applies a move that the build accepts", func(t *testing.T) {
			from := sampled[len(sampled)-1]
			to := moved(from)
			changes.apply(t, "move.file", from, map[string]any{
				"path": from, "to": to, "language": r.Language,
			}, func(t *testing.T) {
				t.Helper()
				if _, err := os.Stat(filepath.Join(w.Root, to)); err != nil {
					t.Errorf("%s is missing after the move: %v", to, err)
				}
			})
		})
	})

	t.Run("Extract", func(t *testing.T) {
		t.Run("applies an extraction that the build accepts", func(t *testing.T) {
			path, line, found := extractable(w, pool)
			if !found {
				t.Skip("no sampled function has a line that contains one call")
			}
			changes.apply(t, "extract.function", fmt.Sprintf("%s:%d", path, line), map[string]any{
				"path": path, "first_line": line, "last_line": line, "new_name": extracted[r.Language],
				"language": r.Language,
			}, nil)
		})
	})

	t.Run("Document", func(t *testing.T) {
		t.Run("applies documentation that the outline reads back", func(t *testing.T) {
			target, found := undocumented(candidates)
			if !found {
				t.Skip("every sampled declaration has documentation")
			}
			const doc = "Corpus documentation."
			changes.apply(t, "document.symbol", target.form, map[string]any{
				"scope": target.path, "name": target.form, "doc": doc,
				"kind": target.item.Kind.String(), "language": r.Language,
			}, func(t *testing.T) {
				t.Helper()
				var read tool.Answer
				ask(t, session, 0, "outline", map[string]any{
					"scope": target.path, "names": []string{target.item.Name}, "detail": "docs", "private": true,
					"max_tokens": 10_000_000, "language": r.Language,
				}, &read)
				var every []probe
				for _, item := range read.Items {
					every = append(every, flattened(target.path, "", item)...)
				}
				documented := func(p probe) bool {
					return p.item.Name == target.item.Name && strings.Contains(p.item.Doc, doc)
				}
				if !slices.ContainsFunc(every, documented) {
					t.Errorf("the outline of %s does not read the documentation of %s back",
						target.path, target.item.Name)
				}
			})
		})
	})
}

// within runs fn with a context that ends after limit, and fails the test
// when fn fails.
func within[T any](t *testing.T, limit time.Duration, fn func(context.Context) (T, error)) T {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), limit)
	defer cancel()
	out, err := fn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// logFile opens the log of a repository for appending, and closes it when
// the test ends.
func logFile(t *testing.T, dir, name string) io.Writer {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, "logs", name+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	fmt.Fprintf(f, "\n=== %s %s\n", name, time.Now().Format(time.RFC3339))
	return f
}

// build runs the build of the workspace within its limit, and returns its
// output and its error.
func build(t *testing.T, w *corpus.Workspace) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), building)
	defer cancel()
	return w.Build(ctx)
}

// start runs techne over the workspace, with the variables that csharp-ls
// needs, and closes the session when the test ends.
func start(t *testing.T, w *corpus.Workspace, binary string, log io.Writer) *corpus.Session {
	t.Helper()
	session, err := corpus.Start(t.Context(), binary, w.Root, dotnet(w.Environ()), log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Logf("close the session: %v", err)
		}
	})
	return session
}

// dotnet returns env with the directory of the .NET tools on PATH and with
// DOTNET_ROOT at the installation of the SDK, which csharp-ls needs to run.
func dotnet(env []string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return env
	}
	if tools := filepath.Join(home, ".dotnet", "tools"); exists(tools) {
		env = append(env, "PATH="+lookup(env, "PATH")+string(os.PathListSeparator)+tools)
	}
	if lookup(env, "DOTNET_ROOT") != "" {
		return env
	}
	listed, err := exec.Command("dotnet", "--list-sdks").Output()
	if err != nil {
		return env
	}
	if found := regexp.MustCompile(`\[(.+)\]`).FindSubmatch(listed); found != nil {
		env = append(env, "DOTNET_ROOT="+filepath.Dir(string(found[1])))
	}
	return env
}

// lookup returns the last value of name in env.
func lookup(env []string, name string) string {
	value := ""
	for _, pair := range env {
		if k, v, found := strings.Cut(pair, "="); found && k == name {
			value = v
		}
	}
	return value
}

// exists reports whether path exists.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// available fails the test unless an engine answers resolve for language at
// the resolved tier and can run.
func available(t *testing.T, session *corpus.Session, language string) {
	t.Helper()
	var report tool.CapabilitiesOutput
	ask(t, session, 0, "capabilities", map[string]any{"language": language}, &report)
	var reasons []string
	for _, c := range report.Items {
		if c.Role != "resolve" || c.Fidelity != "resolved" {
			continue
		}
		if c.Available {
			return
		}
		reasons = append(reasons, c.Engine+": "+c.Unavailable)
	}
	t.Fatalf("no engine resolves %s: %s", language, strings.Join(reasons, "; "))
}

// ask calls tool and fails the test when the call fails. It also fails the
// test when a warm call of a budgeted tool takes longer than budget. A zero
// budget checks no time.
func ask(t *testing.T, session *corpus.Session, budget time.Duration, name string, arguments, out any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), calling)
	defer cancel()
	call, err := session.Call(ctx, name, arguments, out)
	if err != nil {
		t.Fatal(err)
	}
	if budget > 0 && call.Warm && slices.Contains(budgeted, name) && call.Took > budget {
		t.Errorf("%s %v took %s, over the budget of %s", name, arguments, call.Took.Round(time.Millisecond), budget)
	}
}

// sample returns n of paths: the largest half, then a draw with seed from
// the rest.
func sample(t *testing.T, root string, paths []string, n int, seed uint64) []string {
	t.Helper()
	if len(paths) == 0 {
		t.Fatal("the repository has no file of its language")
	}
	sizes := map[string]int64{}
	for _, p := range paths {
		if info, err := os.Stat(filepath.Join(root, p)); err == nil {
			sizes[p] = info.Size()
		}
	}
	bySize := slices.Clone(paths)
	slices.SortStableFunc(bySize, func(a, b string) int { return int(sizes[b] - sizes[a]) })
	largest := bySize[:min(n/2, len(bySize))]
	rest := bySize[len(largest):]
	draw := rand.New(rand.NewPCG(seed, seed))
	draw.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	return append(slices.Clone(largest), rest[:min(n-len(largest), len(rest))]...)
}

// outline outlines each file twice and reports each declaration that
// disagrees with its file. The outline at the detail source returns the
// span and the snippet of each top-level declaration, and no members,
// because a snippet contains them. The outline at the detail docs returns
// every declaration with its line and its documentation. outline returns
// every declaration of the second, the members of a declaration included.
func outline(t *testing.T, w *corpus.Workspace, session *corpus.Session, files []string, budget time.Duration) []probe {
	t.Helper()
	var out []probe
	for _, path := range files {
		content, err := os.ReadFile(filepath.Join(w.Root, path))
		if err != nil {
			t.Fatal(err)
		}
		var sourced, documented tool.Answer
		for detail, answer := range map[string]*tool.Answer{"source": &sourced, "docs": &documented} {
			ask(t, session, budget, "outline", map[string]any{
				"scope": path, "detail": detail, "private": true, "include": []string{"all"},
				"max_tokens": 10_000_000, "language": w.Repository.Language,
			}, answer)
		}
		if failed := cmp.Or(sourced.Error, documented.Error); failed != nil {
			t.Errorf("outline %s: %s: %s", path, failed.Code, failed.Reason)
			continue
		}
		for _, problem := range corpus.Spans(content, sourced.Items) {
			t.Errorf("%s: %s", path, problem)
		}
		for _, item := range documented.Items {
			out = append(out, flattened(path, "", item)...)
		}
	}
	return out
}

// flattened returns d, whose container is named parent, and every
// declaration nested in it, as the probes of the file at path.
func flattened(path, parent string, d tool.Declaration) []probe {
	out := []probe{{path: path, parent: parent, item: d}}
	for _, member := range d.Members {
		out = append(out, flattened(path, d.Name, member)...)
	}
	return out
}

// relate asks relations for the uses of p as an agent asks: by its name and
// kind, and after a refusal of the name as ambiguous by the qualified name
// that the refusal asks for. It returns the answer, the name that relations
// accepted, and the reason of the last refusal.
func relate(
	t *testing.T,
	session *corpus.Session,
	budget time.Duration,
	language string,
	p probe,
) (tool.RelationsOutput, string, string) {
	t.Helper()
	refusal := ""
	for _, form := range forms(p) {
		var related tool.RelationsOutput
		ask(t, session, budget, "relations", map[string]any{
			"scope": p.path, "name": form, "kind": p.item.Kind.String(), "relation": "referenced-by",
			"limit": 50, "language": language,
		}, &related)
		if related.Error == nil {
			return related, form, ""
		}
		refusal = related.Error.Code + ": " + related.Error.Reason
		if !strings.Contains(related.Error.Reason, "declarations in") {
			break
		}
	}
	return tool.RelationsOutput{}, "", refusal
}

// forms returns the names by which an agent addresses p: its name, then the
// name qualified by its container. A Go method has no container in an
// outline, so the type of its receiver qualifies it.
func forms(p probe) []string {
	out := []string{p.item.Name}
	qualifier := p.parent
	if found := receiver.FindStringSubmatch(p.item.Signature); qualifier == "" && found != nil {
		qualifier = found[1]
	}
	if qualifier != "" {
		out = append(out, qualifier+"."+p.item.Name)
	}
	return out
}

// receiver matches the type of the receiver in the signature of a Go
// method.
var receiver = regexp.MustCompile(`^func\s*\(\s*(?:[A-Za-z_]\w*\s+)?\*?\s*([A-Za-z_]\w*)`)

// warm resolves a declaration of pool until the language server settles:
// until it answers with total coverage at the resolved or the indexed tier.
// techne lowers an answer about a project with errors to the indexed tier.
// warm returns how long the server took, logs each change of the answer and
// a line a minute, and fails the test when the server does not settle within
// limit or when no engine can serve the question.
func warm(t *testing.T, session *corpus.Session, w *corpus.Workspace, pool []probe, limit time.Duration) time.Duration {
	t.Helper()
	target, line, column, found := named(w, pool)
	if !found {
		t.Fatal("no sampled declaration writes its name on its first line")
	}
	t.Logf("resolving %s at %s:%d:%d until the server settles", target.item.Name, target.path, line, column)
	start, logged, last := time.Now(), time.Now(), ""
	for {
		var answer tool.Answer
		ask(t, session, 0, "resolve", map[string]any{
			"scope": target.path, "line": line, "column": column, "language": w.Repository.Language,
		}, &answer)
		state := answer.Provenance.Fidelity + ", " + answer.Provenance.Completeness
		if answer.Error != nil {
			state = answer.Error.Code + ": " + answer.Error.Reason
		}
		if state != last || time.Since(logged) > time.Minute {
			t.Logf("after %s: %s", time.Since(start).Round(time.Second), state)
			last, logged = state, time.Now()
		}
		switch {
		case answer.Error == nil && (state == "resolved, total" || state == "indexed, total"):
			return time.Since(start)
		case answer.Error != nil && answer.Error.Code == "unsupported":
			// No engine can serve the question, which waiting does not change.
			t.Errorf("no engine resolves %s: %s", w.Repository.Language, answer.Error.Reason)
			return 0
		}
		if time.Since(start) > limit {
			t.Errorf("the server did not settle within %s: %s", limit, state)
			return 0
		}
		time.Sleep(5 * time.Second)
	}
}

// named returns a declaration of pool whose first line writes its name, with
// the line and the byte column of the name, counted from one.
func named(w *corpus.Workspace, pool []probe) (probe, int, int, bool) {
	for _, p := range chosen(pool) {
		content, err := os.ReadFile(filepath.Join(w.Root, p.path))
		if err != nil {
			continue
		}
		written, _ := corpus.Line(content, p.item.Line)
		if at := word(written, p.item.Name); at >= 0 {
			return p, p.item.Line, at + 1, true
		}
	}
	return probe{}, 0, 0, false
}

// chosen returns the declarations of pool that the probes check: the
// functions, methods, types and constants that code refers to by name, one
// file after the other, so the first probes cover the most files.
func chosen(pool []probe) []probe {
	byPath := map[string][]probe{}
	var paths []string
	for _, p := range pool {
		switch p.item.Kind {
		case sema.KindFunction, sema.KindMethod, sema.KindStruct, sema.KindInterface, sema.KindType,
			sema.KindConstant, sema.KindEnum:
		default:
			continue
		}
		if !identifier(p.item.Name) {
			continue
		}
		if _, seen := byPath[p.path]; !seen {
			paths = append(paths, p.path)
		}
		byPath[p.path] = append(byPath[p.path], p)
	}
	var out []probe
	for round := 0; ; round++ {
		added := false
		for _, path := range paths {
			if round < len(byPath[path]) {
				out, added = append(out, byPath[path][round]), true
			}
		}
		if !added {
			return out
		}
	}
}

// identifier reports whether name is a plain identifier, which the probes
// can find as a word in a line.
func identifier(name string) bool {
	for _, r := range name {
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// word returns the byte offset of the first occurrence of name in line that
// no identifier character touches, or -1.
func word(line, name string) int {
	for from := 0; ; {
		at := strings.Index(line[from:], name)
		if at < 0 {
			return -1
		}
		at += from
		end := at + len(name)
		if !touches(line, at-1) && !touches(line, end) {
			return at
		}
		from = end
	}
}

// touches reports whether the byte at i of line belongs to an identifier.
func touches(line string, i int) bool {
	if i < 0 || i >= len(line) {
		return false
	}
	b := line[i]
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// use returns a site of p in the workspace and outside its own declaration,
// with the byte column of the name on that line, counted from one.
func use(w *corpus.Workspace, p probe) (tool.Connected, int, bool) {
	for _, site := range p.sites {
		if site.Path == p.path && site.Line == p.item.Line || filepath.IsAbs(site.Path) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(w.Root, site.Path))
		if err != nil {
			continue
		}
		written, _ := corpus.Line(content, site.Line)
		if at := word(written, p.item.Name); at >= 0 {
			return site, at + 1, true
		}
	}
	return tool.Connected{}, 0, false
}

// called returns a function that reports whether a declaration is named
// name.
func called(name string) func(tool.Declaration) bool {
	return func(d tool.Declaration) bool { return d.Name == name }
}

// names returns the names of items, joined for a message.
func names(items []tool.Declaration) string {
	var out []string
	for _, d := range items {
		out = append(out, d.Name)
	}
	return "[" + strings.Join(out, ", ") + "]"
}

// reader returns a function that reads the path of an answer: a path in the
// workspace, or the absolute path of a file outside it.
func reader(w *corpus.Workspace) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		if !filepath.IsAbs(p) {
			p = filepath.Join(w.Root, p)
		}
		return os.ReadFile(p)
	}
}

// renamable returns the candidate with the most uses in two files or more.
func renamable(candidates []probe) (probe, bool) {
	best, found := probe{}, false
	for _, p := range candidates {
		files := map[string]bool{}
		for _, site := range p.sites {
			files[site.Path] = true
		}
		if len(files) >= 2 && (!found || len(p.sites) > len(best.sites)) {
			best, found = p, true
		}
	}
	return best, found
}

// undocumented returns a candidate function, method or type without
// documentation, which a relations probe found.
func undocumented(candidates []probe) (probe, bool) {
	for _, p := range candidates {
		switch p.item.Kind {
		case sema.KindFunction, sema.KindMethod, sema.KindStruct:
			if p.item.Doc == "" && p.form != "" {
				return p, true
			}
		}
	}
	return probe{}, false
}

// moved returns the path that a move of path goes to: the same directory,
// with Corpus appended to a name in upper case, and _corpus to any other.
func moved(path string) string {
	dir, base := filepath.Split(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if first := []rune(stem); len(first) > 0 && unicode.IsUpper(first[0]) {
		return dir + stem + "Corpus" + ext
	}
	return dir + stem + "_corpus" + ext
}

// extractable returns a line inside a sampled function of a file that no
// generator wrote, where the line contains one call statement that the line
// before it does not continue, which an extraction can lift into a function
// of its own. The body of a function ends before the next declaration of
// its file, or 60 lines after its first line.
func extractable(w *corpus.Workspace, pool []probe) (string, int, bool) {
	starts := map[string][]int{}
	for _, p := range pool {
		starts[p.path] = append(starts[p.path], p.item.Line)
	}
	for _, p := range pool {
		if p.item.Kind != sema.KindFunction && p.item.Kind != sema.KindMethod {
			continue
		}
		content, err := os.ReadFile(filepath.Join(w.Root, p.path))
		if err != nil || generated(content) {
			continue
		}
		first, last := p.item.Line+1, p.item.Line+60
		for _, start := range starts[p.path] {
			if start > p.item.Line && start < last {
				last = start
			}
		}
		for line := first; line < last; line++ {
			written, _ := corpus.Line(content, line)
			before, _ := corpus.Line(content, line-1)
			if call.MatchString(written) && !continues(before) {
				return p.path, line, true
			}
		}
	}
	return "", 0, false
}

// continues reports whether line ends in a token after which a statement
// goes on, such as the dot of a method chain or an operator.
func continues(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed != "" && strings.ContainsAny(trimmed[len(trimmed)-1:], ".,([+-*/%&|^=<>!?:")
}

// generated reports whether the start of content carries the marker of a
// code generator. gopls offers no refactoring in such a file.
func generated(content []byte) bool {
	head := content[:min(len(content), 2048)]
	for _, marker := range []string{"DO NOT EDIT", "@generated", "<auto-generated"} {
		if bytes.Contains(head, []byte(marker)) {
			return true
		}
	}
	return false
}

// call matches a line that contains one call statement: a name or a dotted
// selector, with arguments that close on the line and an optional
// semicolon.
var call = regexp.MustCompile(`^\s*[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*\([^()]*\);?\s*$`)

// changer applies changes to one workspace and judges them with its build.
type changer struct {
	w       *corpus.Workspace
	session *corpus.Session
	section *corpus.Section
	// baseline is the number of errors that the build reports at the commit
	// of the repository.
	baseline int
}

// apply previews a change with operation, applies it by its handle, builds
// the workspace, runs check, and resets the workspace. A change that techne
// declines is recorded and skips the test. A change that does not apply, or
// that the build refuses, fails it.
func (c changer) apply(t *testing.T, operation, target string, arguments map[string]any, check func(*testing.T)) {
	t.Helper()
	keep, err := c.w.Untracked(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if failed := c.w.Reset(context.WithoutCancel(t.Context()), keep); failed != nil {
			t.Errorf("reset %s: %v", c.w.Root, failed)
		}
	}()
	outcome := corpus.Outcome{Operation: operation, Target: target}
	defer func() { c.section.Outcomes = append(c.section.Outcomes, outcome) }()

	var preview tool.Written
	ask(t, c.session, 0, operation, arguments, &preview)
	if preview.Error != nil || preview.Handle == "" {
		outcome.Result = "declined"
		if preview.Error != nil {
			outcome.Detail = preview.Error.Code + ": " + preview.Error.Reason
		}
		if preview.Verified != nil && preview.Verified.Result != "" {
			outcome.Detail += " (" + preview.Verified.Gate + ": " + preview.Verified.Result + ")"
		}
		for _, caveat := range preview.Provenance.Caveats {
			if caveat.Code != "dynamic" {
				outcome.Detail += " [" + caveat.Code + ": " + caveat.Note + "]"
			}
		}
		t.Skipf("techne declined %s of %s: %s", operation, target, outcome.Detail)
	}

	var written tool.Written
	ask(t, c.session, 0, "apply.change", map[string]any{"handle": preview.Handle}, &written)
	if written.Error != nil || !written.Applied {
		outcome.Result = "failed"
		if written.Error != nil {
			outcome.Detail = written.Error.Code + ": " + written.Error.Reason
		}
		t.Fatalf("techne did not apply %s of %s: %s", operation, target, outcome.Detail)
	}

	gated := "no gate"
	if written.Verified != nil {
		gated = written.Verified.Gate + " by " + written.Verified.Engine
	}
	output, err := build(t, c.w)
	count, counted := c.w.Repository.Count(output)
	switch {
	case c.w.Repository.Errors != "" && (!counted || count > c.baseline):
		outcome.Result = "failed"
		outcome.Detail = fmt.Sprintf("gated by %s, %d errors against %d before", gated, count, c.baseline)
		t.Errorf("the build after %s of %s reports %d errors against %d before:\n%s",
			operation, target, count, c.baseline, corpus.Tail(output, 30))
	case c.w.Repository.Errors == "" && err != nil:
		outcome.Result, outcome.Detail = "failed", "gated by "+gated+": "+corpus.Tail(output, 5)
		t.Errorf("the build after %s of %s fails: %v", operation, target, err)
	default:
		outcome.Result = "applied"
		outcome.Detail = fmt.Sprintf("%d files, gated by %s, built", len(written.Items), gated)
	}
	if check != nil {
		check(t)
	}
}
