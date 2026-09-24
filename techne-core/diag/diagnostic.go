// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag

import (
	"go.dokimi.dev/techne/core/internal/wire"
	"go.dokimi.dev/techne/core/source"
)

// Severity is how much a diagnostic matters. Values increase with severity,
// so callers filter with a comparison. The zero value is SeverityUnset.
type Severity uint8

const (
	// SeverityUnset means the reporting tool assigned no severity.
	SeverityUnset Severity = iota
	// SeverityHint is a suggestion.
	SeverityHint
	// SeverityInfo is information that does not require action.
	SeverityInfo
	// SeverityWarning is a problem that does not stop the build.
	SeverityWarning
	// SeverityError is a problem that stops the build.
	SeverityError
)

var severityWords = wire.New(SeverityUnset, map[Severity]string{
	SeverityUnset:   "unset",
	SeverityHint:    "hint",
	SeverityInfo:    "info",
	SeverityWarning: "warning",
	SeverityError:   "error",
})

// String returns the wire string of s, or "unset" if s is not a declared
// Severity.
func (s Severity) String() string { return severityWords.String(s) }

// MarshalJSON encodes s as its wire string.
func (s Severity) MarshalJSON() ([]byte, error) { return severityWords.Marshal(s) }

// UnmarshalJSON decodes a wire string. An unknown string decodes to
// SeverityUnset.
func (s *Severity) UnmarshalJSON(b []byte) error { return severityWords.Unmarshal(b, s) }

// Severities returns every Severity, including SeverityUnset, in increasing
// order.
func Severities() []Severity {
	return []Severity{SeverityUnset, SeverityHint, SeverityInfo, SeverityWarning, SeverityError}
}

// Diagnostic is one problem a verifier reported in one span of code.
type Diagnostic struct {
	Severity Severity
	// Code is the reporting tool's identifier for the rule, such as a lint
	// check name. Callers suppress by it.
	Code    string
	Message string
	Span    source.Span
	// Source names the reporting tool, such as a compiler or a linter.
	Source string
	// Snippet is the source line the diagnostic is about.
	Snippet string
}
