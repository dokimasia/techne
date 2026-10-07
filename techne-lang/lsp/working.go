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
	"strings"
	"sync"
	"time"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
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

// unanswered returns err as [engine.ErrDecline] when parent did not end and the question got no
// answer, so the next engine serves the question: when the deadline of the question ended it,
// or when the server responded with RequestCancelled or ContentModified of LSP 3.17, as metals
// does for a request that its build import cancels. It returns err otherwise.
func (e *Engine) unanswered(parent context.Context, err error) error {
	switch {
	case err == nil || parent.Err() != nil:
		return err
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%w: %s did not answer within %s: %w",
			engine.ErrDecline, e.server.Name, cmp.Or(e.server.Answering, answering), err)
	case errors.Is(err, protocol.ErrRequestCancelled), errors.Is(err, protocol.ErrContentModified):
		return fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}
	return err
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
// answers. [working.settle] does not wait for these jobs, which change the diagnostics of the
// files alone:
//
//   - A check of the files on disk, a job whose token starts with the [Server.DiskCheck] of the
//     server. [working.checked] waits for it.
//   - A diagnosis, a job whose title starts with the [Server.Diagnosis] of the server.
//     [working.diagnosed] waits for it.
//
// A server can create the token of a job before it begins the job, and the title of the job
// arrives with its begin. A created job counts as loading until its begin arrives with the
// title of a diagnosis. working is safe for concurrent use.
type working struct {
	mu   sync.Mutex
	open map[string]bool
	// idle is closed while no job is open, and replaced when the first job of a run begins.
	idle chan struct{}
	last time.Time

	// disk is the prefix of the token of a check on disk, or empty.
	disk string
	// checks are the begin times of the open checks on disk, by token.
	checks map[string]time.Time
	// ran is the begin time of the latest check on disk that ended, and saved the time of the
	// latest textDocument/didSave that the client sent.
	ran, saved time.Time

	// diagnosis is the prefix of the title of a diagnosis, or empty.
	diagnosis string
	// diagnosing are the tokens of the open diagnoses.
	diagnosing map[string]bool

	// turned is closed and replaced when a check on disk or a diagnosis ends.
	turned chan struct{}
}

// newWorking returns an idle tracker without recorded activity, whose checks on disk have
// tokens that start with disk, and whose diagnoses have titles that start with diagnosis. With
// an empty disk the tracker does not track checks on disk, and with an empty diagnosis it does
// not track diagnoses.
func newWorking(disk, diagnosis string) *working {
	idle := make(chan struct{})
	close(idle)
	return &working{
		open:       map[string]bool{},
		idle:       idle,
		disk:       disk,
		checks:     map[string]time.Time{},
		diagnosis:  diagnosis,
		diagnosing: map[string]bool{},
		turned:     make(chan struct{}),
	}
}

// began records that the job token began. A check on disk keeps the time that it began first,
// because a server can create the token of a job before it begins the job.
func (w *working) began(token string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.checking(token) {
		if _, open := w.checks[token]; !open {
			w.checks[token] = time.Now()
		}
		return
	}
	if len(w.open) == 0 {
		w.idle = make(chan struct{})
	}
	w.open[token] = true
	w.last = time.Now()
}

// begun records the begin of the job token, whose title is title. A job whose title starts with
// the prefix of a diagnosis is a diagnosis: begun moves its token from the loading jobs, where
// the create request of the token put it, to the open diagnoses. [working.began] records every
// other job.
func (w *working) begun(token, title string) {
	if w.diagnosis == "" || !strings.HasPrefix(title, w.diagnosis) {
		w.began(token)
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.open[token] {
		w.drop(token)
	}
	w.diagnosing[token] = true
}

// ended records that the job token ended. An end of a job that never began is ignored.
func (w *working) ended(token string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	began, checked := w.checks[token]
	switch {
	case checked:
		delete(w.checks, token)
		if began.After(w.ran) {
			w.ran = began
		}
	case w.diagnosing[token]:
		delete(w.diagnosing, token)
	case w.open[token]:
		w.drop(token)
		w.last = time.Now()
		return
	default:
		return
	}
	close(w.turned)
	w.turned = make(chan struct{})
}

// drop removes token from the loading jobs, and closes idle when no loading job is left. The
// caller has locked mu, and token is a loading job.
func (w *working) drop(token string) {
	delete(w.open, token)
	if len(w.open) == 0 {
		close(w.idle)
	}
}

// checking reports whether token is the token of a check on disk. The caller has locked mu.
func (w *working) checking(token string) bool {
	return w.disk != "" && strings.HasPrefix(token, w.disk)
}

// touched records that the client sent the server a buffer, which starts analysis in the
// server.
func (w *working) touched() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.last = time.Now()
}

