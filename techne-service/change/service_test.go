// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change_test

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/service/change"
)

func TestService(t *testing.T) {
	t.Parallel()

	t.Run("Apply", func(t *testing.T) {
		t.Parallel()

		t.Run("writes what a plan describes", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(false))

			assert.NoError(t, err, "a change that plans and gates cleanly is applied")
			assert.True(t, got.Applied, "the caller is told the workspace changed")
			assert.Equal(t, got.Status, trust.OK, "a change nothing objected to is not degraded")
			assert.Equal(t, got.Changed, []source.Path{"a.fx"}, "the result names what it wrote")
			assert.Equal(t, files.at("a.fx"), "// Doc.\none\ntwo\n", "the edit landed where it said")
		})

		t.Run("writes nothing for a dry run", func(t *testing.T) {
			t.Parallel()
			// A dry run is the real call without the write, so what it
			// reports is what an apply would do, not a diff to read.
			files, s := serving(t, planner{}, clean())
			got, err := s.Apply(t.Context(), asking(true))

			assert.NoError(t, err, "previewing a change that plans cleanly succeeds")
			assert.False(t, got.Applied, "a dry run leaves the workspace alone")
			assert.Empty(t, got.Changed, "nothing was written, so nothing is named as written")
			assert.Equal(t, files.at("a.fx"), original, "every byte is where it was")
			assert.Length(t, got.Rewrites, 1, "the caller still reads what would change")
		})

		t.Run("refuses a change that stops the file parsing", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{}, faults("this does not parse"))
			got, err := s.Apply(t.Context(), asking(false))

			assert.NoError(t, err, "a gate that objects is an answer, not a fault")
			assert.False(t, got.Applied, "a change the gate refused is not written")
			assert.Equal(t, got.Status, trust.Refused, "the caller is told it was refused")
			assert.Length(t, got.Diagnostics, 1, "the refusal carries what was wrong")
			assert.Equal(t, files.at("a.fx"), original, "a refused change leaves every file as it was")
		})

		t.Run("does not blame a change for what it inherited", func(t *testing.T) {
			t.Parallel()
			// The gate judges what the change replaces as well as what it
			// produces. Refusing on faults that were already there would
			// make the code that most wants fixing the code nothing may
			// touch.
			files, s := serving(t, planner{}, always())
			got, err := s.Apply(t.Context(), asking(false))

			assert.NoError(t, err, "a file that already had faults can still be changed")
			assert.True(t, got.Applied, "the change added no fault, so it is not the reason for one")
			assert.Equal(t, files.at("a.fx"), "// Doc.\none\ntwo\n", "the edit landed")
		})

		t.Run("reports a change nothing could judge as degraded", func(t *testing.T) {
			t.Parallel()
			files, s := serving(t, planner{})
			got, err := s.Apply(t.Context(), asking(false))

			assert.NoError(t, err, "a language with no gate is not refused")
			assert.True(t, got.Applied, "refusing would make the language unwritable")
			assert.Equal(t, got.Status, trust.Degraded,
				"the caller is holding a change that was admitted rather than verified")
			assert.NotEmpty(t, got.Provenance.Caveats, "and is told why")
			assert.Equal(t, files.at("a.fx"), "// Doc.\none\ntwo\n", "the edit still landed")
		})

		t.Run("refuses an operation nobody declared", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{})
			req := asking(true)
			req.Operation = "document.everything"
			got, err := s.Apply(t.Context(), req)

			assert.NoError(t, err, "a name nobody declared is something the caller can correct")
			assert.Equal(t, got.Status, trust.Refused, "nothing was attempted")
			assert.Contains(t, got.Reason, "document.everything", "the refusal names what was asked for")
		})

		t.Run("refuses a target the operation cannot be pointed at", func(t *testing.T) {
			t.Parallel()
			// Validated against the spec before any language is
			// consulted, so a malformed request produces one refusal
			// whatever would have served it.
			_, s := serving(t, planner{})
			req := asking(true)
			req.Target = edit.Target{Kind: edit.TargetFile, Path: "a.fx"}
			got, err := s.Apply(t.Context(), req)

			assert.NoError(t, err, "pointing an operation at the wrong thing is correctable")
			assert.Equal(t, got.Status, trust.Refused, "no planner was asked")
			assert.Contains(t, got.Reason, "a file", "the refusal says what it was pointed at")
		})

		t.Run("refuses a request missing an argument the operation needs", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{})
			req := asking(true)
			req.Args = edit.Args{}
			got, err := s.Apply(t.Context(), req)

			assert.NoError(t, err, "a missing argument is correctable")
			assert.Contains(t, got.Reason, string(edit.ArgDoc), "the refusal names the argument")
		})

		t.Run("refuses an argument no operation reads", func(t *testing.T) {
			t.Parallel()
			// A key nobody reads is an operation that silently does
			// something other than what was asked, which is worse than
			// being told the name is wrong.
			_, s := serving(t, planner{})
			req := asking(true)
			req.Args = edit.Args{edit.ArgDoc: "x", edit.ArgKey("documentation"): "y"}
			got, err := s.Apply(t.Context(), req)

			assert.NoError(t, err, "a mistyped key is correctable")
			assert.Contains(t, got.Reason, "documentation", "the refusal names the key nobody reads")
		})

		t.Run("says nothing serves an operation no engine plans", func(t *testing.T) {
			t.Parallel()
			// A capability gap is routed around. A refusal is corrected.
			// A caller told only "no" cannot tell them apart.
			_, s := serving(t, planner{declines: true})
			got, err := s.Apply(t.Context(), asking(true))

			assert.NoError(t, err, "having nothing to ask is not a fault")
			assert.Equal(t, got.Status, trust.Unsupported, "no engine planned it")
			assert.False(t, got.Provenance.SupportsNegativeClaim(),
				"an answer nothing produced proves nothing")
		})

		t.Run("passes back a planner's refusal rather than raising it", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{refuses: "the text closes its own comment"})
			got, err := s.Apply(t.Context(), asking(true))

			assert.NoError(t, err, "a planner that will not serve a request is not a broken planner")
			assert.Equal(t, got.Status, trust.Refused, "the caller is told it was refused")
			assert.Equal(t, got.Reason, "the text closes its own comment",
				"the reason reaches the caller without the error value that carried it")
		})

		t.Run("raises a planner that is broken", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{breaks: true})
			_, err := s.Apply(t.Context(), asking(true))
			assert.HasError(t, err, "a caller can act on a refusal and can do nothing with a fault")
		})

		t.Run("refuses a plan whose evidence is too weak", func(t *testing.T) {
			t.Parallel()
			_, s := serving(t, planner{})
			req := asking(true)
			req.Operation = edit.RenameSymbol
			req.Target = edit.Target{Kind: edit.TargetSymbol, Symbol: "x"}
			req.Args = edit.Args{edit.ArgNewName: "y"}
			got, err := s.Apply(t.Context(), req)

			assert.NoError(t, err, "being told the evidence is too weak is an answer")
			assert.Equal(t, got.Status, trust.Refused, "a parser cannot rename")
			assert.NotEmpty(t, got.Reason, "and the caller is told what it would take")
		})
	})
}

