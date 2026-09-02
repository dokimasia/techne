// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"slices"
	"sync"

	"go.dokimi.dev/techne/core/source"
)

// locks serialises callers changing the same files.
//
// Per path rather than per workspace: two changes to different files
// have nothing to do with each other, and a single lock would make every
// change wait for every other one's gate.
//
// A path's mutex outlives the call that took it. Reference counting them
// away would trade a bounded map for a race between the last release and
// the next acquire, and the bound here is the number of files ever
// written in one session.
type locks struct {
	mu   sync.Mutex
	held map[source.Path]*sync.Mutex
}

func newLocks() *locks {
	return &locks{held: map[source.Path]*sync.Mutex{}}
}

// hold takes every path's lock and returns the call that gives them
// back.
//
// The paths are taken in the order they arrive, and [edit.Plan.Paths]
// sorts them. Two changes touching the same files in different orders
// would otherwise each hold what the other is waiting for.
func (l *locks) hold(paths []source.Path) func() {
	taken := make([]*sync.Mutex, 0, len(paths))
	for _, p := range paths {
		one := l.at(p)
		one.Lock()
		taken = append(taken, one)
	}
	return func() {
		for _, one := range slices.Backward(taken) {
			one.Unlock()
		}
	}
}

// at returns the mutex for one path, making it on first use.
func (l *locks) at(p source.Path) *sync.Mutex {
	l.mu.Lock()
	defer l.mu.Unlock()
	if one, made := l.held[p]; made {
		return one
	}
	one := &sync.Mutex{}
	l.held[p] = one
	return one
}
