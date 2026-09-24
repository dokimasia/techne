// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// publishing is the method of the notification with the diagnostics of a document.
const publishing = "textDocument/publishDiagnostics"

// process is one run of the scripted server, driven frame by frame.
type process struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    *bufio.Reader
	stderr strings.Builder
}

// run starts the scripted server in mode as a process of this test binary.
func run(t *testing.T, mode lsptest.Mode) *process {
	t.Helper()
	server := lsptest.Server(mode)
	p := &process{cmd: exec.Command(server.Command[0], server.Command[1:]...)}
	p.cmd.Dir = t.TempDir()
	p.cmd.Env = os.Environ()
	for name, value := range server.Env {
		p.cmd.Env = append(p.cmd.Env, name+"="+value)
	}
	p.cmd.Stderr = &p.stderr
	in, err := p.cmd.StdinPipe()
	assert.NoError(t, err, "the stdin of the scripted server")
	out, err := p.cmd.StdoutPipe()
	assert.NoError(t, err, "the stdout of the scripted server")
	p.in, p.out = in, bufio.NewReader(out)
	assert.NoError(t, p.cmd.Start(), "the start of the scripted server")
	t.Cleanup(func() {
		_ = p.in.Close()
		_ = p.cmd.Process.Kill()
		_ = p.cmd.Wait()
	})
	return p
}

// send writes one framed message.
func (p *process) send(t *testing.T, body string) {
	t.Helper()
	_, err := fmt.Fprintf(p.in, "Content-Length: %d\r\n\r\n%s", len(body), body)
	assert.NoError(t, err, "the test writes a frame")
}

// frame is one message of the scripted server. It is either a reply to a request of the test
// or a notification with the diagnostics of a document.
type frame struct {
	ID     *int            `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Params struct {
		Diagnostics json.RawMessage `json:"diagnostics"`
	} `json:"params"`
}

// next reads one framed message.
func (p *process) next(t *testing.T) frame {
	t.Helper()
	length := -1
	for {
		header, err := p.out.ReadString('\n')
		assert.NoError(t, err, "the test reads a header")
		header = strings.TrimRight(header, "\r\n")
		if header == "" {
			break
		}
		if value, found := strings.CutPrefix(header, "Content-Length: "); found {
			length, err = strconv.Atoi(value)
			assert.NoError(t, err, "the Content-Length of a frame")
		}
	}
	body := make([]byte, length)
	_, err := io.ReadFull(p.out, body)
	assert.NoError(t, err, "the test reads a body")
	var out frame
	assert.NoError(t, json.Unmarshal(body, &out), "the body of a frame")
	return out
}

// reply reads frames until the reply to the request id, and returns its result.
func (p *process) reply(t *testing.T, id int) json.RawMessage {
	t.Helper()
	for {
		if m := p.next(t); m.Method == "" && m.ID != nil && *m.ID == id {
			return m.Result
		}
	}
}

// reports reads frames until the reply to the request id, and returns the diagnostics of each
// report before the reply, in order.
func (p *process) reports(t *testing.T, id int) []string {
	t.Helper()
	var out []string
	for {
		m := p.next(t)
		switch {
		case m.Method == publishing:
			out = append(out, string(m.Params.Diagnostics))
		case m.Method == "" && m.ID != nil && *m.ID == id:
			return out
		}
	}
}

// report reads frames until a report, and returns its diagnostics.
func (p *process) report(t *testing.T) string {
	t.Helper()
	for {
		if m := p.next(t); m.Method == publishing {
			return string(m.Params.Diagnostics)
		}
	}
}

// opening is the didOpen notification of a.fake with text, in the root of [process.initialize].
func opening(text string) string {
	quoted, _ := json.Marshal(text)
	return fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":`+
		`{"uri":"file:///tmp/a.fake","languageId":"fake","version":1,"text":%s}}}`, quoted)
}

// changing is the didChange notification that replaces the buffer of a.fake with text under
// version.
func changing(text string, version int) string {
	quoted, _ := json.Marshal(text)
	return fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":`+
		`{"uri":"file:///tmp/a.fake","version":%d},"contentChanges":[{"text":%s}]}}`, version, quoted)
}