// asking is a well-formed document.symbol request.
func asking(dry bool) edit.Request {
	return edit.Request{
		Operation: edit.DocumentSymbol,
		Scope:     "a.fx",
		Language:  fixture,
		Target: edit.Target{Kind: edit.TargetSpan, Span: source.Span{
			Path: "a.fx", Start: source.Position{Offset: 0},
		}},
		Args:   edit.Args{edit.ArgDoc: "Doc."},
		DryRun: dry,
	}
}

const (
	fixture  = source.Language("fixture")
	original = "one\ntwo\n"
)

// serving builds a service over one file and the engines given.
func serving(t *testing.T, engines ...engine.Engine) (*held, *change.Service) {
	t.Helper()
	catalogue := engine.NewCatalog()
	for _, e := range engines {
		assert.NoError(t, catalogue.Add(e), "a test engine registers")
	}
	files := &held{content: map[source.Path]string{"a.fx": original}}
	return files, change.New(catalogue, router{}, files)
}

// held is a workspace in memory, so the whole pipeline runs without a
// directory and a test reads back what landed.
type held struct {
	content map[source.Path]string
	// refuse is the path this workspace will not take a write for, so a
	// change that fails partway through can be driven.
	refuse source.Path
}

func (h *held) at(p source.Path) string { return h.content[p] }

