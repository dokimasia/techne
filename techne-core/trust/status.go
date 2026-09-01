// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust

// Status is what happened to a request.
//
// It is not a severity. [Degraded] and [Partial] both carry a payload
// worth reading; [Unsupported] and [Refused] carry none. An empty item
// list means something different under each, which is why they are
// separate values rather than a boolean and an error string.
//
// The zero value is [Unset], so an answer whose status nobody assigned
// cannot read as a success.
type Status uint8

const (
	// Unset means nobody assigned a status. It is never valid on an
	// answer that left a service.
	Unset Status = iota
	// OK means an engine answered at the fidelity it advertises.
	OK
	// Degraded means an engine answered below the fidelity the caller
	// asked for.
	Degraded
	// Partial means some of the requested scope was not covered. The
	// caveats name what was missed.
	Partial
	// Unsupported means nothing serves this language and role. There is
	// no payload, and a caller routes around it.
	Unsupported
	// Refused means the system declined: policy, or a target that does
	// not exist. There is no payload, and a caller changes the request.
	Refused
)

// Answered reports whether an engine produced a payload.
//
// A false result means the item list is empty because nothing ran, not
// because nothing matched, so a caller must not read it as evidence of
// absence. Use [SupportsNegativeClaim] to decide what an empty list from
// an answer that did run is worth.
func (s Status) Answered() bool {
	return s == OK || s == Degraded || s == Partial
}
