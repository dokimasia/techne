// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package core is what every other module speaks: the vocabulary an
// answer is written in, and the ports an engine implements. It declares
// nothing itself; the packages beneath it do.
//
// # What is not here
//
// Nothing that consumes a port. The read and write paths are
// techne-service and the surface an agent calls is techne-tool, because
// a module holding both the contract and its consumers is one where the
// two grow into each other: a tool comes to hold a service, and a
// service comes to hold a copy of another service's rules.
//
// Nothing language-specific either. Where a doc comment here names Rust
// or Java it is saying what a shared idea covers, which is what makes
// agnostic vocabulary legible rather than what makes it leak.
//
// # Dependency position
//
// Imports nothing else in this repository, and every other module may
// import it. Holding that line is what keeps core free of cgo and of
// every language ecosystem, so an embedder pays for neither.
package core
