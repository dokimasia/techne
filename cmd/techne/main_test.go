// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package main_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/internal/app"
	"go.dokimi.dev/techne/internal/version"
)

// stamps are the values that the build of the tests gives the fields of the templates of the
// release configuration.
var stamps = map[string]string{
	"Version":    "1.2.3",
	"Commit":     "0123456789abcdef0123456789abcdef01234567",
	"CommitDate": "2026-09-24T00:00:00Z",
}

// stamp matches a -X flag of the release configuration, and captures the variable and the
// field of its template: -X go.dokimi.dev/techne/internal/version.buildVersion={{.Version}}.
var stamp = regexp.MustCompile(`-X (\S+)=\{\{\s*\.(\w+)\s*\}\}`)

// store is a file of the mock language with a declaration and a use of it.
const store = ";; Store maps a name to an item.\ntype Store\n  field size\nfunc New\n  use Store\n"

// binary is the path of the command that TestMain builds.
var binary string

// TestMain builds the command with the -X flags of the release configuration, runs the tests,
// and removes the build.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "techne-cmd-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	binary = filepath.Join(dir, "techne")
	code := 2
	if err := build(binary); err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		code = m.Run()
	}
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// build builds the command into path with the -X flags of the release configuration, each with
// its value in stamps. It returns an error for a field that stamps does not have, and for a
// configuration without a flag for each field of stamps.
func build(path string) error {
	release, err := os.ReadFile(filepath.Join("..", "..", ".goreleaser.yml"))
	if err != nil {
		return fmt.Errorf("read the release configuration: %w", err)
	}
	var flags []string
	for _, match := range stamp.FindAllStringSubmatch(string(release), -1) {
		value, known := stamps[match[2]]
		if !known {
			return fmt.Errorf("the release configuration stamps the field %s, which the tests do not set", match[2])
		}
		flags = append(flags, "-X "+match[1]+"="+value)
	}
	if len(flags) != len(stamps) {
		return fmt.Errorf("the release configuration has %d -X flags for the %d fields of the tests",
			len(flags), len(stamps))
	}
	// TestMain builds the command once for every test, so the build has no test's context.
	out, err := exec.CommandContext(context.Background(), "go", "build", "-ldflags", strings.Join(flags, " "), "-o",
		path, ".").CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build: %w\n%s", err, out)
	}
	return nil
}

// environment returns the variables of this process without TECHNE_MOCK, with a cache directory
// of the test and with the variables of extra.
func environment(t *testing.T, extra ...string) []string {
	t.Helper()
	var out []string
	for _, one := range os.Environ() {
		if !strings.HasPrefix(one, "TECHNE_MOCK=") {
			out = append(out, one)
		}
	}
	return append(append(out, "XDG_CACHE_HOME="+t.TempDir()), extra...)
}

// ran runs the command with args and the variables of env, with an empty standard input, and
// returns its standard output, its standard error and its exit status.
func ran(t *testing.T, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		_ = assert.ErrorAs[*exec.ExitError](t, err, "the error of Run")
	}
	return stdout.String(), stderr.String(), cmd.ProcessState.ExitCode()
}

// session returns a client session with the command of args, such as the workspace root, with
// the variables of env.
func session(t *testing.T, env []string, args ...string) *mcp.ClientSession {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, args...)
	cmd.Env = env
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	s, err := client.Connect(t.Context(), &mcp.CommandTransport{Command: cmd}, nil)
	assert.NoError(t, err, "the error of Connect")
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// written returns a directory with the file name of content.
func written(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644), "the error of WriteFile")
	return dir
}

