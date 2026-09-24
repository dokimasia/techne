// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/service/change"
)

const (
	fixture  = source.Language("fixture")
	other    = source.Language("other")
	original = "one\ntwo\n"
)

// asking returns a request to document the declaration at the start of a.fx.
func asking(dry bool) edit.Request {
	return edit.Request{
		Operation: edit.DocumentSymbol,
		Scope:     "a.fx",
		Language:  fixture,
		Target:    edit.Target{Kind: edit.TargetSpan, Span: source.Span{Path: "a.fx"}},
		Args:      edit.Args{edit.ArgDoc: "Doc."},
		DryRun:    dry,
	}
}

// serving returns a service over engines and a workspace that contains a.fx.
func serving(t *testing.T, engines ...engine.Engine) (*workspace, *change.Service) {
	t.Helper()
	catalogue := engine.NewCatalog()
	for _, e := range engines {
		assert.NoError(t, catalogue.Add(e), "Add of "+e.Name())
	}
	files := &workspace{content: map[source.Path]string{"a.fx": original}}
	return files, change.New(catalogue, router{}, files)
}

// workspace is a directory in memory. Its methods are safe for concurrent use, like the
// methods of a directory. Its lock admits one writer at a time.
type workspace struct {
	mu      sync.Mutex
	content map[source.Path]string
	// refuse is the path that Write and Move refuse, so a case can fail a write partway.
	refuse source.Path
	// wrote counts the writes of each path.
	wrote map[source.Path]int
	// moves counts the calls of Move.
	moves int
	// outside counts the changes made while no writer has the lock.
	outside int
	// locked reports whether a writer has the lock.
	locked bool
	// jammed is the error of Lock, when it is set.
	jammed error
	// meanwhile runs once, when the first writer takes the lock, so a case can change a
	// file after the plan read it.
	meanwhile func(*workspace)
	// writer is the lock of the workspace.
	writer sync.Mutex
}

// at returns the content of the file at p, or the empty string for none.
func (w *workspace) at(p source.Path) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.content[p]
}

// has reports whether a file is at p.
func (w *workspace) has(p source.Path) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, found := w.content[p]
	return found
}

// put sets the content of the file at p, as another writer does.
func (w *workspace) put(p source.Path, text string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.content[p] = text
}

// writes returns the number of writes of p.
func (w *workspace) writes(p source.Path) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.wrote[p]
}

func (w *workspace) Read(p source.Path) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	text, found := w.content[p]
	if !found {
		return nil, fmt.Errorf("read %s: %w", p, fs.ErrNotExist)
	}
	return []byte(text), nil
}

func (w *workspace) Write(p source.Path, content []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p == w.refuse {
		return fmt.Errorf("the workspace does not take a write to %s", p)
	}
	w.changed()
	if w.wrote == nil {
		w.wrote = map[source.Path]int{}
	}
	w.wrote[p]++
	w.content[p] = string(content)
	return nil
}

func (w *workspace) Remove(p source.Path) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.changed()
	delete(w.content, p)
	return nil
}

func (w *workspace) Move(from, to source.Path) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	text, found := w.content[from]
	_, taken := w.content[to]
	switch {
	case to == w.refuse:
		return fmt.Errorf("the workspace does not take a move to %s", to)
	case !found:
		return fmt.Errorf("move %s: %w", from, fs.ErrNotExist)
	case taken:
		return fmt.Errorf("move %s: a file is at %s", from, to)
	}
	w.changed()
	w.moves++
	w.content[to] = text
	delete(w.content, from)
	return nil
}

func (w *workspace) Lock(context.Context) (func(), error) {
	if w.jammed != nil {
		return nil, w.jammed
	}
	w.writer.Lock()
	w.mu.Lock()
	w.locked = true
	meanwhile := w.meanwhile
	w.meanwhile = nil
	w.mu.Unlock()
	if meanwhile != nil {
		meanwhile(w)
	}
	return func() {
		w.mu.Lock()
		w.locked = false
		w.mu.Unlock()
		w.writer.Unlock()
	}, nil
}

// changed counts a change made while no writer has the lock. The caller has locked mu.
func (w *workspace) changed() {
	if !w.locked {
		w.outside++
	}
}

// router routes a file by its extension, .fx to fixture and .ot to other, and asks both
// languages about a directory.
type router struct{}

