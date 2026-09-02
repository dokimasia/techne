// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import "io/fs"

// Workspace is the tree a language module builds its engines over.
//
// Two views of one tree, because the engines need different things from
// it. A parser reads content and nothing else, so it takes an
// [io/fs.FS]: every path stays relative, nothing can climb out, and the
// tree need never have been written to disk. A language server is a
// process that opens files itself and cannot be handed a filesystem that
// is not one, so it takes a path.
//
// The second is therefore optional. A workspace that is nowhere on disk
// carries an empty [Workspace.Root], and a module registers its parser
// and no server rather than failing.
type Workspace struct {
	// FS is the tree, rooted at the workspace.
	FS fs.FS

	// Root is where that tree is on disk, and is empty when it is
	// nowhere.
	Root string
}

// OnDisk reports whether this workspace can host a process that opens
// files by name.
func (w Workspace) OnDisk() bool { return w.Root != "" }
