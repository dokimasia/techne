// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"slices"
	"sync"

	"go.dokimi.dev/techne/core/source"
)

// locks serialises the changes of this process that share a path. It locks each path, so
// changes of different files do not wait for each other. The mutex of a path is kept for
// the life of the service, so the map has one entry per path that a change touched.
type locks struct {
	mu    sync.Mutex
	paths map[source.Path]*sync.Mutex
}

// newLocks returns locks without a path.
func newLocks() *locks {
	return &locks{paths: map[source.Path]*sync.Mutex{}}
}

// hold locks each path in the order of paths, and returns the function that unlocks them.
// [edit.Plan.Paths] returns sorted paths, so two changes lock their shared paths in one
// order and cannot deadlock.
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

// at returns the mutex of p, and creates it on first use.
func (l *locks) at(p source.Path) *sync.Mutex {
	l.mu.Lock()
	defer l.mu.Unlock()
	if one, found := l.paths[p]; found {
		return one
	}
	one := &sync.Mutex{}
	l.paths[p] = one
	return one
}