func (router) LanguageOf(p source.Path) (source.Language, bool) {
	switch path.Ext(string(p)) {
	case ".fx":
		return fixture, true
	case ".ot":
		return other, true
	}
	return "", false
}

func (router) Languages() []source.Language { return []source.Language{fixture, other} }

// planner is an engine that plans the changes of a case, or one comment at the top of the
// file of the request when changes is nil. Its name is planner, its language fixture and
// its tier syntactic unless the case sets them.
type planner struct {
	name     string
	language source.Language
	fidelity trust.Fidelity
	changes  []edit.Change
	empty    bool
	skipped  bool
	declines bool
	breaks   bool
	refuses  string
}

func (p planner) Name() string                        { return cmp.Or(p.name, "planner") }
func (p planner) Language() source.Language           { return cmp.Or(p.language, fixture) }
func (p planner) Fidelity(engine.Role) trust.Fidelity { return cmp.Or(p.fidelity, trust.Syntactic) }
func (planner) Cost(engine.Role) engine.Cost          { return engine.CostParse }

func (p planner) Plan(
	_ context.Context,
	req engine.Request,
	_ edit.Operation,
	_ edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	switch {
	case p.declines:
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: not this one", engine.ErrDecline)
	case p.refuses != "":
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %s", engine.ErrRefuse, p.refuses)
	case p.breaks:
		return engine.Result[edit.Change]{}, errors.New("the planner is broken")
	case p.skipped:
		return engine.Result[edit.Change]{Completeness: trust.ScopeTotal, Skipped: true}, nil
	case p.empty:
		return engine.Result[edit.Change]{Completeness: trust.ScopeTotal}, nil
	case p.changes != nil:
		return engine.Result[edit.Change]{Items: p.changes, Completeness: trust.ScopeTotal}, nil
	}
	return engine.Result[edit.Change]{Items: comment(req.Scope, args[edit.ArgDoc]), Completeness: trust.ScopeTotal}, nil
}

// comment returns the change that inserts a comment of doc at the start of the file at p.
func comment(p source.Path, doc string) []edit.Change {
	return []edit.Change{{Kind: edit.ChangeEdit, Path: p, Edits: []edit.TextEdit{{New: "// " + doc + "\n"}}}}
}

// replacing returns the edit of the file at p that replaces the bytes from start to end
// with text.
func replacing(p source.Path, start, end int, text string) edit.Change {
	return edit.Change{Kind: edit.ChangeEdit, Path: p, Edits: []edit.TextEdit{{
		Span: source.Span{Path: p, Start: source.Position{Offset: start}, End: source.Position{Offset: end}},
		New:  text,
	}}}
}

// checker is a gate of the language fixture. Its tier is syntactic unless the case sets
// one, and faults returns the errors of each file, or none when faults is nil.
type checker struct {
	name     string
	fidelity trust.Fidelity
	faults   func(p source.Path, content string) []diag.Diagnostic
}

// clean returns a gate that does not find an error.
func clean() checker { return checker{name: "clean"} }

// marking returns a gate that finds an error on each line that contains mark.
func marking(mark string) checker { return checker{name: "marking", faults: marked(mark)} }

func (c checker) Name() string                        { return c.name }
func (checker) Language() source.Language             { return fixture }
func (c checker) Fidelity(engine.Role) trust.Fidelity { return cmp.Or(c.fidelity, trust.Syntactic) }
func (checker) Cost(engine.Role) engine.Cost          { return engine.CostParse }

func (c checker) Check(_ context.Context, files map[source.Path][]byte) (engine.Result[edit.Finding], error) {
	var out []edit.Finding
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if files[p] == nil || c.faults == nil {
			continue
		}
		for _, one := range c.faults(p, string(files[p])) {
			out = append(out, edit.Finding{Diagnostic: one})
		}
	}
	return engine.Result[edit.Finding]{Items: out, Completeness: trust.ScopeTotal}, nil
}

// marked returns the faults of an error on each line that contains mark.
func marked(mark string) func(source.Path, string) []diag.Diagnostic {
	return func(p source.Path, content string) []diag.Diagnostic {
		var out []diag.Diagnostic
		for i, line := range strings.Split(content, "\n") {
			if strings.Contains(line, mark) {
				out = append(out, fault(p, content, i))
			}
		}
		return out
	}
}

