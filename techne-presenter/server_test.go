// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package presenter_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/presenter"
	"go.dokimi.dev/techne/tool"
)

// serveVar is the variable that makes a process of the test binary serve the tools of
// [registered] over standard input and output.
const serveVar = "TECHNE_PRESENTER_SERVE"

// fault is the value that the tool broken panics with.
const fault = "broken fails"

// about is the name and the version of every server of the cases.
var about = presenter.Info{Name: "techne", Version: "test"}

// TestMain serves the tools of registered in a process that a case started, and runs the
// tests otherwise.
func TestMain(m *testing.M) {
	if os.Getenv(serveVar) == "" {
		os.Exit(m.Run())
	}
	if err := serving(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

// serving serves the tools of registered over standard input and output until the client
// closes standard input.
func serving() error {
	r, err := registered()
	if err != nil {
		return err
	}
	server, err := presenter.NewServer(r, about)
	if err != nil {
		return err
	}
	return presenter.Serve(context.Background(), server)
}

// answerIn is the input of the tools of the cases.
type answerIn struct {
	Scope string `json:"scope"`
}

// answerOut is the output of the tools of the cases. Its fields are out of alphabetical
// order, so an encoding that sorts the fields of an object changes the payload.
type answerOut struct {
	Status string `json:"status"`
	Scope  string `json:"scope"`
}

// Failed reports the statuses of an output that did not do what was asked.
func (a answerOut) Failed() bool {
	return a.Status == "unsupported" || a.Status == "refused"
}

// renderedOut is an output that renders itself as text.
type renderedOut struct {
	Status string `json:"status"`
}

// Render returns the status as a line of text.
func (r renderedOut) Render() string {
	return "status: " + r.Status + "\n"
}

// invalid is a tool whose payload is not JSON. The description and the schemas are those of
// the tool that it embeds.
type invalid struct {
	tool.Tool
}

// Name returns invalid.
func (invalid) Name() string {
	return "invalid"
}

// Execute returns a payload that ends inside an object.
func (invalid) Execute(context.Context, json.RawMessage) (tool.Result, error) {
	return tool.Result{Payload: json.RawMessage(`{"status":`)}, nil
}

// answering returns a handler that returns status and the scope of its input.
func answering(status string) func(context.Context, answerIn) (answerOut, error) {
	return func(_ context.Context, in answerIn) (answerOut, error) {
		return answerOut{Status: status, Scope: in.Scope}, nil
	}
}

// rendering returns an output with a render.
func rendering(context.Context, answerIn) (renderedOut, error) {
	return renderedOut{Status: "ok"}, nil
}

// failing returns the error of a refused path.
func failing(context.Context, answerIn) (answerOut, error) {
	return answerOut{}, errors.New(`tool: "/etc/passwd" is absolute`)
}

// panicking panics with fault.
func panicking(context.Context, answerIn) (answerOut, error) {
	panic(fault)
}

// registered returns a registry of the tools that the cases call:
//   - outline returns the status ok and the scope of its input.
//   - unsupported returns the status unsupported, which fails.
//   - rendered returns an output with a render.
//   - failing returns an error.
//   - broken panics with fault.
//   - invalid returns a payload that is not JSON.
func registered() (*tool.Registry, error) {
	outline, err := tool.New("outline", "PREFER OVER reading a file.", answering("ok"))
	if err != nil {
		return nil, err
	}
	r := tool.NewRegistry()
	add := func(built tool.Tool, failed error) error {
		if failed != nil {
			return failed
		}
		return r.Add(built)
	}
	return r, errors.Join(
		r.Add(outline),
		add(tool.New("unsupported", "PREFER OVER reading a file.", answering("unsupported"))),
		add(tool.New("rendered", "PREFER OVER reading a file.", rendering)),
		add(tool.New("failing", "PREFER OVER reading a file.", failing)),
		add(tool.New("broken", "PREFER OVER reading a file.", panicking)),
		r.Add(invalid{outline}),
	)
}

// record is a writer that one goroutine writes and a case reads.
type record struct {
	mu   sync.Mutex
	text bytes.Buffer
}

// Write appends p to the record.
func (r *record) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.text.Write(p)
}

// String returns the content of the record.
func (r *record) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.text.String()
}

