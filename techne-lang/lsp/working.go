// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"go.dokimi.dev/techne/core/trust"
	"go.lsp.dev/protocol"
)

// The kinds of progress a server reports. The first and the last are
// what matter here: a job begins, and later it ends.
const (
	progressBegin = "begin"
	progressEnd   = "end"
)

// settling is how long a question waits for a server that declared no
// figure of its own.
//
// Long enough for a small project to load, short enough that a caller
// asking about a workspace that will take minutes gets a partial answer
// rather than a stall. A server that needs longer says so through
// [Server.Loading].
const settling = 10 * time.Second

// announcing is how long a freshly started server is given to say that
// it is busy before it is believed to be idle.
//
// A server that loads a workspace says so within a few milliseconds of
// the handshake. One that never reports progress pays this once per
// session and nothing after.
const announcing = 500 * time.Millisecond

// working tracks the jobs a server has told the client it is doing.
//
// # Why this exists
//
// A server that has not finished loading a project answers every
// question with nothing. Not "I do not know" — nothing, in the same
// shape as a real answer. Reported as it stands, that is resolved
// binding over total coverage saying a declaration has no references,
// which is exactly the claim a caller acts on by deleting it.
//
// The protocol's own signal for this is work-done progress: a job begins
// and later ends, and while one is outstanding the server is changing
// what it would answer. Nothing in the specification says which jobs
// matter, so all of them count. The cost of counting one that did not is
// an answer marked partial; the cost of missing one is a false claim
// that something is unused.
type working struct {
	mu   sync.Mutex
	open map[string]bool
	// idle is closed when the last job ends, and replaced when the next
	// begins, so a waiter blocks on the state as it was when it asked.
	idle chan struct{}
	// started counts the jobs ever begun. Loading a workspace is a run
	// of short jobs with gaps between them, so "nothing running" is not
	// "finished": a question asked in a gap is asked of a server that is
	// about to start again. Comparing this across a quiet period is what
	// tells the two apart.
	started int
}

func newWorking() *working {
	held := &working{open: map[string]bool{}, idle: make(chan struct{})}
	close(held.idle)
	return held
}

// quiet is how long nothing may happen before a server is taken to have
// finished loading.
//
// Loading is a run of jobs a few milliseconds apart. Shorter than the
// gaps and every gap reads as finished; much longer and a server that
// genuinely finished is waited on for no reason.
const quiet = 300 * time.Millisecond

// began records a job the server started.
func (w *working) began(token string) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.open) == 0 {
		w.idle = make(chan struct{})
	}
	w.open[token] = true
	w.started++
}

// ended records a job the server finished, and wakes whoever waited for
// the last of them.
func (w *working) ended(token string) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.open[token] {
		return
	}
	delete(w.open, token)
	if len(w.open) == 0 {
		close(w.idle)
	}
}

// announce waits a bounded time for a server to report its first job.
//
// Called once, after the handshake. Until a server has said anything at
// all there is no way to tell one that is idle from one that has not
// started, and the difference is whether an empty answer means empty.
func (w *working) announce(ctx context.Context, within time.Duration) {
	if w == nil || w.busy() {
		return
	}
	deadline := time.NewTimer(within)
	defer deadline.Stop()

	// Polled rather than signalled: nothing has begun yet, so there is
	// no channel to wait on that a begin would close.
	tick := time.NewTicker(within / 10)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if w.busy() {
				return
			}
		case <-deadline.C:
			return
		case <-ctx.Done():
			return
		}
	}
}

// busy reports whether the server is in the middle of something.
func (w *working) busy() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.open) > 0
}