// closing is the didClose notification of a.fake.
const closing = `{"jsonrpc":"2.0","method":"textDocument/didClose","params":{"textDocument":` +
	`{"uri":"file:///tmp/a.fake"}}}`

// asking is a textDocument/documentSymbol request of a.fake with the id id.
func asking(id int) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"textDocument/documentSymbol","params":`+
		`{"textDocument":{"uri":"file:///tmp/a.fake"}}}`, id)
}

// initialize sends initialize and returns the capabilities of the reply.
func (p *process) initialize(t *testing.T) map[string]any {
	t.Helper()
	p.send(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"rootUri":"file:///tmp",`+
		`"capabilities":{}}}`)
	var result struct {
		Capabilities map[string]any `json:"capabilities"`
	}
	assert.NoError(t, json.Unmarshal(p.reply(t, 1), &result), "the reply to initialize")
	return result.Capabilities
}

func TestScript(t *testing.T) {
	t.Parallel()

	t.Run("Main", func(t *testing.T) {
		t.Parallel()

		t.Run("offers pull diagnostics in the Default mode", func(t *testing.T) {
			t.Parallel()
			capabilities := run(t, lsptest.Default).initialize(t)
			assert.NotEmpty(t, capabilities["diagnosticProvider"], "the diagnostic provider")
		})

		t.Run("offers two requests in the Thin mode", func(t *testing.T) {
			t.Parallel()
			capabilities := run(t, lsptest.Thin).initialize(t)
			assert.Length(t, capabilities, 2, "the capabilities of the Thin mode")
		})

		t.Run("offers workspace diagnostics in the WorkspaceDiagnostics mode", func(t *testing.T) {
			t.Parallel()
			capabilities := run(t, lsptest.WorkspaceDiagnostics).initialize(t)
			provider, _ := capabilities["diagnosticProvider"].(map[string]any)
			assert.Equal(t, provider["workspaceDiagnostics"], any(true), "workspaceDiagnostics of the provider")
		})

		t.Run("exits with status 3 in the Dies mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Dies)
			p.send(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}`)
			var exit *exec.ExitError
			assert.True(t, errors.As(p.cmd.Wait(), &exit), "Wait returns an exit error")
			assert.Equal(t, exit.ExitCode(), 3, "the exit status of the Dies mode")
			assert.Contains(t, p.stderr.String(), lsptest.Dying, "the stderr of the Dies mode")
		})

		t.Run("exits with status 0 after exit", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Default)
			p.initialize(t)
			p.send(t, `{"jsonrpc":"2.0","method":"exit"}`)
			assert.NoError(t, p.cmd.Wait(), "Wait after exit")
		})

		t.Run("publishes the report of an opened document after QuietDelay in the Quiet mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Quiet)
			p.initialize(t)
			sent := time.Now()
			p.send(t, opening(lsptest.Faulty))
			got := p.report(t)
			assert.True(t, time.Since(sent) >= lsptest.QuietDelay, "the delay of the report")
			assert.Contains(t, got, lsptest.Broken, "the report of the opened document")
		})

		t.Run("publishes no report of a change that keeps the report empty in the Quiet mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Quiet)
			p.initialize(t)
			p.send(t, opening(lsptest.Content))
			assert.Equal(t, p.report(t), "[]", "the report of the opened document")
			p.send(t, changing(lsptest.Content+"\n", 2))
			p.send(t, asking(2))
			assert.Empty(t, p.reports(t, 2), "the reports of the change")
		})

		t.Run("publishes the report of a closed document after QuietClose in the Quiet mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Quiet)
			p.initialize(t)
			p.send(t, opening(lsptest.Content))
			p.report(t)
			sent := time.Now()
			p.send(t, closing)
			p.report(t)
			assert.True(t, time.Since(sent) >= lsptest.QuietClose, "the delay of the report")
		})

		t.Run("publishes an empty report of a closed document in the Quiet mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Quiet)
			p.initialize(t)
			p.send(t, opening(lsptest.Faulty))
			p.report(t)
			p.send(t, closing)
			p.send(t, asking(2))
			assert.Equal(t, p.reports(t, 2), []string{"[]"}, "the reports of the close")
		})
	})
}
