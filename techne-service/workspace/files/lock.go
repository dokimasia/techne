// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Lock takes the lock of the workspace and returns the function that releases it. The lock
// is an advisory lock of the operating system on a file under the user cache directory,
// named by the SHA-256 of the path of the directory. The file is outside the workspace, and
// every techne process of the workspace locks the same file. The operating system releases
// the lock of a process that exits. Lock polls until the lock is free, and returns an error
// when ctx ends first.
func (r *Root) Lock(ctx context.Context) (func(), error) {
	name, err := r.lockFile()
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("files: open the lock %s: %w", name, err)
	}
	for wait := time.Millisecond; ; wait = min(2*wait, polls) {
		switch taken, err := try(file); {
		case err != nil:
			return nil, errors.Join(fmt.Errorf("files: lock %s: %w", name, err), file.Close())
		case taken:
			return func() {
				_ = release(file)
				_ = file.Close()
			}, nil
		}
		select {
		case <-ctx.Done():
			return nil, errors.Join(fmt.Errorf("files: lock %s: %w", name, ctx.Err()), file.Close())
		case <-time.After(wait):
		}
	}
}

// lockFile returns the path of the lock file of the workspace, and creates its directory.
func (r *Root) lockFile() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("files: find the cache directory: %w", err)
	}
	dir := filepath.Join(cache, "techne", "locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("files: make %s: %w", dir, err)
	}
	sum := sha256.Sum256([]byte(r.dir))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".lock"), nil
}

// polls is the longest wait of Lock between two tries.
const polls = 50 * time.Millisecond
