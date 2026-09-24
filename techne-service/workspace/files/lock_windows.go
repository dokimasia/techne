// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

//go:build windows

package files

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// try takes the LockFileEx lock of the first byte of file without waiting, and reports
// whether it took it.
func try(file *os.File) (bool, error) {
	var at windows.Overlapped
	err := windows.LockFileEx(windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &at)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, windows.ERROR_LOCK_VIOLATION):
		return false, nil
	}
	return false, err
}

// release releases the LockFileEx lock of file.
func release(file *os.File) error {
	var at windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &at)
}
