// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package checker serves the Go roles that need types by type-checking the workspace in this
// process, with the loader of golang.org/x/tools/go/packages.
//
// [Engine] serves resolve, relate, verify and check. It runs the go command that builds the
// workspace, and does not need a language server. gopls serves the same roles at the same tier.
// The Go module registers gopls first, so the catalogue selects the checker when gopls is not
// installed. [Binding] returns the tier of each role.
//
// # Loading
//
// Under a go.work file one load type-checks the modules of the workspace under the root as one
// program, as the go command does. A module under the root that the go.work file does not list
// is a program of its own, which the go command loads with GOWORK=off. Without a go.work file,
// each module under the root is a program with a load of its own. The packages of the
// workspace come from source, with their tests. A
// dependency has the types of the export data that the go command writes, and no syntax. A
// module that fails to load makes an answer about a scope that contains it partial, and a
// caveat lists the module with its error.
//
// The engine keeps the view of the last load. Each question walks the workspace first, and the
// engine loads the view again when the stamp of the walk changes. The stamp covers the path,
// the size and the modification time of every Go file, every go.mod, go.sum, go.work and
// go.work.sum file and every list of vendored modules. The walk skips what the go command
// skips: a name that starts with a dot or an underscore, a testdata directory and a vendor
// directory.
//
// # Tests
//
// The go command compiles a package with test files twice: as the package, and as its test
// variant p [p.test], which adds the test files. An answer reads the test variant in place of
// the package, and reads the external test package p_test [p.test]. The objects of one
// declaration in a package and its test variant share the position of their name, and the
// engine matches declarations by that position.
//
// # Identities
//
// The ID of a declaration has the qualified name that the parser builds. A method is qualified
// by the type of its receiver, a field or a method of an interface by the type that declares
// it, and a parameter or a local by its function. [Engine.Relate] matches an ID to the
// declaration with that ID, or to the declaration of its unit and qualified name when the kinds
// differ.
//
// # Evidence
//
// Every answer that binds names has a caveat that the type checker does not follow reflection,
// dispatch by string or struct tags. The errors of the program of an answer lower it by the
// rule of [go.dokimi.dev/techne/lang.Lowered]. Only an error on a line that can hide a use of
// the name that the answer is about lowers it to indexed.
//
// # Gates
//
// [Engine.Check] type-checks the whole workspace with the content of a change in place of the
// files on disk, so a change that breaks a package that imports a changed package is refused.
// A file that the change deletes gets content that no build compiles. The cached view is the
// answer for a change whose content equals the files on disk.
//
// # Dependency position
//
// Imports the standard library, core/diag, core/edit, core/engine, core/sema, core/source,
// core/trust, lang and golang.org/x/tools/go/packages. The loader runs the go command, so the
// files of a package and its build constraints are the ones that the go command selects.
package checker