func (h *held) Read(p source.Path) ([]byte, error) {
	text, there := h.content[p]
	if !there {
		return nil, fs.ErrNotExist
	}
	return []byte(text), nil
}

func (h *held) Write(p source.Path, content []byte) error {
	if p == h.refuse {
		return fmt.Errorf("this workspace will not take a write to %s", p)
	}
	h.content[p] = string(content)
	return nil
}

func (h *held) Remove(p source.Path) error {
	delete(h.content, p)
	return nil
}

// router claims one extension, as a language registry does.
type router struct{}

func (router) LanguageOf(p source.Path) (source.Language, bool) {
	return fixture, strings.HasSuffix(string(p), ".fx")
}

func (router) Languages() []source.Language { return []source.Language{fixture} }

// planner is an engine that writes one comment at the top of a file.
type planner struct {
	declines bool
	refuses  string
	breaks   bool
	// also is a second file the plan changes, so a change that fails
	// partway through has something to have already written.
	also source.Path
}

func (planner) Name() string                        { return "planner" }
func (planner) Language() source.Language           { return fixture }
func (planner) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }
func (planner) Cost(engine.Role) engine.Cost        { return engine.CostParse }

func (p planner) Plan(
	_ context.Context,
	req engine.Request,
	op edit.Operation,
	_ edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	switch {
	case p.declines:
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: not this one", engine.ErrDecline)
	case p.refuses != "":
		return engine.Result[edit.Change]{}, fmt.Errorf("%w: %s", engine.ErrRefuse, p.refuses)
	case p.breaks:
		return engine.Result[edit.Change]{}, fmt.Errorf("the planner is broken")
	}
	written := []edit.TextEdit{{New: "// " + args[edit.ArgDoc] + "\n"}}
	out := []edit.Change{{Kind: edit.ChangeEdit, Path: req.Scope, Edits: written}}
	if p.also != "" {
		out = append(out, edit.Change{Kind: edit.ChangeEdit, Path: p.also, Edits: written})
	}
	return engine.Result[edit.Change]{Items: out, Completeness: trust.ScopeTotal}, nil
}

// checker is an engine that objects to whatever it is told to.
type checker struct {
	name    string
	message string
	// inherited makes it object to the content the change replaces as
	// well, which is the case where the fault was there first.
	inherited bool
}

// clean finds nothing wrong with anything.
func clean() checker { return checker{name: "clean"} }

// faults objects to what the change produces and not to what it
// replaces, which is a change that broke something whole.
func faults(message string) checker { return checker{name: "faults", message: message} }

// always objects to both, which is a file that did not parse before the
// change either.
func always() checker {
	return checker{name: "always", message: "this does not parse", inherited: true}
}

func (c checker) Name() string            { return c.name }
func (checker) Language() source.Language { return fixture }

func (checker) Fidelity(engine.Role) trust.Fidelity { return trust.Syntactic }
func (checker) Cost(engine.Role) engine.Cost        { return engine.CostParse }

func (c checker) Check(
	_ context.Context,
	files map[source.Path][]byte,
) (engine.Result[edit.Finding], error) {
	var out []edit.Finding
	for p, content := range files {
		if c.message == "" {
			continue
		}
		if !c.inherited && !strings.HasPrefix(string(content), "// ") {
			continue
		}
		out = append(out, edit.Finding{Diagnostic: diag.Diagnostic{
			Severity: diag.SeverityError, Code: "parse",
			Message: c.message, Span: source.Span{Path: p}, Source: c.name,
		}})
	}
	return engine.Result[edit.Finding]{Items: out, Completeness: trust.ScopeTotal}, nil
}
