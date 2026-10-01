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
	Handle string `json:"handle" jsonschema:"the handle of a preview"`
}

// Apply returns the tool that writes the change of a preview without planning it again. The
// output names the operation, the target and the scope of the preview. When the write path does
// not keep a preview under the handle, the output has the handle as its target and an empty scope.
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
			if asked := done.Request; asked.Operation != "" {
				target, scope = asked.Subject, writing(asked.Scope, string(asked.Language))
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

const applyDescription = "PREFER OVER asking for a previewed change again. " +
	"It writes the change of the preview that returned the handle, and refuses it when a file " +
	"changed after the preview."
