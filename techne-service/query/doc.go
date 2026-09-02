// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package query answers read questions by selecting an engine and
// publishing what it returns.
//
// # One path for every role
//
// [Service] dispatches every read role through the same steps: resolve
// the language, take the engines the catalogue offers strongest first,
// ask each until one answers, and stamp the result. What counts as
// degraded is decided in one place, so the rule cannot drift between
// outline and relations.
//
// # Declining is not failing
//
// An engine returning [engine.ErrDecline] serves the role but cannot
// answer this request, so the next engine gets a turn. Any other error
// stops the search and reaches the caller: answering from a weaker
// engine when the stronger one is broken hides the breakage for as long
// as anyone believes the answer.
//
// # Nothing to ask is an answer
//
// A language nothing serves, and a path no language claims, both produce
// [trust.Unsupported] with no payload rather than an error. A caller
// routes around a capability gap; it cannot route around a fault.
//
// # Dependency position
//
// Imports core/engine, core/sema, core/source and core/trust. [Router]
// is a port: the registry that knows which language claims a path lives
// in another module, and core names no language.
package query
