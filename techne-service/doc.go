// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package service is the root of the half of techne that drives the
// ports: the read path, the write path, and the workspace they run
// over. It declares nothing itself; the packages beneath it do.
//
// # Why this is not core
//
// core declares what an engine implements. This consumes it. Keeping
// the two apart is what stops a port growing a dependency on the thing
// that calls it, which is how a tool came to hold a service and a
// service came to hold a copy of another service's rules.
//
// # Dependency position
//
// Imports core and nothing else in this repository. It names no
// language, holds no transport and derives no schema.
package service
