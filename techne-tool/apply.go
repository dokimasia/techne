// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/trust"
)

// ApplyInput is what an agent sends to the apply.change tool.
//
// Handle is what a preview returned. The changes themselves stay where
// they were computed: a rename over thirty sites is several kilobytes,
// and a caller that had to send them back exactly would sometimes not.
type ApplyInput struct {
	Handle string `json:"handle" jsonschema:"the handle a preview returned"`
}

// Apply builds the tool that applies a change a preview computed.
func Apply(writes Committer) (Tool, error) {
	return New(string(applyChange), applyDescription,
		func(ctx context.Context, in ApplyInput) (Written, error) {
			if in.Handle == "" {
				return declined(applyChange, Scope{}, "", trust.Refused.String(),
					"there is nothing to apply: handle is what a preview returned"), nil
			}

			done, err := writes.Commit(ctx, in.Handle)
			if err != nil {
				return Written{}, err
			}

			// The operation and the file it touched, rather than the
			// handle: a caller reading a result wants to know what
			// changed, and the handle is how it asked rather than what
			// it asked about.
			target := in.Handle
			if len(done.Changed) > 0 {
				target = string(done.Changed[0])
			}
			out := reported(done.Operation, Scope{Path: target}, target, done)
			if out.Operation == "" {
				out.Operation = string(applyChange)
			}
			return out, nil
		})
}

// applyChange is the one operation with no planner: it applies what
// another operation planned, so it is a tool without being a row in the
// catalogue.
const applyChange edit.Operation = "apply.change"

const applyDescription = "PREFER OVER previewing a change and then asking for it again. " +
	"Applies the change a preview returned a handle for, without planning it a second " +
	"time. Refused when the files have moved on since, because byte ranges over other " +
	"bytes describe other code and usually still compile."