// connected returns a client session with a server of the tools of registered, over an
// in-memory transport that writes each message of the client to log.
func connected(t *testing.T, log io.Writer) *mcp.ClientSession {
	t.Helper()
	r, err := registered()
	assert.NoError(t, err, "the error of registered")
	server, err := presenter.NewServer(r, about)
	assert.NoError(t, err, "the error of NewServer")

	clientSide, serverSide := mcp.NewInMemoryTransports()
	served, err := server.Connect(t.Context(), serverSide, nil)
	assert.NoError(t, err, "the error of Connect of the server")
	t.Cleanup(func() { _ = served.Wait() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	session, err := client.Connect(t.Context(), &mcp.LoggingTransport{Transport: clientSide, Writer: log}, nil)
	assert.NoError(t, err, "the error of Connect of the client")
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// spawned returns a client session with a process of the test binary that serves the tools
// of registered over standard input and output, and the record of its standard error.
func spawned(t *testing.T) (*mcp.ClientSession, *record) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), serveVar+"=1")
	stderr := &record{}
	cmd.Stderr = stderr

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	session, err := client.Connect(t.Context(), &mcp.CommandTransport{Command: cmd}, nil)
	assert.NoError(t, err, "the error of Connect")
	t.Cleanup(func() { _ = session.Close() })
	return session, stderr
}

// called returns the result of a call of the tool name of session with input.
func called(t *testing.T, session *mcp.ClientSession, name, input string) *mcp.CallToolResult {
	t.Helper()
	got, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: json.RawMessage(input)})
	assert.NoError(t, err, "the error of CallTool of "+name)
	return got
}

// text returns the text of the only content block of result.
func text(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	assert.Length(t, result.Content, 1, "the content blocks of the result")
	block, isText := result.Content[0].(*mcp.TextContent)
	assert.True(t, isText, "the type of the content block")
	return block.Text
}

