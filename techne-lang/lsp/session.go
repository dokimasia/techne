// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// starting is how long the questions to a new server wait for its reply to initialize, from its
// start. [Server.Loading] bounds the load of the workspace that follows.
const starting = 10 * time.Second

// launching is how long a server has to reply to initialize, from its start. A server that
// misses [starting] goes on starting, and a question declines until the reply arrives. ruby-lsp
// installs the gems of its composed bundle before it replies, which took 28 s with a fetch of
// the index of rubygems.org. A server that misses launching fails to start, and the engine keeps
// the failure until [Engine.Close].
const launching = 2 * time.Minute

// leaving is how long a server has to answer shutdown and exit before [session.stop] kills it.
const leaving = 5 * time.Second

// draining is how long [session.stop] waits for the stderr of a server to close after the
// process ends. A child of the server can keep it open: ruby-lsp runs bundle install in a child
// that outlives a killed server.
const draining = 500 * time.Millisecond

// stderrSize is how many bytes of a server's stderr a session keeps.
const stderrSize = 8 << 10

// session is one server process, the connection to it, and the state of the server for the
// connection: its buffers, its diagnostics, its progress jobs and the edits it offered. A new
// session starts with none of that state.
//
// The process and the connection start together in [start] and stop together in
// [session.stop]. Every field that changes after start has its own lock, so a question and a
// stop can run concurrently.
type session struct {
	cmd  *exec.Cmd
	conn jsonrpc2.Conn
	// asks is the server as a [protocol.Server], with one method per request of LSP 3.17.
	asks protocol.Server
	// capable is the capabilities that the server returned from initialize.
	capable protocol.ServerCapabilities
	// stderr keeps the last [stderrSize] bytes that the server wrote to stderr.
	stderr *tail
	// started is the time before the process started. The first check on disk of a server
	// covers a file that did not change after it.
	started time.Time

	// opening guards opened, the buffer of the server for each absolute path.
	opening sync.Mutex
	opened  map[string]sent

	reports  *reports
	working  *working
	offering *asking

	// ends makes stop run once.
	ends sync.Once
}

// start runs the server that declared names in root and connects to it over stdin and stdout.
// The process does not end with ctx, because it serves every later question. The caller
// checks with [Server.Installed] first, so a missing command is reported as unavailable and
// not as a failed call.
func start(ctx context.Context, declared Server, root string) (*session, error) {
	held := &session{
		stderr:   &tail{},
		started:  time.Now(),
		opened:   map[string]sent{},
		reports:  newReports(declared.DiskCheck != ""),
		working:  newWorking(declared.DiskCheck),
		offering: &asking{},
	}

	cmd := exec.Command(declared.Command[0], declared.Command[1:]...)
	cmd.Dir = root
	if len(declared.Env) > 0 {
		cmd.Env = os.Environ()
		for name, value := range declared.Env {
			cmd.Env = append(cmd.Env, name+"="+value)
		}
	}
	// os/exec copies stderr into the tail and finishes the copy before Wait returns, or closes
	// the pipe draining after the process ends.
	cmd.Stderr = held.stderr
	cmd.WaitDelay = draining

	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: stdout: %w", declared.Name, err)
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: stdin: %w", declared.Name, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("lsp: %s: start: %w", declared.Name, err)
	}
	held.cmd = cmd

	client := answers{root: root, settings: declared.Settings, offering: held.offering}
	stream := ordered{
		Stream: jsonrpc2.NewStream(pipes{out: out, in: in}), reports: held.reports, working: held.working,
	}
	_, held.conn, held.asks = protocol.NewClient(context.WithoutCancel(ctx), client, stream)
	return held, nil
}

// launch is a started server whose handshake runs apart from the questions that wait for it.
// done is closed when the handshake ends, and err is the error of the handshake after that.
type launch struct {
	held  *session
	began time.Time
	done  chan struct{}
	err   error
}

// ordered is the stream of a session. It records the diagnostics that the server publishes
// and the work-done progress jobs that it reports when it reads each message, before the
// connection passes the message to a handler. So the records follow the order of the stream:
// a report is kept before the next message is read, such as the reply to a later request or
// the end of the job that produced the report.
//
// A handler cannot record them in that order. go.lsp.dev/protocol v1.0.1 wraps every handler
// in jsonrpc2.AsyncHandler, which releases the reader before the handler runs, so the handlers
// of two messages run concurrently.
type ordered struct {
	jsonrpc2.Stream
	reports *reports
	working *working
}

