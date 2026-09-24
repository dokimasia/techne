// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package files

import (
	"errors"
	"os"
	"syscall"
)

// try takes the flock(2) lock of file without waiting, and reports whether it took it.
func try(file *os.File) (bool, error) {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, syscall.EWOULDBLOCK):
		return false, nil
	}
	return false, err
}

// release releases the flock(2) lock of file.
func release(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