// fault returns an error that covers the zero-based line of content.
func fault(p source.Path, content string, line int) diag.Diagnostic {
	lines := strings.SplitAfter(content, "\n")
	start := 0
	for _, one := range lines[:line] {
		start += len(one)
	}
	end := start + len(strings.TrimSuffix(lines[line], "\n"))
	return diag.Diagnostic{
		Severity: diag.SeverityError,
		Code:     "parse",
		Message:  "this does not parse",
		Span: source.Span{
			Path:  p,
			Start: source.Position{Offset: start, Line: line},
			End:   source.Position{Offset: end, Line: line, Column: end - start},
		},
	}
}

func TestService(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the changes of a plan", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(false))
			assert.NoError(t, err, "Apply")
			assert.True(t, got.Applied, "the application of the change")
			assert.Equal(t, got.Status, trust.OK, "the status of the outcome")
			assert.Equal(t, got.Changed, []source.Path{"a.fx"}, "the files written")
			assert.Equal(t, files.at("a.fx"), "// Doc.\n"+original, "the content of a.fx")
		})

		t.Run("writes no file for a dry run", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.False(t, got.Applied, "the application of the change")
			assert.Empty(t, got.Changed, "the files written")
			assert.Equal(t, files.at("a.fx"), original, "the content of a.fx")
			assert.Length(t, got.Rewrites, 1, "the rewrites of the outcome")
		})

		t.Run("refuses an operation that no spec declares", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{})
			req := asking(true)
			req.Operation = "document.everything"
			got, err := s.Apply(t.Context(), req)
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Contains(t, got.Reason, "document.everything", "the reason of the refusal")
		})

		t.Run("refuses a target that the operation does not accept", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{})
			req := asking(true)
			req.Target = edit.Target{Kind: edit.TargetFile, Path: "a.fx"}
			got, err := s.Apply(t.Context(), req)
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Contains(t, got.Reason, "a file", "the reason of the refusal")
		})

		t.Run("refuses a request without an argument that the operation requires", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{})
			req := asking(true)
			req.Args = edit.Args{}
			got, err := s.Apply(t.Context(), req)
			assert.NoError(t, err, "Apply")
			assert.Contains(t, got.Reason, string(edit.ArgDoc), "the reason of the refusal")
		})

		t.Run("refuses an argument that the operation does not read", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{})
			req := asking(true)
			req.Args = edit.Args{edit.ArgDoc: "x", edit.ArgKey("documentation"): "y"}
			got, err := s.Apply(t.Context(), req)
			assert.NoError(t, err, "Apply")
			assert.Contains(t, got.Reason, "documentation", "the reason of the refusal")
		})

		t.Run("returns unsupported when no engine plans the operation", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{declines: true})
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Unsupported, "the status of the outcome")
			assert.False(t, got.Provenance.SupportsNegativeClaim(), "the negative claim of the outcome")
		})

		t.Run("returns the reason of a planner that refuses", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{refuses: "the text closes its own comment"})
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, got.Reason, "the text closes its own comment", "the reason of the refusal")
		})

		t.Run("returns the error of a planner that fails", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{breaks: true})
			_, err := s.Apply(t.Context(), asking(true))
			assert.HasError(t, err, "the error of Apply")
		})

		t.Run("refuses a plan below the tier of the operation", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{})
			req := asking(true)
			req.Operation = edit.RenameSymbol
			req.Target = edit.Target{Kind: edit.TargetSymbol, Symbol: "x"}
			req.Args = edit.Args{edit.ArgNewName: "y"}
			got, err := s.Apply(t.Context(), req)
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Contains(t, got.Reason, "evidence", "the reason of the refusal")
		})

		t.Run("refuses a plan without a change", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{empty: true}, clean())
			got, err := s.Apply(t.Context(), asking(true))
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Status, trust.Refused, "the status of the outcome")
			assert.Equal(t, got.Reason, "planner planned no change", "the reason of the refusal")
			assert.Empty(t, got.Handle, "the handle of the outcome")
		})

		t.Run("asks the next language after a skipped plan", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t,
				planner{skipped: true},
				planner{name: "other", language: other, changes: comment("b.ot", "Other.")},
			)
			files.put("b.ot", original)
			req := asking(false)
			req.Scope, req.Language = "src", ""
			got, err := s.Apply(t.Context(), req)
			assert.NoError(t, err, "Apply")
			assert.Equal(t, got.Provenance.Engine, "other", "the engine of the plan")
			assert.Equal(t, files.at("b.ot"), "// Other.\n"+original, "the content of b.ot")
		})
	})
}