// settle waits a bounded time for the server to finish what it is doing,
// and reports whether it did.
//
// Bounded because a server loading a large workspace takes as long as it
// takes, and a caller waiting on it has no way to tell a slow start from
// a hang. A question that outwaits this is still answered; what it must
// not do is claim the answer is complete.
func (w *working) settle(ctx context.Context, within time.Duration) bool {
	if w == nil {
		return true
	}
	deadline := time.NewTimer(within)
	defer deadline.Stop()

	for {
		w.mu.Lock()
		idle, running, began := w.idle, len(w.open) > 0, w.started
		w.mu.Unlock()

		if running {
			select {
			case <-idle:
			case <-deadline.C:
				return false
			case <-ctx.Done():
				return false
			}
			continue
		}

		// Idle, but loading is a run of jobs with gaps: wait out a quiet
		// period and see whether another one starts. Nothing new means
		// the server has finished rather than paused.
		pause := time.NewTimer(quiet)
		select {
		case <-pause.C:
		case <-deadline.C:
			pause.Stop()
			return false
		case <-ctx.Done():
			pause.Stop()
			return false
		}
		pause.Stop()

		w.mu.Lock()
		still := w.started == began && len(w.open) == 0
		w.mu.Unlock()
		if still {
			return true
		}
	}
}

// warming is the caveat on an answer a server gave while it was still
// loading, and the empty caveat otherwise.
//
// It carries [trust.ScopePartial] with it at every call site, because
// the two say one thing: what came back is not everything there is, and
// no absence can be read out of it.
var warming = trust.Caveat{
	Code: trust.CaveatIndexWarming,
	Note: "the server was still loading the workspace, so what it did not " +
		"return may not be absent",
}

// settled waits for a server to finish loading, and reports the
// completeness and caveats an answer given now is entitled to.
//
// Every read calls it. A server mid-load answers with nothing, and
// nothing over total coverage is a claim that there is nothing — the one
// claim this package exists to be trusted about.
func (e *Engine) settled(ctx context.Context) (trust.Completeness, []trust.Caveat) {
	if e.working.settle(ctx, e.settling()) {
		return trust.ScopeTotal, []trust.Caveat{dynamic}
	}
	return trust.ScopePartial, []trust.Caveat{dynamic, warming}
}

// bound is what an answer that rests on binding is worth: how much of
// the scope was covered, the tier the answer reaches, and the limits on
// it.
//
// Every role that resolves a name uses it. Verifying does not: reporting
// what is wrong with a workspace is the one answer a workspace being
// wrong does not weaken.
func (e *Engine) bound(
	ctx context.Context,
) (trust.Completeness, trust.Fidelity, []trust.Caveat) {
	covered, caveats := e.settled(ctx)
	held, why := e.lowered()
	return covered, held, append(caveats, why...)
}

// lowered is the tier an answer is worth while the workspace does not
// compile, and nothing while it does.
//
// A type checker over a program with a fault in it binds the names it
// can and guesses at the rest: a reference list is what the last good
// build had plus whatever survives, which is an index rather than a
// binding. Every answer says so rather than claiming the tier the engine
// reaches when the code is whole.
func (e *Engine) lowered() (trust.Fidelity, []trust.Caveat) {
	if !e.pushed.broken() {
		return trust.None, nil
	}
	return trust.Indexed, []trust.Caveat{{
		Code: trust.CaveatBuildBroken,
		Note: "the server reports the workspace does not compile, so names are bound " +
			"where it could bind them and matched where it could not",
	}}
}

// settling is how long this engine's questions wait for its server.
func (e *Engine) settling() time.Duration {
	if e.server.Loading > 0 {
		return e.server.Loading
	}
	return settling
}

// Progress records a job beginning or ending.
//
// The token is whatever the server chose to call it. Only its identity
// matters, so it is read as text rather than decoded into the two shapes
// the protocol allows it to take.
func (a answers) Progress(_ context.Context, params *protocol.ProgressParams) error {
	var held struct {
		Kind string `json:"kind"`
	}
	// A progress value this cannot read is a job neither begun nor
	// ended, which is what it already is. Reporting it would tear the
	// connection down over a notification nobody asked for.
	if err := json.Unmarshal(params.Value, &held); err == nil {
		switch held.Kind {
		case progressBegin:
			a.working.began(tokened(params.Token))
		case progressEnd:
			a.working.ended(tokened(params.Token))
		}
	}
	return nil
}

// tokened is a progress token as text, whichever of the two shapes the
// protocol allows it arrived in.
func tokened(held protocol.ProgressToken) string {
	switch token := held.(type) {
	case protocol.String:
		return string(token)
	case protocol.Integer:
		return "#" + strconv.Itoa(int(token))
	}
	return ""
}
