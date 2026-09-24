// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd || windows)

package files

import (
	"errors"
	"os"
)

// errUnlockable is the error of [Root.Lock] on a platform without an advisory lock of a
// file. techne does not write a workspace that it cannot lock.
var errUnlockable = errors.New("files: this platform has no advisory lock of a file")

// try returns errUnlockable.
func try(*os.File) (bool, error) { return false, errUnlockable }

// release does nothing, because try takes no lock.
func release(*os.File) error { return nil }
