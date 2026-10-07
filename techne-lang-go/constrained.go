// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang

import (
	"context"
	"strings"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/go/checker"
	"go.dokimi.dev/techne/lang/lsp"
)

// Constrained returns the server engine served with the evidence of the build constraints of
// the workspace, which files lists. gopls reads the files of the default build of the go
// command, as the type checker does. It reads a file of another port, such as a file of another
// operating system, in a build of that port. The engine adds the caveats of files to the
// relations, the plans, the verify and the check of the server:
//
//   - a relation gets the caveat of [checker.Engine.Unread], and is partial with it
//   - the plan of a rename gets the changes and the caveat of [checker.Engine.Renamed], and is
//     partial with the caveat
//   - a verify gets the caveat of [checker.Engine.Unverified], and is partial with it
//   - a check gets the caveat of [checker.Engine.Unchecked], and keeps its coverage, because it
//     checks every file of the change
//
// Every other method is the method of served.
func Constrained(served *lsp.Engine, files *checker.Engine) engine.Engine {
	return constrained{Engine: served, files: files}
}

// constrained is the engine of [Constrained].
type constrained struct {
	*lsp.Engine
	// files is the type checker, which lists the Go files that the build constraints exclude.
	files *checker.Engine
}

// Relate returns the relations that the server returns, with the caveat of
// [checker.Engine.Unread]. It returns an error and a skipped result of the server as they are.
// The type checker returns no caveat when it cannot list the files, so the answer of the server
// then stays as it is.
func (c constrained) Relate(
	ctx context.Context,
	req engine.Request,
	of sema.ID,
	kind sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	got, err := c.Engine.Relate(ctx, req, of, kind)
	if err != nil || got.Skipped {
		return got, err
	}
	left, _ := c.files.Unread(ctx, req, of, kind)
	return partly(got, left), nil
}

// Plan returns the plan of the server for op. The plan of a rename gets the changes of
// [checker.Engine.Renamed], which rename the uses of the declaration in the files that the build
// constraints exclude from the build of the server. It gets the caveat of the excluded files
// whose uses stay as they are by the rules of [constrained.Relate], and is partial with it. The
// write path refuses such a plan.
func (c constrained) Plan(
	ctx context.Context,
	req engine.Request,
	op edit.Operation,
	target edit.Target,
	args edit.Args,
) (engine.Result[edit.Change], error) {
	got, err := c.Engine.Plan(ctx, req, op, target, args)
	if err != nil || got.Skipped || op != edit.RenameSymbol {
		return got, err
	}
	renamed, left, _ := c.files.Renamed(ctx, target, strings.TrimSpace(args[edit.ArgNewName]))
	got.Items = append(got.Items, renamed...)
	return partly(got, left), nil
}

// Verify returns the issues that the server reports for the scope of req. The answer gets the
// caveat of [checker.Engine.Unverified] by the rules of [constrained.Relate].
func (c constrained) Verify(
	ctx context.Context,
	req engine.Request,
	suites []string,
) (engine.Result[edit.Finding], error) {
	got, err := c.Engine.Verify(ctx, req, suites)
	if err != nil || got.Skipped {
		return got, err
	}
	left, _ := c.files.Unverified(ctx, req)
	return partly(got, left), nil
}

// Check returns the errors that the server reports for the change to files, with the caveat of
// [checker.Engine.Unchecked], by the rules of [constrained.Relate]. The check keeps the coverage
// of the server.
func (c constrained) Check(ctx context.Context, files map[source.Path][]byte) (engine.Result[edit.Finding], error) {
	got, err := c.Engine.Check(ctx, files)
	if err != nil {
		return got, err
	}
	left, _ := c.files.Unchecked(ctx, files)
	got.Caveats = append(got.Caveats, left...)
	return got, nil
}

// partly returns r with the caveats of left, and with partial coverage when left has one.
func partly[T any](r engine.Result[T], left []trust.Caveat) engine.Result[T] {
	if len(left) == 0 {
		return r
	}
	r.Completeness, r.Caveats = trust.ScopePartial, append(r.Caveats, left...)
	return r
}
