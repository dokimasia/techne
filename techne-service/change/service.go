// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Router maps a path to the languages that a request is about. It is [engine.Router], so
// the read path and the write path route a path by one rule.
type Router = engine.Router

// Files is the directory of the workspace, as the write path reads and changes it. The
// engines read the same directory through an [io/fs.FS].
type Files interface {
	// Read returns the content of the file at p. It returns an error that wraps
	// [fs.ErrNotExist] for a path without a file.
	Read(p source.Path) ([]byte, error)
	// Write replaces the content of the file at p and keeps its mode, or creates the file
	// and the directories that it needs.
	Write(p source.Path, content []byte) error
	// Remove deletes the file at p. A path without a file is not an error.
	Remove(p source.Path) error
	// Move renames the file at from to to, with its mode, and creates the directories of
	// to. It returns an error when a file is at to.
	Move(from, to source.Path) error
	// Lock takes the lock of the workspace and returns the function that releases it. One
	// writer of the workspace takes the lock at a time, in this process and in every other.
	// Lock returns an error when ctx ends before the lock is free.
	Lock(ctx context.Context) (func(), error)
}

// Service applies operations to one workspace. It is safe for concurrent use: a change
// locks its paths in this process from the seal to the write, and [Files.Lock] serialises
// the writes of every process.
type Service struct {
	catalog *engine.Catalog
	router  Router
	files   Files
	policy  edit.Policy
	locks   *locks
	held    *held
}

// New returns a service over the engines of c, the languages of r and the workspace f.
func New(c *engine.Catalog, r Router, f Files) *Service {
	return &Service{catalog: c, router: r, files: f, locks: newLocks(), held: newHeld()}
}

// unchecked is the caveat of a change whose language has no checker.
var unchecked = trust.Caveat{
	Code: trust.CaveatUnsupported,
	Note: "no engine checks this language, so the change was not verified",
}

// Apply plans the operation of req and takes the plan through the write path, or previews
// it for a dry run. A refusal returns an outcome with the status [trust.Refused] and the
// reason, because the caller can change the request. An error means that the workspace
// could not be read or written.
func (s *Service) Apply(ctx context.Context, req edit.Request) (edit.Outcome, error) {
	spec, declared := edit.SpecFor(req.Operation)
	if !declared {
		return refused(req.Operation, fmt.Sprintf("no operation is named %q", req.Operation)), nil
	}
	if err := accepts(spec, req.Target, req.Args); err != nil {
		return refused(req.Operation, err.Error()), nil
	}

	plan, declined, err := s.plan(ctx, req, spec)
	switch {
	case errors.Is(err, engine.ErrRefuse):
		return refused(req.Operation, trimmed(err)), nil
	case err != nil:
		return edit.Outcome{}, err
	case plan.Provenance.Engine == "" && len(declined) > 0:
		return unsupported(req.Operation, declined.Reason()), nil
	case plan.Provenance.Engine == "":
		return unsupported(req.Operation, fmt.Sprintf("no engine plans %s for %q", req.Operation, req.Scope)), nil
	case len(plan.Changes) == 0:
		return refusedBy(plan, plan.Provenance.Engine+" planned no change"), nil
	}

	release := s.locks.hold(plan.Paths())
	defer release()
	sealed, err := s.seal(&plan)
	if err != nil {
		return edit.Outcome{}, err
	}
	return s.run(ctx, req, spec, plan, sealed)
}

// Commit applies the plan that a preview kept under handle, once. It takes the steps of the
// preview again with the request of the preview, over the files as they are now, and
// refuses a plan whose files changed since the preview.
func (s *Service) Commit(ctx context.Context, handle string) (edit.Outcome, error) {
	preview, found := s.held.take(handle)
	if !found {
		return refused("", "no preview is held under that handle: preview again to get one"), nil
	}
	spec, _ := edit.SpecFor(preview.plan.Operation)

	release := s.locks.hold(preview.plan.Paths())
	defer release()
	sealed, drifted, err := s.unchanged(preview.plan)
	switch {
	case err != nil:
		return edit.Outcome{}, err
	case drifted != "":
		return refusedBy(preview.plan, drifted), nil
	}
	req := preview.request
	req.DryRun = false
	return s.run(ctx, req, spec, preview.plan, sealed)
}

