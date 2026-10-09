// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import (
	"context"
	"sync/atomic"
	"time"
)

// Waited sums the time that the engines of one call wait for a process outside techne, such
// as the reply of a language server. Waits that overlap each add their whole time, so the sum
// can exceed the time of the call. Waited is safe for concurrent use.
type Waited struct {
	total atomic.Int64
}

// waitedKey is the key of the [Waited] of a context.
type waitedKey struct{}

// Timing returns ctx with a new [Waited], to which [Waiting] under the returned context adds.
func Timing(ctx context.Context) (context.Context, *Waited) {
	w := &Waited{}
	return context.WithValue(ctx, waitedKey{}, w), w
}

// Untimed returns ctx without its [Waited], for work that runs on after the call, such as the
// start of a server that later calls reuse.
func Untimed(ctx context.Context) context.Context {
	return context.WithValue(ctx, waitedKey{}, (*Waited)(nil))
}

// Waiting starts a wait under ctx and returns the function that ends it and adds its time to
// the [Waited] of ctx. The function does nothing for a context without one.
func Waiting(ctx context.Context) func() {
	w, _ := ctx.Value(waitedKey{}).(*Waited)
	if w == nil {
		return func() {}
	}
	began := time.Now()
	return func() { w.total.Add(int64(time.Since(began))) }
}

// Total returns the sum of the waits.
func (w *Waited) Total() time.Duration { return time.Duration(w.total.Load()) }
