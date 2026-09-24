// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package service is the root of the module that calls the ports of core. The root
// declares nothing, and these packages contain the services:
//
//   - [go.dokimi.dev/techne/service/query] serves the read roles.
//   - [go.dokimi.dev/techne/service/change] plans, gates and writes a change.
//   - [go.dokimi.dev/techne/service/workspace/files] reads and writes the directory of a
//     workspace.
//
// core declares the ports that an engine implements, and this module calls them, so no
// port depends on a caller of it.
//
// # Dependency position
//
// Imports the standard library and core, and golang.org/x/sys/windows for the lock of a
// workspace on Windows. The module does not import a language module, a transport or a
// schema library.
package service