// run takes a sealed plan through the steps that Apply and Commit share: the paths that it
// creates, [edit.Policy.Admit], the projection, the gate, and the write or, for a dry run,
// a handle. A change gets the status [trust.Degraded] when no engine checks its language.
func (s *Service) run(
	ctx context.Context,
	req edit.Request,
	spec edit.Spec,
	plan edit.Plan,
	sealed map[source.Path][]byte,
) (edit.Outcome, error) {
	switch why, err := s.occupied(plan); {
	case err != nil:
		return edit.Outcome{}, err
	case why != "":
		return refusedBy(plan, why), nil
	}
	if refusal := s.policy.Admit(spec, plan); refusal != nil {
		return refusedBy(plan, reason(refusal)), nil
	}
	projected, err := project(plan, sealed)
	if err != nil {
		return refusedBy(plan, reason(err)), nil
	}

	gated, err := s.gate(ctx, req, plan, sealed, projected)
	if err != nil {
		return edit.Outcome{}, err
	}
	rewrites := preview(plan, sealed)
	if gated.worse {
		out := refusedBy(plan, fmt.Sprintf("the change stops %s %s, so it was not written",
			where(gated.found), judging(gated.by)))
		out.Diagnostics, out.Rewrites, out.Gate = gated.found, rewrites, &gated.by
		return out, nil
	}

	out := edit.Outcome{
		Operation:  plan.Operation,
		Status:     trust.OK,
		Changes:    plan.Changes,
		Rewrites:   rewrites,
		Provenance: plan.Provenance,
	}
	if gated.checked {
		out.Gate = &gated.by
	} else {
		out.Status = trust.Degraded
		out.Provenance.Caveats = append(slices.Clone(out.Provenance.Caveats), unchecked)
	}
	if req.DryRun {
		handle, failed := s.held.keep(req, plan)
		if failed != nil {
			return edit.Outcome{}, failed
		}
		out.Handle = handle
		return out, nil
	}

	written, why, err := s.write(ctx, plan, sealed, projected)
	switch {
	case err != nil:
		return edit.Outcome{}, err
	case why != "":
		stopped := refusedBy(plan, why)
		stopped.Rewrites = rewrites
		return stopped, nil
	}
	out.Applied, out.Changed = true, written
	return out, nil
}

// plan asks the languages of the scope of req for the changes of the operation, and returns
// the plan of the first language whose answer is not skipped. A declaration belongs to one
// language, so a second answer would be about another declaration. It returns an empty plan
// and the reasons of the engines that declined when no language returns a plan.
func (s *Service) plan(
	ctx context.Context,
	req edit.Request,
	spec edit.Spec,
) (edit.Plan, engine.Declined, error) {
	asking := engine.Request{
		Scope:     req.Scope,
		Language:  req.Language,
		Preferred: spec.MinFidelity,
		Tests:     true,
	}
	answered, ok, declined, err := engine.AskAny(ctx, s.catalog, s.router, asking, engine.RolePlan,
		func(e engine.Engine) (engine.Result[edit.Change], error) {
			return e.(engine.Planner).Plan(ctx, asking, req.Operation, req.Target, req.Args)
		})
	if err != nil || !ok {
		return edit.Plan{}, declined, err
	}
	return edit.Plan{
		Operation:  req.Operation,
		Changes:    answered.Items,
		Provenance: answered.Provenance,
	}, nil, nil
}

// seal reads the content of each path that the plan reads, and adds its digest to the
// preconditions of the plan. The projection is built from the same bytes, and a write that
// stops writes them back.
func (s *Service) seal(plan *edit.Plan) (map[source.Path][]byte, error) {
	out := map[source.Path][]byte{}
	for _, p := range plan.Reads() {
		content, err := s.files.Read(p)
		if err != nil {
			return nil, fmt.Errorf("change: read %s: %w", p, err)
		}
		out[p] = content
		plan.Preconditions = append(plan.Preconditions, edit.Precondition{
			Path: p, Digest: sha256.Sum256(content),
		})
	}
	return out, nil
}

// unchanged reads the files of the preconditions of the plan of a preview. It returns a
// refusal about the first file that changed since the preview.
func (s *Service) unchanged(plan edit.Plan) (map[source.Path][]byte, string, error) {
	out := map[source.Path][]byte{}
	for _, pinned := range plan.Preconditions {
		content, why, err := s.current(plan, pinned.Path)
		switch {
		case err != nil:
			return nil, "", err
		case why != "":
			return nil, why + ", so the plan describes code that the file does not contain: preview again", nil
		}
		out[pinned.Path] = content
	}
	return out, "", nil
}

