// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust

import "go.dokimi.dev/techne/core/internal/wire"

// Status is the outcome of a request. OK, Degraded, and Partial answers have
// a payload. Unsupported and Refused answers do not. The zero value is Unset,
// so a status nobody assigned never reads as success.
type Status uint8

const (
	// Unset means no status was assigned. Services never return it.
	Unset Status = iota
	// OK means an engine answered at its declared tier.
	OK
	// Degraded means an engine answered below the tier the caller asked for.
	Degraded
	// Partial means part of the scope was not covered. The caveats name it.
	Partial
	// Unsupported means nothing serves this language and role.
	Unsupported
	// Refused means the request was declined by policy or named a target
	// that does not exist.
	Refused
)

var statusWords = wire.New(Unset, map[Status]string{
	Unset:       "unset",
	OK:          "ok",
	Degraded:    "degraded",
	Partial:     "partial",
	Unsupported: "unsupported",
	Refused:     "refused",
})

// String returns the wire string of s, or "unset" if s is not a declared
// Status.
func (s Status) String() string { return statusWords.String(s) }

// Answered reports whether an engine produced a payload. An empty item list
// from an answer that did not run is not evidence of absence. For answers
// that ran, see SupportsNegativeClaim.
func (s Status) Answered() bool {
	return s == OK || s == Degraded || s == Partial
}
