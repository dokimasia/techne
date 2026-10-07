// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
	"go.lsp.dev/uri"
)

// publishing is the method of the notification with the diagnostics of a document, and
// progressing the method of the notification of a work-done progress job.
const (
	publishing  = "textDocument/publishDiagnostics"
	progressing = "$/progress"
)

// The kinds of work-done progress value that begin and end a job.
const (
	beginning = "begin"
	ending    = "end"
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

// frame is one message of the scripted server: a reply to a request of the test, a
// notification with the diagnostics of a document, or a notification of a progress job.
type frame struct {
	ID     *int            `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Params struct {
		Diagnostics json.RawMessage `json:"diagnostics"`
		Token       string          `json:"token"`
		Value       struct {
			Kind string `json:"kind"`
		} `json:"value"`
	} `json:"params"`
}

// of reports whether m is a progress notification of kind for the check on disk.
func (m frame) of(kind string) bool {
	return m.Method == progressing && m.Params.Token == lsptest.DiskToken && m.Params.Value.Kind == kind
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

// check reads frames until the end of a check on disk, and returns the diagnostics of each
// report after the begin of the check, in order.
func (p *process) check(t *testing.T) []string {
	t.Helper()
	var out []string
	for begun := false; ; {
		m := p.next(t)
		switch {
		case m.of(beginning):
			begun = true
		case m.of(ending):
			return out
		case begun && m.Method == publishing:
			out = append(out, string(m.Params.Diagnostics))
		}
	}
}

// jobs reads frames until the reply to the request id, and returns the kinds of the
// notifications of the check on disk before the reply, in order.
func (p *process) jobs(t *testing.T, id int) []string {
	t.Helper()
	var out []string
	for {
		m := p.next(t)
		switch {
		case m.of(beginning), m.of(ending):
			out = append(out, m.Params.Value.Kind)
		case m.Method == "" && m.ID != nil && *m.ID == id:
			return out
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

// asking is the textDocument/documentSymbol request of a.fake that a test sends after
// initialize, and asked is its id, the id after the id 1 of initialize.
const (
	asking = `{"jsonrpc":"2.0","id":2,"method":"textDocument/documentSymbol","params":` +
		`{"textDocument":{"uri":"file:///tmp/a.fake"}}}`
	asked = 2
)

// defining returns the textDocument/definition request of a.fake at a character of line 2, the
// line of the imports of [importing] and [naming], with the id [asked].
func defining(character int) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"textDocument/definition","params":`+
		`{"textDocument":{"uri":"file:///tmp/a.fake"},"position":{"line":2,"character":%d}}}`,
		asked, character)
}

// located returns the URIs of the locations of the reply to the request [asked].
func (p *process) located(t *testing.T) []string {
	t.Helper()
	var got []struct {
		URI string `json:"uri"`
	}
	assert.NoError(t, json.Unmarshal(p.reply(t, asked), &got), "the reply to definition")
	out := make([]string, 0, len(got))
	for _, one := range got {
		out = append(out, one.URI)
	}
	return out
}

// initialized is the notification that follows the reply to initialize. saving is the didSave
// notification of a.fake.
const (
	initialized = `{"jsonrpc":"2.0","method":"initialized","params":{}}`
	saving      = `{"jsonrpc":"2.0","method":"textDocument/didSave","params":{"textDocument":` +
		`{"uri":"file:///tmp/a.fake"}}}`
)

// initialize sends initialize with the root /tmp and returns the capabilities of the reply.
func (p *process) initialize(t *testing.T) map[string]any {
	t.Helper()
	return p.initializeAt(t, "/tmp")
}

