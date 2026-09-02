// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag

import "go.dokimi.dev/techne/core/source"

// Severity is how much a diagnostic matters.
//
// The order rises, so a caller wanting errors alone compares against
// [SeverityError] rather than enumerating the rest. The zero value is
// [SeverityUnset].
type Severity uint8

const (
	// SeverityUnset means nobody assigned one. It is never valid on a
	// diagnostic a verifier returned.
	SeverityUnset Severity = iota
	SeverityHint
	SeverityInfo
	SeverityWarning
	SeverityError
)

// Diagnostic is one thing a verifier reported about one span of code.
type Diagnostic struct {
	Severity Severity
	// Code is the reporting tool's own identifier, such as a linter rule
	// name, carried unchanged so a caller can suppress by it without
	// matching the message text.
	Code    string
	Message string
	Span    source.Span
	// Source names the tool that reported it, so a broken build can be
	// told from a linter's objection.
	Source string
	// Snippet is the source the diagnostic is about. Whoever renders it
	// has no filesystem, and a message without the line it is about
	// costs a read per diagnostic to make sense of.
	Snippet string
}
