// Copyright Dokimasia B.V. 2026
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
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
	"go.lsp.dev/uri"
)

// publishing is the method of the notification with the diagnostics of a document, progressing
// the method of the notification of a work-done progress job, and creating the method of the
// request that creates the token of a job.
const (
	publishing  = "textDocument/publishDiagnostics"
	progressing = "$/progress"
	creating    = "window/workDoneProgress/create"
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

// run starts the scripted server in mode with options as a process of this test binary.
func run(t *testing.T, mode lsptest.Mode, options ...lsptest.Option) *process {
	t.Helper()
	server := lsptest.Server(mode, options...)
	p := &process{cmd: exec.CommandContext(t.Context(), server.Command[0], server.Command[1:]...)}
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
// notification with the diagnostics of a document, a request that creates the token of a
// progress job, or a notification of a progress job.
type frame struct {
	ID     *int            `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Params struct {
		Diagnostics json.RawMessage `json:"diagnostics"`
		Token       string          `json:"token"`
		Value       struct {
			Kind  string `json:"kind"`
			Title string `json:"title"`
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

// reports reads frames until the reply to the request [asked], and returns the diagnostics of
// each report before the reply, in order.
func (p *process) reports(t *testing.T) []string {
	t.Helper()
	var out []string
	for {
		m := p.next(t)
		switch {
		case m.Method == publishing:
			out = append(out, string(m.Params.Diagnostics))
		case m.Method == "" && m.ID != nil && *m.ID == asked:
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

// progress reads frames until the reply to the request id, and returns, in order, the method of
// each request before the reply that creates the token of a job, and the kind of each
// notification of a job before the reply.
func (p *process) progress(t *testing.T, id int) []string {
	t.Helper()
	var out []string
	for {
		m := p.next(t)
		switch {
		case m.Method == creating:
			out = append(out, m.Method)
		case m.Method == progressing:
			out = append(out, m.Params.Value.Kind)
		case m.Method == "" && m.ID != nil && *m.ID == id:
			return out
		}
	}
}

// diagnosed reads frames until the end of the first diagnosis, a job whose title starts with
// [lsptest.DiagnosisPrefix], and returns the diagnostics of each report after the begin of the
// diagnosis, in order. It reads on while the server ends no diagnosis.
func (p *process) diagnosed(t *testing.T) []string {
	t.Helper()
	var out []string
	for token := ""; ; {
		m := p.next(t)
		switch {
		case token == "" && m.Method == progressing && m.Params.Value.Kind == beginning &&
			strings.HasPrefix(m.Params.Value.Title, lsptest.DiagnosisPrefix):
			token = m.Params.Token
		case token != "" && m.Method == progressing && m.Params.Value.Kind == ending && m.Params.Token == token:
			return out
		case token != "" && m.Method == publishing:
			out = append(out, string(m.Params.Diagnostics))
		}
	}
}

// opening is the didOpen notification of a.fake with text, in the root of [process.initialize].
func opening(text string) string { return openingOf("a.fake", text) }

// openingOf is the didOpen notification of the file name with text, in the root of
// [process.initialize].
func openingOf(name, text string) string {
	quoted, _ := json.Marshal(text)
	return fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":`+
		`{"uri":"file:///tmp/%s","languageId":"fake","version":1,"text":%s}}}`, name, quoted)
}

// openingAt is the didOpen notification of the file at the absolute path full with text.
func openingAt(full, text string) string {
	quoted, _ := json.Marshal(text)
	return fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":`+
		`{"uri":%q,"languageId":"fake","version":1,"text":%s}}}`, string(uri.File(full)), quoted)
}

