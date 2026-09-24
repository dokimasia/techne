// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

// ApplyInput is the input of the apply.change tool. Handle is the handle of a preview. The
// write path keeps the changes of the preview, so a caller sends the handle and not the
// changes.
type ApplyInput struct {
	Handle string `json:"handle" jsonschema:"the handle that a preview returned"`
}

// Apply returns the tool that writes the change of a preview without planning it again. The
// output names the operation of the preview, and its target and its path are the first file
// that the change wrote. The output of a change that wrote no file has the handle as its
// target and no path.
func Apply(writes Committer) (Tool, error) {
	return New(string(applyChange), applyDescription,
		func(ctx context.Context, in ApplyInput) (Written, error) {
			if in.Handle == "" {
				return declined(applyChange, Scope{}, "", trust.Refused.String(),
					"handle is empty: handle is the handle that a preview returned"), nil
			}

			done, err := writes.Commit(ctx, in.Handle)
			if err != nil {
				return Written{}, err
			}

			target, scope := in.Handle, Scope{}
			if len(done.Changed) > 0 {
				target = string(done.Changed[0])
				scope.Path = target
			}
			out := reported(done.Operation, scope, target, done)
			if out.Operation == "" {
				out.Operation = string(applyChange)
			}
			return out, nil
		})
}

// applyChange is the operation of the apply.change tool. It writes what another operation
// planned, so it has no planner and no row in the catalogue of operations.
const applyChange edit.Operation = "apply.change"

const applyDescription = "PREFER OVER previewing a change and then asking for it again. " +
	"It writes the change of the preview that returned the handle, without planning it again. " +
	"It refuses the change when a file changed after the preview, because byte ranges over " +
	"other bytes describe other code and usually still compile."
