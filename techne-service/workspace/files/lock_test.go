// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package files_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/service/workspace/files"
)

// The variables of the parts of the lock tests.
const (
	// logVar is the file that the part append writes its lines to.
	logVar = "TECHNE_FILES_LOG"
	// idVar is the name that the part append writes.
	idVar = "TECHNE_FILES_ID"
	// readyVar is the file that the part hold creates once it has the lock.
	readyVar = "TECHNE_FILES_READY"
	// holdVar is how long the part hold keeps the lock, or exit to exit with the lock taken.
	holdVar = "TECHNE_FILES_HOLD"
	// patienceVar is how long the part wait waits for the lock.
	patienceVar = "TECHNE_FILES_PATIENCE"
	// rounds is the number of times that the part append takes the lock.
	rounds = 5
)

// appending takes the lock rounds times, and under the lock appends a line to the log,
// writes a.txt and b.txt of the workspace 20 milliseconds apart, and appends a second line.
func appending(root *files.Root) error {
	log, err := os.OpenFile(os.Getenv(logVar), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = log.Close() }()
	id := os.Getenv(idVar)
	for range rounds {
		unlock, err := root.Lock(context.Background())
		if err != nil {
			return err
		}
		_, opened := fmt.Fprintln(log, id+" in")
		first := root.Write("a.txt", []byte(id))
		time.Sleep(20 * time.Millisecond)
		second := root.Write("b.txt", []byte(id))
		_, closed := fmt.Fprintln(log, id+" out")
		unlock()
		if err := errors.Join(opened, first, second, closed); err != nil {
			return err
		}
	}
	return nil
}

// holding takes the lock and creates the ready file. It then exits with the lock taken, or
// releases the lock after the time that the environment sets.
func holding(root *files.Root) error {
	unlock, err := root.Lock(context.Background())
	if err != nil {
		return err
	}
	if failed := os.WriteFile(os.Getenv(readyVar), nil, 0o600); failed != nil {
		return failed
	}
	if os.Getenv(holdVar) == "exit" {
		os.Exit(0)
	}
	held, err := time.ParseDuration(os.Getenv(holdVar))
	if err != nil {
		return err
	}
	time.Sleep(held)
	unlock()
	return nil
}

// waiting waits for the lock for the patience of the environment.
func waiting(root *files.Root) error {
	patience, err := time.ParseDuration(os.Getenv(patienceVar))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), patience)
	defer cancel()
	unlock, err := root.Lock(ctx)
	if err != nil {
		return err
	}
	unlock()
	return nil
}

// created waits up to ten seconds for the file at p.
func created(t *testing.T, p string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(p); err == nil {
			return
		}
	}
	t.Fatalf("%s was not created within ten seconds", p)
}

func TestLock(t *testing.T) {
	t.Parallel()

	t.Run("Lock", func(t *testing.T) {
		t.Parallel()

		t.Run("serialises the writes of two processes", func(t *testing.T) {
			t.Parallel()
			dir, cache := t.TempDir(), t.TempDir()
			log := filepath.Join(t.TempDir(), "log")
			var outputs [2]strings.Builder
			var started [2]func() error
			for i, id := range []string{"A", "B"} {
				cmd := child(t, cache, "append", dir, "", logVar+"="+log, idVar+"="+id)
				cmd.Stdout, cmd.Stderr = &outputs[i], &outputs[i]
				assert.NoError(t, cmd.Start(), "Start of "+id)
				started[i] = cmd.Wait
			}
			for i, wait := range started {
				assert.NoError(t, wait(), "the exit of a writer: "+outputs[i].String())
			}

			lines := strings.Split(strings.TrimSpace(read(t, filepath.Dir(log), "log")), "\n")
			assert.Length(t, lines, 2*2*rounds, "the lines of the log")
			for i := 0; i < len(lines); i += 2 {
				assert.Equal(t, lines[i+1], strings.TrimSuffix(lines[i], " in")+" out", "the line after "+lines[i])
			}
		})

		t.Run("returns an error when the context ends before the lock is free", func(t *testing.T) {
			t.Parallel()
			dir, cache := t.TempDir(), t.TempDir()
			ready := filepath.Join(t.TempDir(), "ready")
			holder := child(t, cache, "hold", dir, "", readyVar+"="+ready, holdVar+"=5s")
			assert.NoError(t, holder.Start(), "Start of the holder")
			t.Cleanup(func() {
				_ = holder.Process.Kill()
				_ = holder.Wait()
			})
			created(t, ready)

			out, err := child(t, cache, "wait", dir, "", patienceVar+"=100ms").CombinedOutput()
			assert.HasError(t, err, "the exit of the waiter")
			assert.Contains(t, string(out), context.DeadlineExceeded.Error(), "the error of the waiter")
		})

		t.Run("frees the lock of a process that exits without releasing it", func(t *testing.T) {
			t.Parallel()
			dir, cache := t.TempDir(), t.TempDir()
			ready := filepath.Join(t.TempDir(), "ready")
			out, err := child(t, cache, "hold", dir, "", readyVar+"="+ready, holdVar+"=exit").CombinedOutput()
			assert.NoError(t, err, "the exit of the holder: "+string(out))

			out, err = child(t, cache, "wait", dir, "", patienceVar+"=2s").CombinedOutput()
			assert.NoError(t, err, "the exit of the waiter: "+string(out))
		})
	})
}
