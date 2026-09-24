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

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

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

// reply reads frames until the reply to the request id, and returns its result.
func (p *process) reply(t *testing.T, id int) json.RawMessage {
	t.Helper()
	for {
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

		var message struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
		}
		assert.NoError(t, json.Unmarshal(body, &message), "the body of a frame")
		if message.Method == "" && message.ID != nil && *message.ID == id {
			return message.Result
		}
	}
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
	})
}
