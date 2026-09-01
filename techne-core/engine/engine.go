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
