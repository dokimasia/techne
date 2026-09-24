// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"context"
	"errors"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// ErrDecline reports that an engine serves the role but cannot serve this
// request. [Ask] records the reason in [Declined] and tries the next engine.
// Wrap it with the reason, as in fmt.Errorf("%w: reason", ErrDecline).
var ErrDecline = errors.New("engine: decline")

// ErrRefuse reports that an engine understands a request and will not serve
// it, for a reason the caller can act on. [Ask] returns it without trying
// another engine, because the request must change before any engine can
// serve it. The write path reports it as a refusal with the reason.
var ErrRefuse = errors.New("engine: refuse")

// Engine is the interface every adapter implements. An adapter also
// implements the port of each role it serves.
type Engine interface {
	// Name identifies the engine in a provenance and a capability report.
	Name() string

	// Language returns the language the engine serves.
	Language() source.Language

	// Fidelity returns the tier the engine declares for a role. The value is
	// fixed for the engine's lifetime. [trust.None] declines the role. The
	// catalogue calls Fidelity only for roles whose port the engine
	// implements.
	Fidelity(Role) trust.Fidelity

	// Cost returns what serving a role takes.
	Cost(Role) Cost
}

// Available is implemented by an engine that depends on something outside
// the process, such as a language server binary. The catalogue does not
// select an engine whose Available returns an error, and
// [Catalog.Capabilities] reports the error. An engine that does not
// implement Available is always available.
type Available interface {
	Available(ctx context.Context) error
}
