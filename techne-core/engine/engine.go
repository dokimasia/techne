// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"context"
	"errors"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// ErrDecline reports that an engine cannot serve one particular request,
// though it serves the role in general.
//
// A service moves on to the next engine. Any other error stops selection
// instead, because substituting a weaker answer for a broken engine
// hides the breakage.
var ErrDecline = errors.New("engine: decline")

// ErrRefuse reports that an engine will not serve a request it
// understands, for a reason the caller can act on.
//
// A service stops looking and passes the reason back. It is separate
// from [ErrDecline] because the two lead somewhere different: a decline
// says another engine may do better, and a refusal says no engine will
// until the request changes. Both are separate from an error, which says
// something is broken and the caller did nothing wrong.
var ErrRefuse = errors.New("engine: refuse")

// Engine is what every adapter implements, on top of whichever roles it
// serves.
type Engine interface {
	// Name identifies the adapter in a provenance and a capability
	// report. It names the engine, not the language.
	Name() string

	// Language is the one language this engine answers about.
	Language() source.Language

	// Fidelity is the tier this engine claims for a role, fixed for the
	// engine's lifetime. A role it does not serve may return any value:
	// a service asks only after asserting the port.
	Fidelity(Role) trust.Fidelity

	// Cost is what answering that role takes.
	Cost(Role) Cost
}

// Available is implemented by an engine that depends on something
// outside the process, such as a language server on the path.
//
// A service skips an engine whose Available returns an error and reports
// the reason, rather than advertising a capability that cannot run. An
// engine that does not implement this is always available.
type Available interface {
	Available(ctx context.Context) error
}
