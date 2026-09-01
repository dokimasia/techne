// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package sema describes what code means: the declarations a scope
// makes, how they reach each other, and the identity by which one is
// named across processes.
//
// # Identity
//
// [ID] is derived from what a declaration is, never from where it sits.
// Moving a declaration inside its file leaves its identity unchanged, so
// an index may store one and a later process may still resolve it.
// Renaming the symbol or its unit does change the identity, which is
// correct: that is a different declaration. [NewID] is the only way to
// build one.
//
// # A shared vocabulary, not a union
//
// [Kind] and [RelationKind] are smaller than any one language's grammar.
// A language that draws a distinction they do not carry maps both sides
// onto the nearest value rather than adding one, so a caller reads the
// same set whichever language answered.
//
// # Directions
//
// Every [RelationKind] in [RelationKinds] has an inverse. An engine
// implements whichever direction it can compute and a service turns the
// question around, so a caller asks for callers or callees without
// knowing which the engine stored.
//
// # Dependency position
//
// Imports the standard library and core/source. Engines produce these
// values and services pass them on unchanged.
package sema