func TestTechne(t *testing.T) {
	t.Parallel()

	t.Run("main", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the version of the release configuration for --version", func(t *testing.T) {
			t.Parallel()
			stdout, stderr, code := ran(t, environment(t), "--version")
			assert.Equal(t, code, 0, "the exit status")
			assert.Equal(t, stdout, version.Format(stamps["Version"], stamps["Commit"], stamps["CommitDate"])+"\n",
				"the standard output")
			assert.Empty(t, stderr, "the standard error")
		})

		t.Run("writes the usage to standard output for each help flag", func(t *testing.T) {
			t.Parallel()
			for _, flag := range []string{"-h", "--help"} {
				stdout, stderr, code := ran(t, environment(t), flag)
				assert.Equal(t, code, 0, "the exit status for "+flag)
				assert.Equal(t, stdout, app.Usage, "the standard output for "+flag)
				assert.Empty(t, stderr, "the standard error for "+flag)
			}
		})

		t.Run("exits 2 for an undeclared flag", func(t *testing.T) {
			t.Parallel()
			stdout, stderr, code := ran(t, environment(t), "--verbose")
			assert.Equal(t, code, 2, "the exit status")
			assert.Empty(t, stdout, "the standard output")
			assert.Equal(t, stderr, "techne: app: the command does not take the flag \"--verbose\"\n"+app.Usage,
				"the standard error")
		})

		t.Run("exits 2 for a second workspace", func(t *testing.T) {
			t.Parallel()
			_, stderr, code := ran(t, environment(t), t.TempDir(), t.TempDir())
			assert.Equal(t, code, 2, "the exit status")
			assert.HasPrefix(t, stderr, "techne: app: the command takes one workspace", "the standard error")
		})

		t.Run("exits 1 for a root that does not exist", func(t *testing.T) {
			t.Parallel()
			_, stderr, code := ran(t, environment(t), filepath.Join(t.TempDir(), "nowhere"))
			assert.Equal(t, code, 1, "the exit status")
			assert.HasPrefix(t, stderr, "techne: app: the workspace root ", "the standard error")
		})

		t.Run("serves a workspace whose root is a symbolic link", func(t *testing.T) {
			t.Parallel()
			link := filepath.Join(t.TempDir(), "link")
			assert.NoError(t, os.Symlink(written(t, "a.mock", store), link), "the error of Symlink")
			client := session(t, environment(t, "TECHNE_MOCK=1"), "--structured", link)
			got, err := client.CallTool(t.Context(), &mcp.CallToolParams{
				Name:      "relations",
				Arguments: map[string]any{"scope": "a.mock", "name": "Store", "relation": "referenced-by"},
			})
			assert.NoError(t, err, "the error of CallTool")
			assert.False(t, got.IsError, "IsError of the result")
			items := got.StructuredContent.(map[string]any)["items"].([]any)
			assert.Length(t, items, 1, "the relations of Store")
			assert.Equal(t, items[0].(map[string]any)["path"], any("a.mock"), "the path of the relation")
		})

		t.Run("serves the render of a result without structured content by default", func(t *testing.T) {
			t.Parallel()
			got, err := session(t, environment(t, "TECHNE_MOCK=1"), written(t, "a.mock", store)).CallTool(t.Context(),
				&mcp.CallToolParams{Name: "outline", Arguments: map[string]any{"scope": "a.mock"}})
			assert.NoError(t, err, "the error of CallTool")
			assert.Nil(t, got.StructuredContent, "the structured content of the result")
			assert.Length(t, got.Content, 1, "the content blocks of the result")
			rendered, isText := got.Content[0].(*mcp.TextContent)
			assert.True(t, isText, "the type of the content block")
			assert.HasPrefix(t, rendered.Text, "a.mock — ", "the text of the result")
		})

		t.Run("exits 1 for a trusted folder that does not exist", func(t *testing.T) {
			t.Parallel()
			_, stderr, code := ran(t, environment(t), "--trust", filepath.Join(t.TempDir(), "nowhere"), t.TempDir())
			assert.Equal(t, code, 1, "the exit status")
			assert.HasPrefix(t, stderr, "techne: app: the trusted folder ", "the standard error")
		})

		t.Run("exits 2 for a --trust without a folder", func(t *testing.T) {
			t.Parallel()
			_, stderr, code := ran(t, environment(t), "--trust")
			assert.Equal(t, code, 2, "the exit status")
			assert.Equal(t, stderr, "techne: app: --trust takes a folder\n"+app.Usage, "the standard error")
		})

		t.Run("exits 0 when the client closes stdin", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, session(t, environment(t), t.TempDir()).Close(), "the exit of the command after Close")
		})
	})
}