// projecting is the workspace/executeCommand request with the id [asked] that asks tsserver for
// the project of the file at the absolute path full, as techne asks typescript-language-server.
func projecting(full string) string {
	quoted, _ := json.Marshal(full)
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"workspace/executeCommand","params":`+
		`{"command":"typescript.tsserverRequest","arguments":["projectInfo",{"file":%s}]}}`,
		asked, quoted)
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

// referencing is the textDocument/references request at the name Store of a.fake, with the id
// [asked].
const referencing = `{"jsonrpc":"2.0","id":2,"method":"textDocument/references","params":` +
	`{"textDocument":{"uri":"file:///tmp/a.fake"},"position":{"line":2,"character":5},` +
	`"context":{"includeDeclaration":false}}}`

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

// loadMargin is how long a test waits past [lsptest.LoadTime] before it reads whether the loading
// job ended, so the end of a job that ends at LoadTime arrives before the reply of a later request.
const loadMargin = 500 * time.Millisecond

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

// initializeAt sends initialize with the root at the absolute path root and the capabilities
// [publishes], by the rule of [initializing], and returns the capabilities of the reply.
func (p *process) initializeAt(t *testing.T, root string) map[string]any {
	t.Helper()
	p.send(t, initializing(root, publishes))
	var result struct {
		Capabilities map[string]any `json:"capabilities"`
	}
	assert.NoError(t, json.Unmarshal(p.reply(t, 1), &result), "the reply to initialize")
	return result.Capabilities
}

// publishes are the client capabilities that declare textDocument.publishDiagnostics, as techne
// declares it, and no other capability.
const publishes = `{"textDocument":{"publishDiagnostics":{}}}`

// initializing is the initialize request with the id 1, the root at the absolute path root, and
// capabilities, an object of client capabilities written as JSON.
func initializing(root, capabilities string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"rootUri":%q,`+
		`"capabilities":%s}}`, string(uri.File(root)), capabilities)
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

		t.Run("responds to a definition after the lag of Lagging", func(t *testing.T) {
			t.Parallel()
			lag := 100 * time.Millisecond
			p := run(t, lsptest.Default, lsptest.Lagging(lag))
			p.initialize(t)
			p.send(t, opening(lsptest.Content))
			sent := time.Now()
			p.send(t, defining(5))
			p.reply(t, asked)
			// The clock of Go on Windows advances in steps of about 16 ms, so the test takes half the
			// lag as the least wait.
			assert.InRange(t, time.Since(sent), float64(lag/2), 1<<63, "the time to the reply")
		})

		t.Run("responds to projectInfo with the tsconfig.json of a project with a buffer in the Projects mode",
			func(t *testing.T) {
				t.Parallel()
				root := lsptest.Workspace(t, map[string]string{
					"project/tsconfig.json": "{}\n", "project/x.fake": lsptest.Content,
				})
				x := filepath.Join(root, "project", "x.fake")
				p := run(t, lsptest.Projects)
				p.initializeAt(t, root)
				p.send(t, openingAt(x, lsptest.Content))
				p.send(t, projecting(x))
				var got struct {
					Body struct {
						ConfigFileName string `json:"configFileName"`
					} `json:"body"`
				}
				assert.NoError(t, json.Unmarshal(p.reply(t, asked), &got), "the reply to projectInfo")
				assert.Equal(t, got.Body.ConfigFileName, filepath.Join(root, "project", "tsconfig.json"),
					"the configuration file of the project of x.fake")
			})

		t.Run("responds to projectInfo with no project for a project without a buffer in the Projects mode",
			func(t *testing.T) {
				t.Parallel()
				root := lsptest.Workspace(t, map[string]string{
					"project/tsconfig.json": "{}\n", "project/x.fake": lsptest.Content, "other/b.fake": lsptest.Content,
				})
				p := run(t, lsptest.Projects)
				p.initializeAt(t, root)
				p.send(t, openingAt(filepath.Join(root, "other", "b.fake"), lsptest.Content))
				p.send(t, projecting(filepath.Join(root, "project", "x.fake")))
				assert.Empty(t, p.reply(t, asked), "the result of projectInfo")
			})

		t.Run("responds to projectInfo with no project for a file without a tsconfig.json in the Projects mode",
			func(t *testing.T) {
				t.Parallel()
				root := lsptest.Workspace(t, map[string]string{"x.fake": lsptest.Content})
				x := filepath.Join(root, "x.fake")
				p := run(t, lsptest.Projects)
				p.initializeAt(t, root)
				p.send(t, openingAt(x, lsptest.Content))
				p.send(t, projecting(x))
				assert.Empty(t, p.reply(t, asked), "the result of projectInfo")
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
			assert.Empty(t, p.reports(t), "the reports of the change")
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
			assert.Equal(t, p.reports(t), []string{"[]"}, "the reports of the close")
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
			assert.Equal(t, p.progress(t, asked), []string{beginning}, "the messages of jobs before the reply")
		})

		t.Run("publishes an empty report and then the report of the buffer in the Diagnoses mode",
			func(t *testing.T) {
				t.Parallel()
				p := run(t, lsptest.Diagnoses)
				p.initialize(t)
				p.send(t, opening(lsptest.Faulty))
				got := p.diagnosed(t)
				assert.Length(t, got, 2, "the reports of the diagnosis")
				expect.Equal(t, got[0], "[]", "the first report of the diagnosis")
				expect.Contains(t, got[1], lsptest.Broken, "the second report of the diagnosis")
			})

		t.Run("ends a diagnosis DiagnosisDelay and DiagnosisTime after a change in the Diagnoses mode",
			func(t *testing.T) {
				t.Parallel()
				p := run(t, lsptest.Diagnoses)
				p.initialize(t)
				sent := time.Now()
				p.send(t, opening(lsptest.Content))
				p.diagnosed(t)
				assert.InRange(t, time.Since(sent), float64(lsptest.DiagnosisDelay+lsptest.DiagnosisTime), 1<<63,
					"the time of the diagnosis")
			})

		t.Run("begins the diagnosis of a change before the reply to a later request in the Diagnoses mode",
			func(t *testing.T) {
				t.Parallel()
				p := run(t, lsptest.Diagnoses)
				p.initialize(t)
				p.send(t, changing(lsptest.Content, 2))
				p.send(t, asking)
				assert.Equal(
					t,
					p.progress(t, asked),
					[]string{creating, beginning},
					"the messages of jobs before the reply",
				)
			})

		t.Run("begins a diagnosis that never ends in the DiagnosisStuck mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.DiagnosisStuck)
			p.initialize(t)
			p.send(t, initialized)
			p.send(t, asking)
			assert.Equal(
				t,
				p.progress(t, asked),
				[]string{creating, beginning},
				"the messages of jobs before the reply",
			)
		})

		t.Run("replies to a request after a diagnosis ends in the Diagnoses mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Diagnoses)
			p.initialize(t)
			p.send(t, opening(lsptest.Content))
			p.diagnosed(t)
			p.send(t, asking)
			assert.NotEmpty(t, p.reply(t, asked), "the reply to documentSymbol")
		})

		{
			tests := []struct {
				name string
				give lsptest.Mode
				want []string
			}{
				{"sends no job message during initialize in the Default mode", lsptest.Default, nil},
				{
					"creates and begins the loading job during initialize in the Loading mode", lsptest.Loading,
					[]string{creating, beginning},
				},
				{
					"creates and begins the loading job during initialize in the Stuck mode", lsptest.Stuck,
					[]string{creating, beginning},
				},
				{"creates the loading job during initialize in the Created mode", lsptest.Created, []string{creating}},
				{
					"creates and begins the loading job during initialize in the Diagnoses mode", lsptest.Diagnoses,
					[]string{creating, beginning},
				},
				{"begins its first job during initialize in the Burst mode", lsptest.Burst, []string{beginning}},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					p := run(t, tt.give)
					p.send(t, initializing("/tmp", publishes))
					assert.Equal(t, p.progress(t, 1), tt.want, "the messages of jobs before the reply to initialize")
				})
			}
		}

		{
			tests := []struct {
				name string
				give lsptest.Mode
				want []string
			}{
				{
					"ends the loading job LoadTime after initialize in the Loading mode", lsptest.Loading,
					[]string{ending},
				},
				{
					"ends the loading job LoadTime after initialize in the Diagnoses mode", lsptest.Diagnoses,
					[]string{ending},
				},
				{"keeps the loading job open past LoadTime in the Stuck mode", lsptest.Stuck, nil},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					p := run(t, tt.give)
					p.initialize(t)
					time.Sleep(lsptest.LoadTime + loadMargin)
					p.send(t, asking)
					assert.Equal(t, p.progress(t, asked), tt.want, "the messages of jobs before the reply")
				})
			}
		}

		t.Run("ends its first job and begins and ends its second after initialize in the Burst mode",
			func(t *testing.T) {
				t.Parallel()
				p := run(t, lsptest.Burst)
				p.initialize(t)
				time.Sleep(2*lsptest.BurstJob + lsptest.BurstGap + loadMargin)
				p.send(t, asking)
				assert.Equal(t, p.progress(t, asked), []string{ending, beginning, ending},
					"the messages of jobs before the reply")
			})

		{
			tests := []struct {
				name string
				give lsptest.Mode
				want int
			}{
				{
					"responds to references with no location before its job ends in the Loading mode",
					lsptest.Loading, 0,
				},
				{"responds to references with no location while its job runs in the Stuck mode", lsptest.Stuck, 0},
				{
					"responds to references with no location before its job ends in the Created mode",
					lsptest.Created, 0,
				},
				{
					"responds to references with no location before its second job ends in the Burst mode",
					lsptest.Burst, 0,
				},
				{"responds to references with the uses of Store in the Default mode", lsptest.Default, 2},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					p := run(t, tt.give)
					p.initialize(t)
					p.send(t, opening(lsptest.Content))
					p.send(t, referencing)
					assert.Length(t, p.located(t), tt.want, "the locations of the reply to references")
				})
			}
		}

		t.Run("begins its job CreateTime after initialize in the Created mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Created)
			p.initialize(t)
			time.Sleep(lsptest.CreateTime + loadMargin)
			p.send(t, asking)
			assert.Equal(t, p.progress(t, asked), []string{beginning}, "the messages of jobs before the reply")
		})

		t.Run("responds to references with the uses of Store after its job ends in the Loading mode",
			func(t *testing.T) {
				t.Parallel()
				p := run(t, lsptest.Loading)
				p.initialize(t)
				p.send(t, opening(lsptest.Content))
				time.Sleep(lsptest.LoadTime + loadMargin)
				p.send(t, referencing)
				assert.Length(t, p.located(t), 2, "the locations of the reply to references")
			})

		t.Run("responds to references with the use in its buffer of b.fake in the Scoped mode", func(t *testing.T) {
			t.Parallel()
			p := run(t, lsptest.Scoped)
			p.initialize(t)
			p.send(t, opening(lsptest.Content))
			p.send(t, openingOf("b.fake", "var _ Store\n"))
			p.send(t, referencing)
			assert.Equal(t, p.located(t), []string{"file:///tmp/b.fake"}, "the files of the reply to references")
		})

		t.Run("responds to references with the uses of Store after its second job ends in the Burst mode",
			func(t *testing.T) {
				t.Parallel()
				p := run(t, lsptest.Burst)
				p.initialize(t)
				p.send(t, opening(lsptest.Content))
				time.Sleep(2*lsptest.BurstJob + lsptest.BurstGap + loadMargin)
				p.send(t, referencing)
				assert.Length(t, p.located(t), 2, "the locations of the reply to references")
			})

		t.Run("publishes no report to a client without publishDiagnostics in the Pushes mode",
			func(t *testing.T) {
				t.Parallel()
				p := run(t, lsptest.Pushes)
				p.send(t, initializing("/tmp", "{}"))
				p.reply(t, 1)
				p.send(t, opening(lsptest.Faulty))
				p.send(t, asking)
				assert.Empty(t, p.reports(t), "the reports before the reply to documentSymbol")
			})

		{
			tests := []struct {
				name string
				give lsptest.Mode
				want int
			}{
				{"publishes no report of an opened document in the Default mode", lsptest.Default, 0},
				{"publishes a report of an opened document in the Pushes mode", lsptest.Pushes, 1},
				{"publishes a report of an opened document in the PushesOne mode", lsptest.PushesOne, 1},
				{"publishes a report of an opened document in the DiagnosisStuck mode", lsptest.DiagnosisStuck, 1},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					p := run(t, tt.give)
					p.initialize(t)
					p.send(t, opening(lsptest.Faulty))
					p.send(t, asking)
					assert.Length(t, p.reports(t), tt.want, "the reports before the reply to documentSymbol")
				})
			}
		}
	})
}
