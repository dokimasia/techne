// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import "io/fs"

// Workspace is the tree a language module builds its engines over.
//
// FS serves the parsers, which read content through io/fs. Root serves the
// language servers, which open files by path. A workspace that exists only
// in memory has an empty Root, and a language module then registers no
// server.
type Workspace struct {
	// FS is the tree, rooted at the workspace root.
	FS fs.FS

	// Root is the directory of the tree on disk, or empty.
	Root string
}

// OnDisk reports whether Root is set.
func (w Workspace) OnDisk() bool { return w.Root != "" }