// initializeAt sends initialize with the root at the absolute path root and returns the
// capabilities of the reply.
func (p *process) initializeAt(t *testing.T, root string) map[string]any {
	t.Helper()
	p.send(t, fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"rootUri":"file://%s",`+
		`"capabilities":{}}}`, root))
	var result struct {
		Capabilities map[string]any `json:"capabilities"`
	}
	assert.NoError(t, json.Unmarshal(p.reply(t, 1), &result), "the reply to initialize")
	return result.Capabilities
}

// placed writes content to a.fake in a new directory and returns the directory.
func placed(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(root, "a"+lsptest.Extension), []byte(content), 0o644),
		"the test writes a.fake")
	return root
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
			exit := assert.ErrorAs[*exec.ExitError](t, p.cmd.Wait(), "Wait returns an exit error")
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

		t.Run("ranges each name of a var line over the line after var in the Aims mode", func(t *testing.T) {
			t.Parallel()
			type place struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			}
			type ranged struct {
				Start place `json:"start"`
				End   place `json:"end"`
			}
			type described struct {
				Name      string `json:"name"`
				Range     ranged `json:"range"`
				Selection ranged `json:"selectionRange"`
			}
			on := func(line, from, to int) ranged { return ranged{place{line, from}, place{line, to}} }
			p := run(t, lsptest.Aims)
			p.initialize(t)
			p.send(t, opening(lsptest.Pair))
			p.send(t, asking)
			var got []described
			assert.NoError(t, json.Unmarshal(p.reply(t, asked), &got), "the reply to documentSymbol")
			assert.Equal(t, got, []described{
				{Name: "s", Range: on(2, 4, 15), Selection: on(2, 4, 5)},
				{Name: "t", Range: on(2, 4, 15), Selection: on(2, 7, 8)},
				{Name: "Sum", Range: on(4, 0, 31), Selection: on(4, 5, 8)},
			}, "the symbols of Pair")
		})

		t.Run("responds at the end of an import name with the file of the name in the Resolves mode",
			func(t *testing.T) {
				t.Parallel()
				root := lsptest.Workspace(t, map[string]string{"a/store.fake": "package a\n"})
				p := run(t, lsptest.Resolves)
				p.initializeAt(t, root)
				p.send(t, opening(importing))
				p.send(t, defining(20))
				assert.Equal(t, p.located(t), []string{string(uri.File(filepath.Join(root, "a", "store.fake")))},
					"the definition at the end of a/store")
			})

		t.Run("responds at the qualifier of an import name with no location in the Resolves mode",
			func(t *testing.T) {
				t.Parallel()
				root := lsptest.Workspace(t, map[string]string{"a/store.fake": "package a\n"})
				p := run(t, lsptest.Resolves)
				p.initializeAt(t, root)
				p.send(t, opening(importing))
				p.send(t, defining(14))
				assert.Empty(t, p.located(t), "the definition at the qualifier of a/store")
			})

		t.Run("responds at the end of an import name with no location before LateStart in the ResolvesLate mode",
			func(t *testing.T) {
				t.Parallel()
				root := lsptest.Workspace(t, map[string]string{"a/store.fake": "package a\n"})
				p := run(t, lsptest.ResolvesLate)
				p.initializeAt(t, root)
				p.send(t, opening(importing))
				p.send(t, defining(20))
				assert.Empty(t, p.located(t), "the definition at the end of a/store")
			})

		t.Run("responds at the end of an import name with its file from LateStart in the ResolvesLate mode",
			func(t *testing.T) {
				t.Parallel()
				root := lsptest.Workspace(t, map[string]string{"a/store.fake": "package a\n"})
				p := run(t, lsptest.ResolvesLate)
				p.initializeAt(t, root)
				p.send(t, opening(importing))
				time.Sleep(lsptest.LateStart)
				p.send(t, defining(20))
				assert.Equal(t, p.located(t), []string{string(uri.File(filepath.Join(root, "a", "store.fake")))},
					"the definition at the end of a/store")
			})

		t.Run("responds at the end of the module of a from line with its file in the Resolves mode",
			func(t *testing.T) {
				t.Parallel()
				root := lsptest.Workspace(t, map[string]string{"a/store.fake": "package a\n"})
				p := run(t, lsptest.Resolves)
				p.initializeAt(t, root)
				p.send(t, opening(naming))
				p.send(t, defining(11))
				assert.Equal(t, p.located(t), []string{string(uri.File(filepath.Join(root, "a", "store.fake")))},
					"the definition at the end of a/store")
			})

		t.Run("responds at the name of a from line with the name in the Resolves mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Resolves)
			p.initializeAt(t, lsptest.Workspace(t, map[string]string{"a/store.fake": "package a\n"}))
			p.send(t, opening(naming))
			p.send(t, defining(24))
			var got []struct {
				URI   string `json:"uri"`
				Range struct {
					Start struct {
						Character int `json:"character"`
					} `json:"start"`
				} `json:"range"`
			}
			assert.NoError(t, json.Unmarshal(p.reply(t, asked), &got), "the reply to definition")
			assert.Length(t, got, 1, "the definitions at value")
			assert.Equal(t, got[0].URI, "file:///tmp/a.fake", "the file of the definition at value")
			assert.Equal(t, got[0].Range.Start.Character, 20, "the start of the definition at value")
		})

		t.Run("publishes the report of an opened document after QuietDelay in the Quiet mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Quiet)
			p.initialize(t)
			sent := time.Now()
			p.send(t, opening(lsptest.Faulty))
			got := p.report(t)
			assert.InRange(t, time.Since(sent), float64(lsptest.QuietDelay), 1<<63, "the delay of the report")
			assert.Contains(t, got, lsptest.Broken, "the report of the opened document")
		})

		t.Run("publishes no report of a change that keeps the report empty in the Quiet mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Quiet)
			p.initialize(t)
			p.send(t, opening(lsptest.Content))
			assert.Equal(t, p.report(t), "[]", "the report of the opened document")
			p.send(t, changing(lsptest.Content+"\n", 2))
			p.send(t, asking)
			assert.Empty(t, p.reports(t, asked), "the reports of the change")
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
			assert.InRange(t, time.Since(sent), float64(lsptest.QuietClose), 1<<63, "the delay of the report")
		})

		t.Run("publishes an empty report of a closed document in the Quiet mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Quiet)
			p.initialize(t)
			p.send(t, opening(lsptest.Faulty))
			p.report(t)
			p.send(t, closing)
			p.send(t, asking)
			assert.Equal(t, p.reports(t, asked), []string{"[]"}, "the reports of the close")
		})

		t.Run("checks the files on disk after initialized in the DiskChecks mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.DiskChecks)
			p.initializeAt(t, placed(t, lsptest.Unsound+"\n"))
			sent := time.Now()
			p.send(t, initialized)
			got := strings.Join(p.check(t), "")
			assert.InRange(t, time.Since(sent), float64(lsptest.DiskDelay), 1<<63, "the delay of the check")
			assert.That(t, got).
				Contains(lsptest.Unsound, "the report of the check").
				Contains("checks=1", "the note of the check")
		})

		t.Run("checks the files on disk after didSave in the DiskChecks mode", func(t *testing.T) {
			t.Parallel()
			root := placed(t, lsptest.Content)
			p := run(t, lsptest.DiskChecks)
			p.initializeAt(t, root)
			p.send(t, initialized)
			assert.NotContains(t, strings.Join(p.check(t), ""), lsptest.Unsound, "the report of the first check")

			assert.NoError(t, os.WriteFile(filepath.Join(root, "a"+lsptest.Extension), []byte(lsptest.Unsound+"\n"),
				0o644), "the test writes a.fake")
			p.send(t, saving)
			got := strings.Join(p.check(t), "")
			assert.That(t, got).
				Contains(lsptest.Unsound, "the report of the check after the save").
				Contains("checks=2", "the note of the check after the save")
		})

		t.Run("begins a check on disk that never ends in the DiskStuck mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.DiskStuck)
			p.initializeAt(t, placed(t, lsptest.Content))
			p.send(t, initialized)
			p.send(t, asking)
			assert.Equal(t, p.jobs(t, asked), []string{beginning}, "the notifications of the check before the reply")
		})
	})
}
