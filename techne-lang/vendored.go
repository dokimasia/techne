// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

// vendored are the directory names that hold code a workspace did not
// write.
//
// # Why a walk has to skip them
//
// A scope is walked for the files a language claims, and a dependency
// tree is full of them. Searching a two-declaration TypeScript project
// for a name returned a hundred and twenty-five matches, every one of
// them in node_modules, reported as total coverage of the workspace.
// That answer is true and useless: the caller asked what its own code
// declares.
//
// It is worse than noise on the write path, where the file set a change
// is planned over and gated against is this same walk. A rename that
// reaches a dependency is a rename that cannot be applied.
//
// # What is in the set, and what is not
//
// Only names that hold source a project did not write. Directories that
// hold build output are absent: nothing here matches a compiled
// artefact's extension, so bin and obj cost a caller nothing by being
// walked, and a project keeping real source under either would lose it.
//
// The same test excludes build and dist, which are output for some
// projects and source for others. A workspace that needs them skipped
// wants the rule its own version control already has, which this is not
// pretending to be.
//
// The set is by directory name at any depth, because that is how every
// one of these is laid out and none of them is anchored to the root: a
// monorepo has a node_modules per package.
var vendored = map[string]bool{
	// Version control, which holds a copy of everything.
	".git": true, ".hg": true, ".svn": true, ".jj": true,

	// Dependencies, by ecosystem. Each holds source in a language techne
	// serves, which is what makes them worth skipping rather than
	// harmless.
	"node_modules":  true, // npm, yarn and pnpm
	"vendor":        true, // Go modules, Composer, Bundler
	"target":        true, // Cargo, and Maven's generated sources
	".venv":         true, // Python, and site-packages under it
	"venv":          true,
	"__pycache__":   true,
	"site-packages": true,
	".bundle":       true,
	"Pods":          true, // CocoaPods

	// Tool state that mirrors sources into itself. A language server
	// writes its own bookkeeping into the workspace it was pointed at —
	// ruby-lsp bootstraps a bundle, metals and jdtls keep an index —
	// and none of it is the workspace's own code.
	".bloop": true, ".metals": true, ".bsp": true, ".scala-build": true,
	".ruby-lsp": true, ".jdtls": true, ".eclipse": true,
	".gradle": true, ".mvn": true,
	".tox": true, ".mypy_cache": true, ".pytest_cache": true, ".ruff_cache": true,
}

// Vendored reports whether a directory of this name holds code the
// workspace did not write.
//
// It answers on the name alone, so it needs no filesystem and no
// context. A walk asks it per directory and does not descend.
func Vendored(name string) bool { return vendored[name] }