func TestServer(t *testing.T) {
	t.Parallel()

	t.Run("NewServer", func(t *testing.T) {
		t.Parallel()

		t.Run("lists every tool of the registry", func(t *testing.T) {
			t.Parallel()
			listed, err := connected(t, io.Discard).ListTools(t.Context(), nil)
			assert.NoError(t, err, "the error of ListTools")
			var names []string
			for _, one := range listed.Tools {
				names = append(names, one.Name)
				assert.Equal(t, one.Description, "PREFER OVER reading a file.", "the description of "+one.Name)
			}
			slices.Sort(names)
			assert.Equal(t, names, []string{"broken", "failing", "invalid", "outline", "rendered", "unsupported"},
				"the names of the tools")
		})

		t.Run("returns an error for a schema that does not encode", func(t *testing.T) {
			t.Parallel()
			unencodable, err := tool.New("outline", "PREFER OVER reading a file.", answering("ok"))
			assert.NoError(t, err, "the error of New")
			// A schema with both Type and Types set does not encode.
			unencodable.InputSchema().Types = []string{"object"}
			r := tool.NewRegistry()
			assert.NoError(t, r.Add(unencodable), "the error of Add")
			_, err = presenter.NewServer(r, about)
			assert.HasError(t, err, "the error of NewServer")
			assert.HasPrefix(t, err.Error(), `presenter: "outline" input schema: `, "the error of NewServer")
		})

		t.Run("returns the payload of a tool as its structured content", func(t *testing.T) {
			t.Parallel()
			got := called(t, connected(t, io.Discard), "outline", `{"scope":"a.fx"}`)
			assert.False(t, got.IsError, "IsError of the result")
			assert.Equal(t, got.StructuredContent, any(map[string]any{"status": "ok", "scope": "a.fx"}),
				"the structured content of the result")
		})

		t.Run("returns the render of a tool as its text", func(t *testing.T) {
			t.Parallel()
			got := called(t, connected(t, io.Discard), "rendered", `{"scope":"a.fx"}`)
			assert.Equal(t, text(t, got), "status: ok\n", "the text of the result")
		})

		t.Run("returns the payload as the text of a tool without a render", func(t *testing.T) {
			t.Parallel()
			got := called(t, connected(t, io.Discard), "outline", `{"scope":"a.fx"}`)
			assert.Equal(t, text(t, got), `{"status":"ok","scope":"a.fx"}`, "the text of the result")
		})

		t.Run("marks a failed result as an error", func(t *testing.T) {
			t.Parallel()
			got := called(t, connected(t, io.Discard), "unsupported", `{"scope":"a.md"}`)
			assert.True(t, got.IsError, "IsError of the result")
			assert.Equal(t, got.StructuredContent, any(map[string]any{"status": "unsupported", "scope": "a.md"}),
				"the structured content of the result")
		})

		t.Run("returns the error of a tool as an error result", func(t *testing.T) {
			t.Parallel()
			got := called(t, connected(t, io.Discard), "failing", `{"scope":"/etc/passwd"}`)
			assert.True(t, got.IsError, "IsError of the result")
			assert.Equal(t, text(t, got), `tool: "/etc/passwd" is absolute`, "the text of the result")
		})

		t.Run("returns an error result that names a tool that panics", func(t *testing.T) {
			t.Parallel()
			got := called(t, connected(t, io.Discard), "broken", `{"scope":"a.fx"}`)
			assert.True(t, got.IsError, "IsError of the result")
			assert.Equal(t, text(t, got),
				`presenter: "broken" panicked: broken fails. The stack is on the standard error of the server.`,
				"the text of the result")
		})

		t.Run("serves the next call after a tool panics", func(t *testing.T) {
			t.Parallel()
			session := connected(t, io.Discard)
			called(t, session, "broken", `{"scope":"a.fx"}`)
			got := called(t, session, "outline", `{"scope":"a.fx"}`)
			assert.False(t, got.IsError, "IsError of the call after the panic")
			assert.Equal(t, text(t, got), `{"status":"ok","scope":"a.fx"}`, "the text of the call after the panic")
		})

		t.Run("writes the stack of a panic to standard error", func(t *testing.T) {
			t.Parallel()
			session, stderr := spawned(t)
			called(t, session, "broken", `{"scope":"a.fx"}`)
			assert.NoError(t, session.Close(), "the error of Close")
			assert.Contains(t, stderr.String(), `presenter: "broken" panicked: broken fails`,
				"the standard error of the server")
			assert.Contains(t, stderr.String(), "server_test.go", "the stack on the standard error of the server")
		})

		t.Run("returns a protocol error for a payload that is not JSON", func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			_, err := connected(t, io.Discard).CallTool(ctx, &mcp.CallToolParams{
				Name: "invalid", Arguments: json.RawMessage(`{"scope":"a.fx"}`),
			})
			assert.HasError(t, err, "the error of CallTool")
			assert.Contains(t, err.Error(), `presenter: "invalid" returned a payload that is not JSON`,
				"the error of CallTool")
		})
	})

	t.Run("Serve", func(t *testing.T) {
		t.Parallel()

		t.Run("serves a client over stdio", func(t *testing.T) {
			t.Parallel()
			session, _ := spawned(t)
			got := called(t, session, "outline", `{"scope":"a.fx"}`)
			assert.Equal(t, text(t, got), `{"status":"ok","scope":"a.fx"}`, "the text of the result")
		})

		t.Run("returns nil when the client closes stdin", func(t *testing.T) {
			t.Parallel()
			session, stderr := spawned(t)
			assert.NoError(t, session.Close(), "the exit of the server after Close")
			assert.Empty(t, stderr.String(), "the standard error of the server")
		})
	})
}