// current reads the file at p, and returns a refusal that names p when the file is gone or
// its digest differs from the precondition of p in the plan.
func (s *Service) current(plan edit.Plan, p source.Path) ([]byte, string, error) {
	content, err := s.files.Read(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Sprintf("%s was deleted after the plan read it", p), nil
	case err != nil:
		return nil, "", fmt.Errorf("change: read %s: %w", p, err)
	}
	for _, pinned := range plan.Preconditions {
		if pinned.Path == p && sha256.Sum256(content) != pinned.Digest {
			return nil, fmt.Sprintf("%s changed after the plan read it", p), nil
		}
	}
	return content, "", nil
}

// occupied returns a refusal about the first path that the plan creates or moves a file to
// while a file is at it.
func (s *Service) occupied(plan edit.Plan) (string, error) {
	for _, p := range arrivals(plan) {
		if why, err := s.vacant(p); why != "" || err != nil {
			return why, err
		}
	}
	return "", nil
}

// vacant returns a refusal that names p when a file is at p.
func (s *Service) vacant(p source.Path) (string, error) {
	_, err := s.files.Read(p)
	switch {
	case err == nil:
		return fmt.Sprintf("a file is at %s, and the change does not replace a file", p), nil
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	}
	return "", fmt.Errorf("change: read %s: %w", p, err)
}

// arrivals returns the paths that the plan creates or moves a file to, sorted.
func arrivals(plan edit.Plan) []source.Path {
	var out []source.Path
	for _, c := range plan.Changes {
		switch c.Kind {
		case edit.ChangeCreate:
			out = append(out, c.Path)
		case edit.ChangeMove:
			out = append(out, c.To)
		case edit.ChangeUnset, edit.ChangeEdit, edit.ChangeDelete:
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// accepts returns an error that states why a request does not match the spec of its
// operation, or nil. It checks the kind of the target, the required arguments, and that
// each argument is one that the operation reads.
func accepts(spec edit.Spec, target edit.Target, args edit.Args) error {
	if !slices.Contains(spec.Accepts, target.Kind) {
		return fmt.Errorf("%s cannot be pointed at %s", spec.Operation, named(target.Kind))
	}
	for _, key := range spec.Required {
		if args[key] == "" {
			return fmt.Errorf("%s needs %q", spec.Operation, key)
		}
	}
	for key := range args {
		if !slices.Contains(spec.Required, key) && !slices.Contains(spec.Optional, key) {
			return fmt.Errorf("%s takes no argument named %q", spec.Operation, key)
		}
	}
	return nil
}

// named returns the words for a target kind in a refusal.
func named(k edit.TargetKind) string {
	switch k {
	case edit.TargetSymbol:
		return "a symbol"
	case edit.TargetFile:
		return "a file"
	case edit.TargetSpan:
		return "a span"
	default:
		return "nothing"
	}
}

// trimmed returns the reason of a refusal of a planner without the text of
// [engine.ErrRefuse].
func trimmed(err error) string {
	if _, why, cut := strings.Cut(err.Error(), engine.ErrRefuse.Error()+": "); cut {
		return why
	}
	return reason(err)
}

// reason returns the text of err without the name of the package of the write path that
// raised it, edit or change. A refusal is read by the caller of a tool, which does not know
// those packages.
func reason(err error) string {
	out := err.Error()
	for _, prefix := range []string{"edit: ", "change: "} {
		out = strings.TrimPrefix(out, prefix)
	}
	return out
}

// refused returns the outcome of a request that the write path refuses before it has a
// plan.
func refused(op edit.Operation, why string) edit.Outcome {
	return edit.Outcome{Operation: op, Status: trust.Refused, Reason: why}
}

// refusedBy returns the outcome of a plan that the write path refuses, with the changes and
// the evidence of the plan.
func refusedBy(plan edit.Plan, why string) edit.Outcome {
	return edit.Outcome{
		Operation:  plan.Operation,
		Status:     trust.Refused,
		Changes:    plan.Changes,
		Provenance: plan.Provenance,
		Reason:     why,
	}
}

// unsupported returns the outcome of an operation without a planner for the scope, with the
// evidence of [engine.Unsupported].
func unsupported(op edit.Operation, why string) edit.Outcome {
	nothing := engine.Unsupported[edit.Change](why)
	return edit.Outcome{
		Operation:  op,
		Status:     nothing.Status,
		Reason:     why,
		Provenance: nothing.Provenance,
	}
}
