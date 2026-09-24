// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.dokimi.dev/techne/core/engine"
	"go.lsp.dev/protocol"
)

// The kinds of work-done progress value that begin and end a job.
const (
	progressBegin = "begin"
	progressEnd   = "end"
)

// settling is how long a question waits for a server whose declaration sets no
// [Server.Loading].
const settling = 10 * time.Second

// answering is how long a question waits for the answers of a server whose declaration sets
// no [Server.Answering].
const answering = time.Minute

// answered returns ctx with the deadline of one question, and the function that releases it.
// A question takes the deadline after the server runs, so the start of a server that imports a
// build for minutes is not cut short.
func (e *Engine) answered(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, cmp.Or(e.server.Answering, answering))
}

// unanswered returns err as [engine.ErrDecline] when the deadline of a question ended it and
// parent did not end, so the next engine answers the question, and err otherwise.
func (e *Engine) unanswered(parent context.Context, err error) error {
	if err == nil || parent.Err() != nil || !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %s did not answer within %s: %w",
		engine.ErrDecline, e.server.Name, cmp.Or(e.server.Answering, answering), err)
}

// announcing is how long the handshake waits for a started server to report its first job.
const announcing = 500 * time.Millisecond

// quiet is how long a server must go without a job, and the client without sending a buffer,
// before a question takes the server as settled. A server that loads a workspace runs a series
// of jobs with gaps of a few milliseconds between them.
const quiet = 300 * time.Millisecond

// working tracks the LSP 3.17 work-done progress jobs that a server reports, and the time of
// the last activity: a job that begins or ends, or a buffer the client sends.
//
// A server that has not finished loading returns empty answers in the same form as complete
// ones. Every job counts as loading, because the protocol does not say which jobs change
// answers. working is safe for concurrent use.
type working struct {
	mu   sync.Mutex
	open map[string]bool
	// idle is closed while no job is open, and replaced when the first job of a run begins.
	idle chan struct{}
	last time.Time
}

// newWorking returns an idle tracker without recorded activity.
func newWorking() *working {
	idle := make(chan struct{})
	close(idle)
	return &working{open: map[string]bool{}, idle: idle}
}

// began records that the job token began.
func (w *working) began(token string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.open) == 0 {
		w.idle = make(chan struct{})
	}
	w.open[token] = true
	w.last = time.Now()
}

// ended records that the job token ended. An end of a job that never began is ignored.
func (w *working) ended(token string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.open[token] {
		return
	}
	delete(w.open, token)
	w.last = time.Now()
	if len(w.open) == 0 {
		close(w.idle)
	}
}

// touched records that the client sent the server a buffer, which starts analysis in the
// server.
func (w *working) touched() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.last = time.Now()
}

// busy reports whether a job is open.
func (w *working) busy() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.open) > 0
}

// announce waits until the server reports a job, for at most within or until ctx ends. A
// server that loads a workspace reports its first job a few milliseconds after the handshake.
func (w *working) announce(ctx context.Context, within time.Duration) {
	if w.busy() {
		return
	}
	deadline := time.NewTimer(within)
	defer deadline.Stop()
	// Polled, because no job has begun and so no channel exists that a job would close.
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

// settle waits until no job is open and [quiet] has passed since the last activity, and
// reports whether that happened within the deadline and before ctx ended. It returns true at
// once for a server that has been quiet for that long.
func (w *working) settle(ctx context.Context, within time.Duration) bool {
	deadline := time.NewTimer(within)
	defer deadline.Stop()

	for {
		w.mu.Lock()
		idle, running, last := w.idle, len(w.open) > 0, w.last
		w.mu.Unlock()

		if running {
			select {
			case <-idle:
				continue
			case <-deadline.C:
				return false
			case <-ctx.Done():
				return false
			}
		}
		left := quiet - time.Since(last)
		if left <= 0 {
			return true
		}
		pause := time.NewTimer(left)
		select {
		case <-pause.C:
		case <-deadline.C:
			pause.Stop()
			return false
		case <-ctx.Done():
			pause.Stop()
			return false
		}
	}
}

// settle waits for the server of held to settle, for at most [Server.Loading] or [settling],
// and reports whether it settled.
func (e *Engine) settle(ctx context.Context, held *session) bool {
	within := e.server.Loading
	if within <= 0 {
		within = settling
	}
	return held.working.settle(ctx, within)
}

// Progress records a job that begins or ends. The token is compared as text, whichever of
// the two token types of the protocol it arrives as. A value that is not an object with a
// kind is ignored.
func (a answers) Progress(_ context.Context, params *protocol.ProgressParams) error {
	var value struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(params.Value, &value); err == nil {
		switch value.Kind {
		case progressBegin:
			a.working.began(tokened(params.Token))
		case progressEnd:
			a.working.ended(tokened(params.Token))
		}
	}
	return nil
}

// WorkDoneProgressCreate records the job of a token as begun when the request arrives, before
// the begin notification, so a question asked between the two waits for the job. The end of
// the job closes it.
func (a answers) WorkDoneProgressCreate(
	_ context.Context,
	params *protocol.WorkDoneProgressCreateParams,
) error {
	a.working.began(tokened(params.Token))
	return nil
}

// tokened returns a progress token as text: a string token as it is, and an integer token as
// "#" and its decimal digits.
func tokened(token protocol.ProgressToken) string {
	switch held := token.(type) {
	case protocol.String:
		return string(held)
	case protocol.Integer:
		return "#" + strconv.Itoa(int(held))
	}
	return ""
}