// save records that the client sends textDocument/didSave, which starts a check on disk in a
// server that runs one. The client calls save before it sends the notification, so the check
// that the save starts begins after the recorded time.
func (w *working) save() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.saved = time.Now()
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
	defer engine.Waiting(ctx)()
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

// checked waits until a check on disk that began after the latest save has ended, for at most
// within or until ctx ends, and reports whether one has. Before the first save, any check that
// ended counts, such as the check that a server runs after it loads the workspace.
func (w *working) checked(ctx context.Context, within time.Duration) bool {
	return w.until(ctx, within, func() bool { return w.ran.After(w.saved) })
}

// diagnosed waits until no diagnosis is open, for at most within or until ctx ends, and reports
// whether none is.
func (w *working) diagnosed(ctx context.Context, within time.Duration) bool {
	return w.until(ctx, within, func() bool { return len(w.diagnosing) == 0 })
}

// until waits until done reports true, for at most within or until ctx ends, and reports
// whether it did. It calls done with mu locked: once at the start, and again each time a check
// on disk or a diagnosis ends.
func (w *working) until(ctx context.Context, within time.Duration, done func() bool) bool {
	defer engine.Waiting(ctx)()
	deadline := time.NewTimer(within)
	defer deadline.Stop()
	for {
		w.mu.Lock()
		finished, turned := done(), w.turned
		w.mu.Unlock()
		if finished {
			return true
		}
		select {
		case <-turned:
		case <-deadline.C:
			return false
		case <-ctx.Done():
			return false
		}
	}
}

// loading returns how long a question waits for the server to settle and for its check on
// disk: [Server.Loading], or [settling] for a declaration without one.
func (e *Engine) loading() time.Duration {
	if e.server.Loading > 0 {
		return e.server.Loading
	}
	return settling
}

// settle waits for the server of held to settle, for at most [Engine.loading], and reports
// whether it settled.
func (e *Engine) settle(ctx context.Context, held *session) bool {
	return held.working.settle(ctx, e.loading())
}

// diagnosed waits for the diagnoses of a server that declares [Server.Diagnosis], and reports
// whether every diagnosis ended. It fences the stream of the session with a request about the
// file at p, after which the session has recorded the begin of the diagnosis of every buffer
// that the question sent, and then waits up to [Engine.loading] until no diagnosis is open. It
// reports false when ctx ends first, and true at once for a server without the declaration.
func (e *Engine) diagnosed(ctx context.Context, held *session, p source.Path) bool {
	if e.server.Diagnosis == "" {
		return true
	}
	return e.fence(ctx, held, e.fullPath(p)) == nil && held.working.diagnosed(ctx, e.loading())
}

// progressed records a job of params that begins or ends. The token is compared as text,
// whichever of the two token types of the protocol it arrives as, and the title of a begin tells
// a diagnosis from the other jobs. A value that is not an object with a kind is ignored.
func (w *working) progressed(params *protocol.ProgressParams) {
	var value struct {
		Kind  string `json:"kind"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(params.Value, &value); err != nil {
		return
	}
	switch value.Kind {
	case progressBegin:
		w.begun(tokened(params.Token), value.Title)
	case progressEnd:
		w.ended(tokened(params.Token))
	}
}

// Progress returns nil without a record, because [ordered] records the job when the stream of
// the session reads the notification.
func (answers) Progress(context.Context, *protocol.ProgressParams) error { return nil }

// WorkDoneProgressCreate accepts the token with a null reply and records nothing. [ordered]
// records the job as begun when it reads the request, so a question asked between the request
// and the begin notification waits for the job.
func (answers) WorkDoneProgressCreate(context.Context, *protocol.WorkDoneProgressCreateParams) error {
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
