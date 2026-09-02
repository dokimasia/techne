// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Router reports which languages exist and which one claims a path.
//
// It is the port the read path takes, re-exported rather than
// redeclared: a read and a write that disagreed about which language
// owns a file would plan a change with one engine and gate it with
// another.
type Router = engine.Router

// Files is the workspace as something that can be written to.
//
// An engine reads through an [io/fs.FS], which cannot write. This is the
// other half, and it is a port so a test can drive the whole pipeline
// without a directory and so the one implementation that touches disk is
// in the composition root.
//
// A move is a write and a remove rather than a rename, so an
// implementation has one way to put bytes somewhere and one way to take
// a file away.
type Files interface {
	Read(p source.Path) ([]byte, error)
	Write(p source.Path, content []byte) error
	Remove(p source.Path) error
}

// Service applies operations to one workspace.
//
// It is safe for concurrent use: the per-path locks are what serialise
// two callers changing the same files, and nothing else here is mutable
// once a composition root has finished.
type Service struct {
	catalog *engine.Catalog
	router  Router
	files   Files
	policy  edit.Policy
	locks   *locks
	held    *held
}

// New returns a service over a catalogue, a router and a workspace.
func New(c *engine.Catalog, r Router, f Files) *Service {
	return &Service{catalog: c, router: r, files: f, locks: newLocks(), held: newHeld()}
}

// Apply runs one operation through the write path.
//
// A refusal is a result rather than an error, because a caller can act
// on being told the evidence was too weak or the gate failed, and can do
// nothing with a fault. An error means the workspace could not be read
// or written, which is not something the caller got wrong.
func (s *Service) Apply(ctx context.Context, req edit.Request) (edit.Outcome, error) {
	spec, declared := edit.SpecFor(req.Operation)
	if !declared {
		return refused(req.Operation, fmt.Sprintf(
			"no operation is named %q", req.Operation)), nil
	}
	if err := accepts(spec, req.Target, req.Args); err != nil {
		return refused(req.Operation, err.Error()), nil
	}

	plan, err := s.plan(ctx, req, spec)
	switch {
	case errors.Is(err, engine.ErrRefuse):
		// The planner understood the request and will not serve it, so
		// the reason is the answer rather than a fault.
		return refused(req.Operation, trimmed(err)), nil
	case err != nil:
		return edit.Outcome{}, err
	case len(plan.Changes) == 0 && plan.Provenance.Engine == "":
		return unsupported(req.Operation, fmt.Sprintf(
			"no engine plans %s for %q", req.Operation, req.Scope)), nil
	}

	// From here to the write the plan's paths are held, so nothing in
	// this process can rewrite a file between the content being pinned
	// and the same content being written back changed.
	release := s.locks.hold(plan.Paths())
	defer release()

	sealed, err := s.seal(&plan)
	if err != nil {
		return edit.Outcome{}, err
	}
	if refusal := s.policy.Admit(spec, plan); refusal != nil {
		return refusedBy(plan, refusal.Error()), nil
	}

	projected, err := project(plan, sealed)
	if err != nil {
		return refusedBy(plan, err.Error()), nil
	}

	gated, err := s.gate(ctx, req, sealed, projected)
	if err != nil {
		return edit.Outcome{}, err
	}
	if gated.worse {
		out := refusedBy(plan, fmt.Sprintf(
			"the change stops %s parsing, so it was not written", where(gated.found)))
		out.Diagnostics, out.Rewrites = gated.found, preview(plan, sealed)
		return out, nil
	}

	out := edit.Outcome{
		Operation:  plan.Operation,
		Status:     trust.OK,
		Changes:    plan.Changes,
		Rewrites:   preview(plan, sealed),
		Provenance: plan.Provenance,
	}
	if !gated.checked {
		// Nothing judged the result, so the caller is holding a change
		// that was admitted rather than one that was verified.
		out.Status = trust.Degraded
		out.Provenance.Caveats = append(out.Provenance.Caveats, trust.Caveat{
			Code: trust.CaveatUnsupported,
			Note: "nothing checks this language, so the change was not verified",
		})
	}
	if req.DryRun {
		// The plan is kept so applying it costs no second planning run,
		// and the preconditions sealed above are what say the files have
		// not moved on when the caller comes back.
		handle, err := s.held.keep(plan)
		if err != nil {
			return edit.Outcome{}, err
		}
		out.Handle = handle
		return out, nil
	}

	if written := s.write(plan, sealed, projected); written != nil {
		return edit.Outcome{}, written
	}
	out.Applied, out.Changed = true, plan.Paths()
	return out, nil
}

