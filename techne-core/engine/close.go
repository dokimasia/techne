// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"context"
	"errors"
)

// Closer is implemented by an engine that keeps a subprocess or a connection
// open across calls. [Catalog.Close] closes it.
type Closer interface {
	Close(ctx context.Context) error
}

// Close closes every registered engine that implements [Closer]. It calls
// every engine even when one fails, and returns the failures joined with
// [errors.Join]. The catalogue is not usable after Close, and Close is not
// safe for concurrent use with other methods.
func (c *Catalog) Close(ctx context.Context) error {
	var failed []error
	for _, e := range c.engines {
		closer, ok := e.(Closer)
		if !ok {
			continue
		}
		if err := closer.Close(ctx); err != nil {
			failed = append(failed, err)
		}
	}
	return errors.Join(failed...)
}
