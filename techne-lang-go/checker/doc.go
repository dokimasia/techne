// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package checker serves the Go roles that need types by type-checking the workspace in this
// process, with the loader of golang.org/x/tools/go/packages.
//
// [Engine] serves resolve, relate, plan, verify and check. It runs the go command that builds
// the workspace, and does not need a language server. gopls serves the same roles at the same
// tier. The Go module registers gopls first, so the catalogue selects the checker when gopls is
// not installed. The checker plans the move of a file alone, which gopls declines because it does
// not serve workspace/willRenameFiles. [Binding] returns the tier of each role.
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
// # Build constraints
//
// The go command loads one build of each package: the Go files that the build constraints select
// for the operating system, the architecture and the tags of the environment. A pattern that ends
// in /... leaves out a directory whose Go files the build constraints all exclude, such as the
// directory of a package of windows alone. The engine lists each such directory of the walk by
// its path, so its files count as excluded files too. A file that the build constraints exclude,
// such as a file of another operating system, has no syntax and no types in the load, and an
// answer states the files that it does not read:
//
//   - A relation is partial when an excluded file can contain one, and a caveat names the files.
//     A file can contain a use of a declaration when it names the declaration. The file must
//     also be in the directory of a dependent package or import one. The dependent packages are
//     the package of the declaration and each package that imports a dependent package.
//   - A verify is partial when its scope contains an excluded file, and a caveat names the files.
//   - A check has a caveat that names the excluded files that can use a changed package.
//   - [Engine.importers] reads the imports of an excluded file from its source, so the answer
//     about the importers of a package contains them.
//
// [Engine.Unread], [Engine.Unverified] and [Engine.Unchecked] return the caveats of the answers
// of gopls. gopls reads a file of the default build in that build, and a file that the default
// build excludes in the build of another port that includes it. gopls renames a declaration in
// the default build alone. [Engine.Renamed] type-checks each excluded file that contains the name
// of the declaration in the build of a port that includes it, and returns the changes that rename
// its uses there, with the caveat of the files that it cannot rename.
//
// # Renames
//
// gopls renames each word of the doc comment of a renamed declaration that equals the old name,
// also a word of its text. [Engine.Referenced] keeps the edits of the plan of gopls that rewrite
// a reference, and leaves out every other edit in a comment. A reference is one of these:
//
//   - a name of the code
//   - the last name of a doc link
//   - the word that starts the doc comment of the declaration, which the Go convention makes
//     its name
//
// # Moves
//
// [Engine.Plan] moves a file into the package of another directory and rewrites the uses of the
// declarations that the move separates. Each file gets the edits of its role: the moved file,
// another file of the source package, a file of the destination package, and any other file.
// The rules of the four roles apply to the godoc links of each file. A rewritten link qualifies
// a declaration by the name under which its file imports the package, and otherwise by the
// import path of the package. Each rewritten file imports the packages that it uses, and goimports prints
// it. The plan contains the lines that change.
//
// One file cannot import two packages under one name. When the destination has the name of the
// source, an importing file gets one of two edits:
//
//   - A file that uses only moved declarations imports the destination in place of the source.
//   - A file that also uses a declaration of the source imports the destination under an alias:
//     the name of the parent directory of the destination followed by the name of the package.
//
// Another package can use only an exported name of a package that is not an external test. The
// plan is refused when the move puts a use of any other name on the other side of the two
// packages, and the refusal lists the declarations.
//
// A file that the build constraints of the load exclude has no types. The plan rewrites the
// qualified uses of the moved declarations in it by name, with a caveat that lists the file.
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
// core/trust, lang, golang.org/x/tools/go/packages, golang.org/x/tools/go/ast/astutil and
// golang.org/x/tools/imports. The loader runs the go command, so the files of a package and its
// build constraints are the ones that the go command selects. go/build decides whether the
// build of another port includes a file, as gopls decides it. A move edits imports with astutil
// and prints a file with the printer of goimports, which does not add or delete an import.
package checker
