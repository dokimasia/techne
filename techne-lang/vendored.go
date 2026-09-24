// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

// vendored contains the names of the directories that contain code the
// workspace did not write: version control, dependency trees and the state
// of tools. It omits names such as build and dist, which contain source in
// some projects. The .gitignore files of a workspace exclude those where
// they contain output.
var vendored = map[string]bool{
	// Version control.
	".git": true, ".hg": true, ".svn": true, ".jj": true,

	// Dependency trees.
	"node_modules":  true, // npm, yarn and pnpm
	"vendor":        true, // Go modules, Composer and Bundler
	"target":        true, // Cargo and Maven
	".venv":         true, // Python
	"venv":          true,
	"__pycache__":   true,
	"site-packages": true,
	".bundle":       true,
	"Pods":          true, // CocoaPods

	// State that language servers and build tools write into a workspace.
	".bloop": true, ".metals": true, ".bsp": true, ".scala-build": true,
	".ruby-lsp": true, ".jdtls": true, ".eclipse": true,
	".gradle": true, ".mvn": true,
	".tox": true, ".mypy_cache": true, ".pytest_cache": true, ".ruff_cache": true,
}

// Vendored reports whether a directory named name contains code the
// workspace did not write. It compares the name only. A walk does not enter
// such a directory unless the directory is the scope.
func Vendored(name string) bool { return vendored[name] }
