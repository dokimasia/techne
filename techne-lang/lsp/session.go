// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// session is one running language server and the connection to it.
//
// The process, the framing and the dispatcher share a lifetime. A
// connection outliving its process answers nothing and a process
// outliving its connection is leaked, so they are started together in
// [start] and stopped together in [session.stop].
type session struct {
	cmd  *exec.Cmd
	conn jsonrpc2.Conn

	// asks is the server as something to call: one method per request
	// the specification defines, each decoding into the union that
	// specification names for it.
	asks protocol.Server

	// capable is what the server said it can do, at initialise. A role
	// reads it rather than trying a request to find out: a server
	// answering "method not found" is indistinguishable from one
	// answering that there is nothing to report.
	capable protocol.ServerCapabilities

	// ends is what makes stopping twice do nothing. A close racing the
	// composition root's own would otherwise wait on a process already
	// reaped, which never returns.
	ends sync.Once
}

// starting is how long a server is given to answer the handshake.
//
// Separate from [Server.Loading], which is how long it may take to read
// the workspace after answering. This is the answer itself, and a server
// that does not give one is not going to: the connection came up, so the
// program is there and it is not talking.
//
// Bounded because the alternative is unbounded. A server that accepts
// the connection and never answers initialise held a question for as
// long as its caller allowed — measured against
// typescript-language-server over a repository whose TypeScript version
// it refuses, fourteen seconds of an agent's turn, spent on a server
// that was never going to answer. The failure is remembered, so it is
// paid once per session either way; what this decides is how much.
const starting = 10 * time.Second

// leaving is how long the protocol's own shutdown is given before the
// process is killed instead.
//
// Bounded for the same reason [starting] is, and against the same
// failure: shutdown is a request, and a server that does not answer
// requests does not answer this one either. Unbounded it held a caller
// for as long as it allowed, which is what [session.stop] exists to
// prevent for the process and had not been doing for the conversation.
//
// Long enough for a server that is working to write out what it was
// holding, which is why the whole sequence is attempted rather than
// killing outright.
const leaving = 5 * time.Second

// start runs a server and brings up the connection to it.
//
// The command is run rather than looked for. Whether it exists is
// settled before this is reached, so a language whose server is not
// installed is reported as unavailable rather than as a call that
// failed.
func start(ctx context.Context, declared Server, root string, answers protocol.Client) (*session, error) {
	if len(declared.Command) == 0 {
		return nil, fmt.Errorf("lsp: %s: no command to run", declared.Name)
	}

	// The server outlives the call that happened to be first. Tied to
	// that call's context it would be killed when an outline was
	// cancelled, and the next question would pay the start again.
	live := context.WithoutCancel(ctx)

	cmd := exec.CommandContext(live, declared.Command[0], declared.Command[1:]...)
	cmd.Dir = root
	if len(declared.Env) > 0 {
		cmd.Env = os.Environ()
		for name, value := range declared.Env {
			cmd.Env = append(cmd.Env, name+"="+value)
		}
	}
	// A server's own logging is not this process's to print, and one
	// writing a great deal into a pipe nobody reads blocks on it. Given
	// to os/exec rather than drained here, so the copy is finished
	// before the process is reaped rather than racing it.
	cmd.Stderr = io.Discard

	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: %s stdout: %w", declared.Name, err)
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: %s stdin: %w", declared.Name, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("lsp: start %s: %w", declared.Name, err)
	}

	_, conn, asks := protocol.NewClient(live, answers, jsonrpc2.NewStream(pipes{out: out, in: in}))
	return &session{cmd: cmd, conn: conn, asks: asks}, nil
}

// stop ends the server, and kills it if it will not end.
//
// In the order the pieces depend on each other. The protocol's own
// sequence first, because a server told to shut down writes out what it
// was holding. Then the connection, which is what unblocks a read parked
// mid-frame and does not return until the read has stopped. The process
// last: waiting on it while its output is still being read closes the
// pipe under the reader.
//
// A server still running when the context is done is killed. One that
// will not exit is leaked for the life of the parent, and a tool that
// leaks one per language per run is unusable.
func (s *session) stop(ctx context.Context) error {
	var refused error
	s.ends.Do(func() {
		// Bounded rather than the caller's own, so a server that stopped
		// answering cannot hold the caller here. A context already done
		// keeps its own deadline, so cancelling still kills at once.
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

	// A server that would not shut down was killed, which is this
	// package's business and not the caller's.
	if refused != nil &&
		!errors.Is(refused, context.Canceled) &&
		!errors.Is(refused, context.DeadlineExceeded) {
		return fmt.Errorf("lsp: shutdown: %w", refused)
	}
	return nil
}

// pipes is a process's stdin and stdout as the one stream the protocol
// is framed over.
//
// The conversation is duplex and a subprocess offers two half-open
// pipes, so they are joined here rather than in whatever reads or
// writes.
type pipes struct {
	out io.ReadCloser
	in  io.WriteCloser
}

func (p pipes) Read(into []byte) (int, error)  { return p.out.Read(into) }
func (p pipes) Write(from []byte) (int, error) { return p.in.Write(from) }

// Close shuts both halves and reports whatever went wrong with either.
//
// Closing the write half is what tells a server reading its stdin that
// there is nothing more coming, so both are closed even when the first
// fails.
func (p pipes) Close() error { return errors.Join(p.in.Close(), p.out.Close()) }
