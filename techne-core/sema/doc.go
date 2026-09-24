// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package sema describes declarations, the edges between them, and the IDs
// that identify them across processes.
//
// # IDs
//
// NewID derives an ID from a declaration's language, unit, qualified name,
// and kind. Moving a declaration within its file keeps its ID, so IDs can be
// stored in an index. Renaming the declaration or its unit changes the ID.
//
// The qualified name joins the names of the declarations that contain a
// declaration and its own name with dots, as [Qualify] does:
//
//   - A method Get of a type Store is Store.Get, and a parameter p of Get is
//     Store.Get.p.
//   - A Go method is contained by the type of its receiver, which the syntax
//     writes outside the type.
//   - An import is not qualified, because its name is a path.
//
// Members of one name and kind in different containers of one unit have
// different IDs. Overloads in one container share one ID.
//
// # A shared vocabulary
//
// [Kind] and [RelationKind] are shared by every language. Each language maps
// the distinctions of its grammar onto the closest value, so a caller does
// not depend on the language of a declaration. [Kind.Declares] separates
// declarations that code elsewhere can name from bindings local to one
// scope.
//
// # Annotations
//
// Annotations, decorators, attributes, and struct tags declare nothing. Each
// is an [Annotation] of the declaration it is attached to. A declared
// annotation type is a [Symbol] of [KindAnnotation].
//
// # Directions
//
// Every [RelationKind] in [RelationKinds] has its inverse in RelationKinds,
// and [RelationKind.Inverse] returns it. An engine returns the relations in
// the direction a request names.
//
// # Nesting
//
// [Containers] computes the enclosing declaration of every symbol from its
// span.
//
// # Dependency position
//
// Imports the standard library, core/source, and core/internal/wire.
package sema
