// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package core is the root of the language-agnostic half of techne: the
// vocabulary every other module speaks, the ports an engine implements,
// and the services that drive them. It declares nothing itself; the
// packages beneath it do.
//
// # Dependency position
//
// Imports nothing else in this repository, and every other module may
// import it. Holding that line is what keeps core free of cgo and of
// every language ecosystem, so an embedder pays for neither.
package core
