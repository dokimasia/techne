// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package diag describes what a verifier reported about code.
//
// # Severity is ordered
//
// [Severity] rises from [SeverityHint] to [SeverityError], so a caller
// filters with a comparison rather than a set membership test. The zero
// value is [SeverityUnset] and is never valid on a diagnostic a verifier
// returned.
//
// # Who said it
//
// [Diagnostic.Source] names the tool that reported the problem and
// [Diagnostic.Code] carries that tool's own identifier unchanged, so a
// caller can tell a broken build from a linter's objection and can
// suppress by rule without matching prose.
//
// # Dependency position
//
// Imports the standard library and core/source. The gate and the verify
// tool produce these values; nothing here runs a verifier.
package diag
