// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package main_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.dokimi.dev/assert"
)

// messages are an initialize request, the notification that follows it and a request for the
// tools, with the ids 1 and 2.
var messages = []string{
	`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18",` +
		`"capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
	`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
}

// TestDoc covers the claims of the comment of the command about its streams and its
// environment.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Streams", func(t *testing.T) {
		t.Parallel()

		t.Run("writes only messages of the protocol to standard output", func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, t.TempDir())
			cmd.Env = environment(t)
			stdin, err := cmd.StdinPipe()
			assert.NoError(t, err, "the error of StdinPipe")
			stdout, err := cmd.StdoutPipe()
			assert.NoError(t, err, "the error of StdoutPipe")
			assert.NoError(t, cmd.Start(), "the error of Start")
			for _, message := range messages {
				_, err := io.WriteString(stdin, message+"\n")
				assert.NoError(t, err, "the error of a write to standard input")
			}

			lines := bufio.NewScanner(stdout)
			lines.Buffer(nil, 16<<20)
			var ids []int
			for lines.Scan() {
				var message struct {
					Version string `json:"jsonrpc"`
					ID      *int   `json:"id"`
				}
				assert.NoError(t, json.Unmarshal(lines.Bytes(), &message), "a line of standard output")
				assert.Equal(t, message.Version, "2.0", "the version of a message")
				if message.ID != nil {
					ids = append(ids, *message.ID)
					if len(ids) == 2 {
						assert.NoError(t, stdin.Close(), "the error of Close of standard input")
					}
				}
			}
			assert.NoError(t, lines.Err(), "the error of the scan of standard output")
			assert.NoError(t, cmd.Wait(), "the exit of the command")
			assert.Equal(t, ids, []int{1, 2}, "the ids of the responses")
		})
	})

	t.Run("Environment", func(t *testing.T) {
		t.Parallel()

		t.Run("serves the mock languages of TECHNE_MOCK at their tiers", func(t *testing.T) {
			t.Parallel()
			env := environment(t, "TECHNE_MOCK=alpha@indexed/partial")
			got, err := session(t, env, written(t, "a.alpha", store)).CallTool(t.Context(), &mcp.CallToolParams{
				Name: "outline", Arguments: map[string]any{"scope": "a.alpha"},
			})
			assert.NoError(t, err, "the error of CallTool")
			assert.False(t, got.IsError, "IsError of the result")
			provenance := got.StructuredContent.(map[string]any)["provenance"].(map[string]any)
			assert.Equal(t, provenance["fidelity"], any("indexed"), "the fidelity of the outline")
			assert.Equal(t, provenance["completeness"], any("partial"), "the completeness of the outline")
		})

		t.Run("exits 1 for a tier of TECHNE_MOCK outside the vocabulary", func(t *testing.T) {
			t.Parallel()
			_, stderr, code := ran(t, environment(t, "TECHNE_MOCK=alpha@resolvd"), t.TempDir())
			assert.Equal(t, code, 1, "the exit status")
			assert.Contains(t, stderr, `does not take the fidelity "resolvd"`, "the standard error")
		})
	})
}