// Read returns the next message of the stream. It first records a publishDiagnostics
// notification, a $/progress notification and a window/workDoneProgress/create request. A
// message whose params do not decode is returned without a record.
//
// ordered does not implement the frame reader of the stream it wraps, so the connection reads
// every message through Read.
func (o ordered) Read(ctx context.Context) (jsonrpc2.Message, int64, error) {
	msg, n, err := o.Stream.Read(ctx)
	request, isRequest := msg.(jsonrpc2.RequestMessage)
	if err != nil || !isRequest {
		return msg, n, err
	}
	switch request.Method() {
	case protocol.MethodTextDocumentPublishDiagnostics:
		var params protocol.PublishDiagnosticsParams
		if protocol.Unmarshal(request.Params(), &params) == nil {
			o.reports.keep(params.URI, params.Diagnostics)
		}
	case protocol.MethodProgress:
		var params protocol.ProgressParams
		if protocol.Unmarshal(request.Params(), &params) == nil {
			o.working.progressed(&params)
		}
	case protocol.MethodWindowWorkDoneProgressCreate:
		var params protocol.WorkDoneProgressCreateParams
		if protocol.Unmarshal(request.Params(), &params) == nil {
			o.working.began(tokened(params.Token))
		}
	}
	return msg, n, err
}

// stop ends the server and returns the error of its shutdown.
//
// It sends shutdown and exit, closes the connection, and waits for the process. A server that
// has not exited [leaving] after the call, or when ctx is done, is killed. The wait for the
// stderr of the server ends [draining] after the process ends. Only the first call stops the
// session, and every later call returns nil. A shutdown refused because ctx ended is not an
// error.
func (s *session) stop(ctx context.Context) error {
	var refused error
	s.ends.Do(func() {
		saying, done := context.WithTimeout(ctx, leaving)
		defer done()

		refused = s.asks.Shutdown(saying)
		_ = s.asks.Exit(saying)
		_ = s.conn.Close()

		gone := make(chan error, 1)
		go func() { gone <- s.cmd.Wait() }()
		select {
		case <-gone:
		case <-saying.Done():
			_ = s.cmd.Process.Kill()
			<-gone
		}
	})

	if refused != nil &&
		!errors.Is(refused, context.Canceled) &&
		!errors.Is(refused, context.DeadlineExceeded) {
		return fmt.Errorf("lsp: shutdown: %w", refused)
	}
	return nil
}

// buffers returns a copy of the buffers of the server, by absolute path.
func (s *session) buffers() map[string]sent {
	s.opening.Lock()
	defer s.opening.Unlock()
	return maps.Clone(s.opened)
}

// withStderr returns err with the end of the server's stderr appended, or err when the server
// wrote nothing.
func (s *session) withStderr(err error) error {
	if written := s.stderr.String(); written != "" {
		return fmt.Errorf("%w: stderr: %s", err, written)
	}
	return err
}

// tail keeps the last [stderrSize] bytes written to it. It is safe for concurrent use.
type tail struct {
	mu   sync.Mutex
	kept []byte
}

// Write keeps the end of what has been written, and never returns an error.
func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.kept = append(t.kept, p...)
	if over := len(t.kept) - stderrSize; over > 0 {
		t.kept = append(t.kept[:0], t.kept[over:]...)
	}
	return len(p), nil
}

// String returns the kept bytes as valid UTF-8, without leading and trailing white space.
func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(strings.ToValidUTF8(string(t.kept), "�"))
}

// pipes joins the stdout and the stdin of a process into the one stream that the protocol is
// framed over.
type pipes struct {
	out io.ReadCloser
	in  io.WriteCloser
}

func (p pipes) Read(into []byte) (int, error)  { return p.out.Read(into) }
func (p pipes) Write(from []byte) (int, error) { return p.in.Write(from) }

// Close closes stdin and then stdout, and returns both errors joined. A server that reads
// stdin ends its input when stdin closes.
func (p pipes) Close() error { return errors.Join(p.in.Close(), p.out.Close()) }
