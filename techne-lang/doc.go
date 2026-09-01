// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package lang declares what a language is, independently of which
// engine answers questions about it, and holds the engines more than one
// language can use.
//
// # Dependency position
//
// Imports core and nothing else in this repository. A language module
// imports this package; nothing here imports a language module.
package lang
