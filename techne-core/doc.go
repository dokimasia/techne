// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package core is the root of the techne core module and declares nothing.
// Its packages define the vocabulary of every answer and the ports every
// engine implements:
//
//   - [go.dokimi.dev/techne/core/source] defines languages, paths and byte
//     positions.
//   - [go.dokimi.dev/techne/core/sema] defines declarations, kinds and
//     relations.
//   - [go.dokimi.dev/techne/core/trust] defines fidelity, coverage, status
//     and provenance.
//   - [go.dokimi.dev/techne/core/diag] defines diagnostics and severities.
//   - [go.dokimi.dev/techne/core/edit] defines operations, plans and the
//     write policy.
//   - [go.dokimi.dev/techne/core/engine] defines the engine ports and the
//     catalogue that selects engines.
//
// # Scope
//
// Core contains no consumer of a port and no language. The read and write
// paths are in techne-service, and the tools an agent calls are in
// techne-tool. Each language module declares its own language value.
//
// # Dependency position
//
// Imports the standard library and its own packages. It does not use cgo, so
// it builds with CGO_ENABLED=0. Every other module of techne can import it.
package core
