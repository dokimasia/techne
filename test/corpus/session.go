// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package corpus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.dokimi.dev/techne/tool"
)

// waitedMeta is the key of the _meta of a result that states the time techne waited for
// processes outside it.
const waitedMeta = tool.WaitedMeta

// ModuleRoot returns the directory of the module that contains the working
// directory, which is the root of this repository.
func ModuleRoot(ctx context.Context) (string, error) {
	module, err := exec.CommandContext(ctx, "go", "env", "GOMOD").Output()
	if err != nil {
		return "", fmt.Errorf("corpus: find the module: %w", err)
	}
	return filepath.Dir(strings.TrimSpace(string(module))), nil
}

// Binary builds cmd/techne of the module at root into dir, an existing
// directory, and returns the path of the binary. go build names the binary
// after the command, with the suffix of executables of the system, such as
// .exe on Windows.
func Binary(ctx context.Context, root, dir string) (string, error) {
	build := exec.CommandContext(ctx, "go", "build", "-o", dir, "./cmd/techne")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("corpus: build techne: %w\n%s", err, out)
	}
	binary, err := exec.LookPath(filepath.Join(dir, "techne"))
	if err != nil {
		return "", fmt.Errorf("corpus: find the build of techne: %w", err)
	}
	return binary, nil
}

// Session is one MCP session with a techne process over one workspace. It
// records the time of every call.
type Session struct {
	session *mcp.ClientSession

	calls []Call

	mu sync.Mutex

	warm bool
}

// Call is one tool call of a session.
type Call struct {
	// Tool is the name of the tool.
	Tool string
	// Fidelity and Completeness are the provenance of the answer, and empty
	// for an answer without one.
	Fidelity     string
	Completeness string
	// Took is the time from the request to the result.
	Took time.Duration
	// Server is the part of Took that techne waited for processes outside
	// it, such as a language server, as the result states it.
	Server time.Duration
	// Warm reports that the call followed Session.Warm.
	Warm bool
	// Failed reports a call that failed, or a tool that returned an error
	// result.
	Failed bool
}

// Own returns the part of Took that techne spent itself: Took without Server, and zero when the
// waits that overlap sum to more than Took.
func (c Call) Own() time.Duration { return max(c.Took-c.Server, 0) }

// provenance is the part of every answer that states its evidence.
type provenance struct {
	Provenance struct {
		Fidelity     string `json:"fidelity"`
		Completeness string `json:"completeness"`
	} `json:"provenance"`
}

// structured is the flag that makes techne send the structured content of
// each result, which [Session.Call] decodes.
const structured = "--structured"

// Start runs binary over root with env and [structured], and connects to it
// over its standard input and output. The standard error of the process goes
// to stderr.
func Start(ctx context.Context, binary, root string, env []string, stderr io.Writer) (*Session, error) {
	// The process lives until Close, so it runs on a context that does not end with ctx.
	cmd := exec.CommandContext(context.WithoutCancel(ctx), binary, structured, root)
	cmd.Env = env
	cmd.Stderr = stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "techne-corpus", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("corpus: start techne over %s: %w", root, err)
	}
	return &Session{session: session}, nil
}

// Call calls tool with arguments, decodes its structured result into out,
// and records the call with the provenance of the answer. A tool that
// returns an error result still decodes into out, and Call returns no error
// for it. Call returns an error when the call fails or the result has no
// structured content.
func (s *Session) Call(ctx context.Context, tool string, arguments, out any) (Call, error) {
	s.mu.Lock()
	warm := s.warm
	s.mu.Unlock()

	start := time.Now()
	result, err := s.session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: arguments})
	call := Call{Tool: tool, Took: time.Since(start), Warm: warm, Failed: err != nil || result.IsError}
	defer s.record(&call)

	if err != nil {
		return call, fmt.Errorf("corpus: call %s: %w", tool, err)
	}
	if ms, stated := result.Meta[waitedMeta].(float64); stated {
		call.Server = time.Duration(ms) * time.Millisecond
	}
	if result.StructuredContent == nil {
		return call, fmt.Errorf("corpus: %s returned no structured content: %s", tool, texts(result.Content))
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return call, fmt.Errorf("corpus: %s: %w", tool, err)
	}
	var stated provenance
	if json.Unmarshal(encoded, &stated) == nil {
		call.Fidelity, call.Completeness = stated.Provenance.Fidelity, stated.Provenance.Completeness
	}
	if err := json.Unmarshal(encoded, out); err != nil {
		return call, fmt.Errorf("corpus: decode the result of %s: %w", tool, err)
	}
	return call, nil
}

// texts returns the text blocks of content, joined by a space, such as the reason of an error
// result that has no structured content.
func texts(content []mcp.Content) string {
	var out []string
	for _, block := range content {
		if text, isText := block.(*mcp.TextContent); isText {
			out = append(out, text.Text)
		}
	}
	return strings.Join(out, " ")
}

// Warm marks the calls that follow as warm.
func (s *Session) Warm() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.warm = true
}

// Calls returns every call of the session, in order.
func (s *Session) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Call(nil), s.calls...)
}

// Close ends the session and waits for the techne process to exit.
func (s *Session) Close() error {
	if err := s.session.Close(); err != nil {
		return fmt.Errorf("corpus: close the session: %w", err)
	}
	return nil
}

// record appends call to the calls of the session.
func (s *Session) record(call *Call) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, *call)
}
