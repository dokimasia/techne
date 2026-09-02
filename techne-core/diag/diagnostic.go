// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag

import (
	"strconv"

	"go.dokimi.dev/techne/core/source"
)

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

// severityNames is the single definition point for the wire form of
// each severity. An answer names them, so these strings reach a caller.
var severityNames = map[Severity]string{
	SeverityUnset:   "unset",
	SeverityHint:    "hint",
	SeverityInfo:    "info",
	SeverityWarning: "warning",
	SeverityError:   "error",
}

// String returns the wire form of the severity, or the form of
// [SeverityUnset] for a value outside the declared set.
func (s Severity) String() string {
	if name, ok := severityNames[s]; ok {
		return name
	}
	return severityNames[SeverityUnset]
}

// MarshalJSON writes the severity as the word a caller reads.
//
// A caller branching on a number would have to know the order, and the
// order is this package's to change.
func (s Severity) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(s.String())), nil
}

// UnmarshalJSON reads the wire form back. A word this package does not
// know becomes [SeverityUnset], which is what a verifier that did not
// say how much something matters reports.
func (s *Severity) UnmarshalJSON(b []byte) error {
	name, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	*s = SeverityUnset
	for held, spelt := range severityNames {
		if spelt == name {
			*s = held
			return nil
		}
	}
	return nil
}

// Severities returns every severity, including [SeverityUnset].
//
// Unset is in the set because it is an answer: a verifier that did not
// say how much something matters is different from one that said it
// matters least.
func Severities() []Severity {
	return []Severity{
		SeverityUnset, SeverityHint, SeverityInfo, SeverityWarning, SeverityError,
	}
}

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
