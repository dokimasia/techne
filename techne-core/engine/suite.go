// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"fmt"
	"slices"
	"strings"
)

// Unrun returns nil when the engine named name runs every suite of asked, and otherwise an
// error that wraps [ErrDecline] and names the suites of asked that runs does not contain. A
// [Verifier] returns it for a request that names a suite it does not run, because an answer
// without that suite says nothing about it.
func Unrun(name string, runs, asked []string) error {
	var others []string
	for _, one := range asked {
		if !slices.Contains(runs, one) && !slices.Contains(others, one) {
			others = append(others, one)
		}
	}
	if len(others) == 0 {
		return nil
	}
	ran := "no suite"
	if len(runs) > 0 {
		ran = "only " + strings.Join(runs, ", ")
	}
	return fmt.Errorf("%w: %s runs %s, and not %s", ErrDecline, name, ran, strings.Join(others, ", "))
}