// Commit applies a plan a preview computed and kept.
//
// It runs the same checks the preview ran, over the files as they are
// now rather than as they were: a plan whose file moved on describes
// code that is no longer there, and byte ranges over other bytes usually
// still compile.
func (s *Service) Commit(ctx context.Context, handle string) (edit.Outcome, error) {
	plan, kept := s.held.take(handle)
	if !kept {
		return refused("", "no preview is held under that handle: preview again to get one"), nil
	}
	spec, declared := edit.SpecFor(plan.Operation)
	if !declared {
		return refused(plan.Operation, fmt.Sprintf(
			"no operation is named %q", plan.Operation)), nil
	}

	release := s.locks.hold(plan.Paths())
	defer release()

	sealed, drifted, err := s.unchanged(plan)
	switch {
	case err != nil:
		return edit.Outcome{}, err
	case drifted != "":
		return refusedBy(plan, drifted), nil
	}
	if refusal := s.policy.Admit(spec, plan); refusal != nil {
		return refusedBy(plan, refusal.Error()), nil
	}

	projected, err := project(plan, sealed)
	if err != nil {
		return refusedBy(plan, err.Error()), nil
	}
	gated, err := s.gate(ctx, edit.Request{Scope: plan.Paths()[0]}, sealed, projected)
	if err != nil {
		return edit.Outcome{}, err
	}
	if gated.worse {
		out := refusedBy(plan, fmt.Sprintf(
			"the change stops %s parsing, so it was not written", where(gated.found)))
		out.Diagnostics, out.Rewrites = gated.found, preview(plan, sealed)
		return out, nil
	}

	if written := s.write(plan, sealed, projected); written != nil {
		return edit.Outcome{}, written
	}
	return edit.Outcome{
		Operation:  plan.Operation,
		Status:     trust.OK,
		Applied:    true,
		Changed:    plan.Paths(),
		Changes:    plan.Changes,
		Rewrites:   preview(plan, sealed),
		Provenance: plan.Provenance,
	}, nil
}

// unchanged reads the files a plan depends on and reports which of them
// moved on since it was computed.
func (s *Service) unchanged(plan edit.Plan) (map[source.Path][]byte, string, error) {
	sealed := map[source.Path][]byte{}
	for _, pinned := range plan.Preconditions {
		content, err := s.files.Read(pinned.Path)
		if err != nil {
			return nil, "", fmt.Errorf("change: read %s: %w", pinned.Path, err)
		}
		if sha256.Sum256(content) != pinned.Digest {
			return nil, fmt.Sprintf(
				"%s changed since it was previewed, so the change describes code that "+
					"is no longer there: preview again", pinned.Path), nil
		}
		sealed[pinned.Path] = content
	}
	return sealed, "", nil
}

// plan asks the languages a request could be about for the edits it
// would need.
//
// The first that answers wins. A symbol is declared in one language, so
// a second answer would be about a second symbol.
func (s *Service) plan(ctx context.Context, req edit.Request, spec edit.Spec) (edit.Plan, error) {
	asking := engine.Request{
		Scope:     req.Scope,
		Language:  req.Language,
		Preferred: spec.MinFidelity,
		Tests:     true,
	}
	answered, ok, err := engine.AskAny(ctx, s.catalog, s.router, asking, engine.RolePlan,
		func(e engine.Engine) (engine.Result[edit.Change], error) {
			return e.(engine.Planner).Plan(ctx, asking, req.Operation, req.Target, req.Args)
		})
	if err != nil || !ok {
		return edit.Plan{}, err
	}
	return edit.Plan{
		Operation:  req.Operation,
		Changes:    answered.Items,
		Provenance: answered.Provenance,
	}, nil
}

// seal pins the content the plan depends on and hands it back.
//
// The same read serves three purposes: the digest the policy checks, the
// bytes the projection is built from, and the snapshot a rollback
// restores. Reading three times would let the file differ between them.
func (s *Service) seal(plan *edit.Plan) (map[source.Path][]byte, error) {
	held := map[source.Path][]byte{}
	for _, p := range plan.Reads() {
		content, err := s.files.Read(p)
		if err != nil {
			return nil, fmt.Errorf("change: read %s: %w", p, err)
		}
		held[p] = content
		plan.Preconditions = append(plan.Preconditions, edit.Precondition{
			Path: p, Digest: sha256.Sum256(content),
		})
	}
	return held, nil
}

// verdict is what the gate made of a projection.
type verdict struct {
	// checked reports that something judged it. Nothing judging it is
	// not the same as nothing being wrong.
	checked bool
	// worse reports that the change broke something that was whole.
	worse bool
	found []edit.Finding
}

