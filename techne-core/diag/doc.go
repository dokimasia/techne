// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package diag describes the problems a verifier reports in code.
//
// [Diagnostic.Source] is the tool that reported the problem and
// [Diagnostic.Code] is that tool's rule identifier. Callers use them to tell
// a compiler error from a lint warning and to suppress by rule without
// parsing the message. [Severity] values increase with severity.
//
// # Dependency position
//
// Imports the standard library, core/source, and core/internal/wire.
package diag
