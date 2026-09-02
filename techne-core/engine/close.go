// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"context"
	"errors"
)

// Closer is implemented by an engine holding something that outlives a
// call: a subprocess, a connection, a handle on a file.
//
// A composition root closes the catalogue when it is done, which closes
// these. An engine that holds nothing does not implement it, so no
// adapter carries a method that returns nil.
type Closer interface {
	Close(ctx context.Context) error
}

// Close stops every engine holding something that outlives a call.
//
// All of them are asked even when one fails, and every failure is
// reported. Stopping at the first would leave a process running for each
// engine after it, and a tool that leaks one per language per run is
// unusable.
//
// The catalogue is not usable afterwards. Nothing checks: closing is
// what a composition root does on the way out, and a check would be a
// lock on every call to serve a case that cannot arise.
func (c *Catalog) Close(ctx context.Context) error {
	var failed []error
	for _, e := range c.engines {
		held, closes := e.(Closer)
		if !closes {
			continue
		}
		if err := held.Close(ctx); err != nil {
			failed = append(failed, err)
		}
	}
	return errors.Join(failed...)
}