// gate judges the projection, and judges what it replaces, so a change
// is refused for what it broke rather than for what it inherited.
//
// A file that did not parse before the change does not fail because of
// it, and refusing on what was already there would make the code that
// most wants fixing the code nothing may touch. The comparison is by
// count: a change that trades one fault for another passes, which is the
// price of not matching messages whose line numbers the change moved.
func (s *Service) gate(
	ctx context.Context,
	req edit.Request,
	sealed, projected map[source.Path][]byte,
) (verdict, error) {
	after, checked, err := s.check(ctx, req, projected)
	if err != nil || !checked {
		return verdict{checked: checked}, err
	}
	if len(after) == 0 {
		return verdict{checked: true}, nil
	}

	// Only the files the change touches, and only as they were. A fault
	// anywhere else is nothing this change did.
	was := map[source.Path][]byte{}
	for p := range projected {
		if content, held := sealed[p]; held {
			was[p] = content
		}
	}
	before, _, err := s.check(ctx, req, was)
	if err != nil {
		return verdict{}, err
	}
	return verdict{checked: true, worse: len(after) > len(before), found: after}, nil
}

// check asks whatever serves this language what is wrong with content,
// and reports whether anything answered.
func (s *Service) check(
	ctx context.Context,
	req edit.Request,
	files map[source.Path][]byte,
) ([]edit.Finding, bool, error) {
	if len(files) == 0 {
		return nil, true, nil
	}
	asking := engine.Request{Scope: req.Scope, Language: req.Language}
	answered, ok, err := engine.AskAny(ctx, s.catalog, s.router, asking, engine.RoleCheck,
		func(e engine.Engine) (engine.Result[edit.Finding], error) {
			return e.(engine.Checker).Check(ctx, files)
		})
	if err != nil || !ok {
		return nil, false, err
	}
	return errorsIn(answered.Items), true, nil
}

// errorsIn keeps the findings that stop a change being written.
// A warning about the code is not a reason to refuse a comment.
func errorsIn(found []edit.Finding) []edit.Finding {
	var out []edit.Finding
	for _, one := range found {
		if one.Diagnostic.Severity >= diag.SeverityError {
			out = append(out, one)
		}
	}
	return out
}

// trimmed reads a refusal's reason without the sentinel it was wrapped
// in. A caller is told what to change, not which error value said so.
func trimmed(err error) string {
	out := err.Error()
	if _, why, cut := strings.Cut(out, engine.ErrRefuse.Error()+": "); cut {
		return why
	}
	return out
}

// where names the files a set of diagnostics is about, for a refusal a
// caller reads.
func where(found []edit.Finding) string {
	var named []string
	for _, one := range found {
		if !slices.Contains(named, string(one.Diagnostic.Span.Path)) {
			named = append(named, string(one.Diagnostic.Span.Path))
		}
	}
	if len(named) == 0 {
		return "the file"
	}
	return strings.Join(named, ", ")
}

// accepts reports why a request does not match what the operation
// declares, or nil.
//
// It runs before any language is consulted, so a malformed request
// produces one refusal whatever would have served it.
func accepts(spec edit.Spec, target edit.Target, args edit.Args) error {
	if !slices.Contains(spec.Accepts, target.Kind) {
		return fmt.Errorf("%s cannot be pointed at %s", spec.Operation, named(target.Kind))
	}
	for _, key := range spec.Required {
		if args[key] == "" {
			return fmt.Errorf("%s needs %q", spec.Operation, key)
		}
	}
	// A key nobody reads is an operation that silently does something
	// other than what was asked, which is worse than being told the name
	// is wrong.
	for key := range args {
		if !slices.Contains(spec.Required, key) && !slices.Contains(spec.Optional, key) {
			return fmt.Errorf("%s takes no argument named %q", spec.Operation, key)
		}
	}
	return nil
}

// named is the wire form of a target kind, for a refusal a caller reads.
func named(k edit.TargetKind) string {
	switch k {
	case edit.TargetSymbol:
		return "a symbol"
	case edit.TargetFile:
		return "a file"
	case edit.TargetSpan:
		return "a span"
	case edit.TargetUnset:
		return "nothing"
	default:
		return "nothing"
	}
}

// refused is the result when the request itself is wrong.
func refused(op edit.Operation, why string) edit.Outcome {
	return edit.Outcome{Operation: op, Status: trust.Refused, Reason: why}
}

// refusedBy is the result when a plan existed and was not admitted or
// did not survive the gate. It carries the plan's evidence, because why
// the evidence was not enough is the answer.
func refusedBy(plan edit.Plan, why string) edit.Outcome {
	return edit.Outcome{
		Operation:  plan.Operation,
		Status:     trust.Refused,
		Changes:    plan.Changes,
		Provenance: plan.Provenance,
		Reason:     why,
	}
}

// unsupported is the result when nothing serves the operation here.
//
// It is separate from a refusal: a caller routes around a capability gap
// and corrects a malformed request. The evidence comes from where a
// read's does, so both say the same thing about an answer nothing
// produced.
func unsupported(op edit.Operation, why string) edit.Outcome {
	nothing := engine.Unsupported[edit.Change](why)
	return edit.Outcome{
		Operation:  op,
		Status:     nothing.Status,
		Reason:     why,
		Provenance: nothing.Provenance,
	}
}
