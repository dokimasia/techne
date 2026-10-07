// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"

	"go.dokimi.dev/assert"
	assertfiles "go.dokimi.dev/assert/files"
	"go.dokimi.dev/techne/internal/app"
	"go.dokimi.dev/techne/service/workspace/files"
	"go.dokimi.dev/techne/tool"
)

// projectFile is the file of the mock language in each project of the tests.
const projectFile = "a.mock"

// mockLanguage is the specification of the mock language, which serves every tool from the
// files of a workspace on disk.
const mockLanguage = "1"

// gone is the reason of the refusal of apply.change for a handle that its workspace does not
// keep, as a workspace that a session closed and opened again does not.
const gone = "no preview is kept under that handle: preview again to get one"

// outcome is the result and the error of a call that runs in another goroutine.
type outcome struct {
	result tool.Result
	err    error
}

// project returns the directory name under dir, with a [projectFile] that declares the type
// declared and uses it.
func project(t *testing.T, dir, name, declared string) string {
	t.Helper()
	full := filepath.Join(dir, name)
	assert.NoError(t, os.MkdirAll(full, 0o755), "the error of MkdirAll")
	content := "type " + declared + "\nfunc New\n  use " + declared + "\n"
	assert.NoError(t, os.WriteFile(filepath.Join(full, projectFile), []byte(content), 0o644),
		"the error of WriteFile")
	return full
}

// resolved returns p without symbolic links.
func resolved(t *testing.T, p string) string {
	t.Helper()
	out, err := filepath.EvalSymlinks(p)
	assert.NoError(t, err, "the error of EvalSymlinks")
	return out
}

// started returns the session of command with the mock language, and closes it when the test
// ends.
func started(t *testing.T, command app.Command) *app.Session {
	t.Helper()
	s, err := app.Open(t.Context(), command, mockLanguage)
	assert.NoError(t, err, "the error of Open")
	t.Cleanup(func() { _ = s.Close(context.WithoutCancel(t.Context())) })
	return s
}

// execute returns the payload of a call of the tool name of s with the fields of input,
// decoded as an object, and the error of the call.
func execute(t *testing.T, s *app.Session, name string, input map[string]any) (map[string]any, error) {
	t.Helper()
	encoded, err := json.Marshal(input)
	assert.NoError(t, err, "the encoding of the input")
	called, found := s.Tools.Tool(name)
	assert.True(t, found, "a tool named "+name)
	got, err := called.Execute(t.Context(), encoded)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	assert.NoError(t, json.Unmarshal(got.Payload, &decoded), "the payload of "+name)
	return decoded, nil
}

// outlined returns the names of the declarations of an outline of [projectFile] in the
// directory wd, or in the workspace of s for the empty wd.
func outlined(t *testing.T, s *app.Session, wd string) []string {
	t.Helper()
	input := map[string]any{"scope": projectFile}
	if wd != "" {
		input[tool.WorkingDirectory] = wd
	}
	got, err := execute(t, s, "outline", input)
	assert.NoError(t, err, "the error of the outline")
	var out []string
	for _, item := range got["items"].([]any) {
		out = append(out, item.(map[string]any)["name"].(string))
	}
	return out
}

// renamed previews the rename of the type declared in the project at wd to Moved, and returns
// the handle of the preview.
func renamed(t *testing.T, s *app.Session, wd, declared string) string {
	t.Helper()
	got, err := execute(t, s, "rename.symbol", map[string]any{
		"scope": projectFile, "name": declared, "new_name": "Moved", tool.WorkingDirectory: wd,
	})
	assert.NoError(t, err, "the error of the rename")
	handle, _ := got["handle"].(string)
	assert.NotEmpty(t, handle, "the handle of the preview")
	return handle
}

// applied returns the payload of apply.change of handle in the directory wd.
func applied(t *testing.T, s *app.Session, wd, handle string) map[string]any {
	t.Helper()
	got, err := execute(t, s, "apply.change", map[string]any{"handle": handle, tool.WorkingDirectory: wd})
	assert.NoError(t, err, "the error of apply.change")
	return got
}

func TestSession(t *testing.T) {
	t.Parallel()

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		t.Run("serves a call without wd in the workspace of the session", func(t *testing.T) {
			t.Parallel()
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{t.TempDir()},
			})
			assert.Equal(t, outlined(t, s, ""), []string{"Home", "New"}, "the declarations of the outline")
		})

		t.Run("serves a call in the workspace of a wd under a trusted folder", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			})
			got := outlined(t, s, project(t, trusted, "away", "Away"))
			assert.Equal(t, got, []string{"Away", "New"}, "the declarations of the outline")
		})

		t.Run("serves a wd that is the trusted folder", func(t *testing.T) {
			t.Parallel()
			trusted := project(t, t.TempDir(), "projects", "Projects")
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			})
			assert.Equal(t, outlined(t, s, trusted), []string{"Projects", "New"}, "the declarations of the outline")
		})

		t.Run("serves a wd relative to the root of the session", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			project(t, trusted, "away", "Away")
			s := started(t, app.Command{Root: project(t, trusted, "home", "Home"), Trusted: []string{trusted}})
			assert.Equal(t, outlined(t, s, "../away"), []string{"Away", "New"}, "the declarations of the outline")
		})

		t.Run("serves a wd that links to a directory under a trusted folder", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			link := filepath.Join(t.TempDir(), "link")
			assert.NoError(t, os.Symlink(project(t, trusted, "away", "Away"), link), "the error of Symlink")
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			})
			assert.Equal(t, outlined(t, s, link), []string{"Away", "New"}, "the declarations of the outline")
		})

		t.Run("serves a wd at the root of the session in the workspace of the session", func(t *testing.T) {
			t.Parallel()
			home := project(t, t.TempDir(), "home", "Home")
			s := started(t, app.Command{Root: home, Trusted: []string{t.TempDir()}})
			assert.Equal(t, outlined(t, s, home), []string{"Home", "New"}, "the declarations of the outline")
		})

		t.Run("returns an error for a wd under no trusted folder", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			outside := project(t, t.TempDir(), "outside", "Outside")
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			})
			_, err := execute(t, s, "outline", map[string]any{"scope": projectFile, tool.WorkingDirectory: outside})
			assert.HasError(t, err, "the error of the outline")
			assert.Equal(t, err.Error(), `app: the working directory "`+outside+`" is under no trusted folder: `+
				resolved(t, trusted), "the error of the outline")
		})

		t.Run("returns an error for a wd that links out of the trusted folders", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			escape := filepath.Join(trusted, "escape")
			assert.NoError(t, os.Symlink(project(t, t.TempDir(), "outside", "Outside"), escape),
				"the error of Symlink")
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			})
			_, err := execute(t, s, "outline", map[string]any{"scope": projectFile, tool.WorkingDirectory: escape})
			assert.HasError(t, err, "the error of the outline")
			assert.Contains(t, err.Error(), "is under no trusted folder", "the error of the outline")
		})

		t.Run("returns an error for a wd that is a file", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			file := filepath.Join(project(t, trusted, "away", "Away"), projectFile)
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			})
			_, err := execute(t, s, "outline", map[string]any{"scope": projectFile, tool.WorkingDirectory: file})
			assert.HasError(t, err, "the error of the outline")
			assert.Equal(t, err.Error(), `app: the working directory "`+file+`" is not a directory`,
				"the error of the outline")
		})

		t.Run("returns an error for a wd that does not exist", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			})
			nowhere := filepath.Join(trusted, "nowhere")
			_, err := execute(t, s, "outline", map[string]any{"scope": projectFile, tool.WorkingDirectory: nowhere})
			assert.HasError(t, err, "the error of the outline")
			assert.HasPrefix(t, err.Error(), `app: the working directory "`+nowhere+`": `, "the error of the outline")
		})

		t.Run("adds wd to every tool for a trusted folder", func(t *testing.T) {
			t.Parallel()
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{t.TempDir()},
			})
			for _, one := range s.Tools.Tools() {
				assert.NotNil(t, one.InputSchema().Properties[tool.WorkingDirectory], "the wd of "+one.Name())
			}
		})

		t.Run("adds wd to no tool without a trusted folder", func(t *testing.T) {
			t.Parallel()
			s := started(t, app.Command{Root: project(t, t.TempDir(), "home", "Home")})
			for _, one := range s.Tools.Tools() {
				assert.NotContains(t, one.InputSchema().Properties, tool.WorkingDirectory, "the wd of "+one.Name())
			}
		})

		t.Run("applies a preview in the workspace of its wd", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			away := project(t, trusted, "away", "Away")
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			})
			got := applied(t, s, away, renamed(t, s, away, "Away"))
			assert.Equal(t, got["applied"], any(true), "applied of the change")
			assertfiles.HasContent(t, filepath.Join(away, projectFile), "type Moved\nfunc New\n  use Moved\n",
				"the file after the change")
		})

		t.Run("keeps the workspaces of three directories open", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			first := project(t, trusted, "first", "First")
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			})
			handle := renamed(t, s, first, "First")
			for _, name := range []string{"second", "third"} {
				outlined(t, s, project(t, trusted, name, "Other"))
			}
			assert.Equal(t, applied(t, s, first, handle)["applied"], any(true), "applied of the change")
		})

		t.Run("closes the workspace that served a call least recently for a fourth directory", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			first := project(t, trusted, "first", "First")
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			})
			handle := renamed(t, s, first, "First")
			for _, name := range []string{"second", "third", "fourth"} {
				outlined(t, s, project(t, trusted, name, "Other"))
			}
			failure, _ := applied(t, s, first, handle)["error"].(map[string]any)
			assert.Equal(t, failure["reason"], any(gone), "the reason of the refusal")
		})

		t.Run("closes the workspace used least recently in place of the one opened first", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			first := project(t, trusted, "first", "First")
			second := project(t, trusted, "second", "Second")
			s := started(t, app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			})
			kept := renamed(t, s, first, "First")
			closed := renamed(t, s, second, "Second")
			outlined(t, s, project(t, trusted, "third", "Other"))
			outlined(t, s, first)
			outlined(t, s, project(t, trusted, "fourth", "Other"))
			assert.Equal(t, applied(t, s, first, kept)["applied"], any(true), "applied of the change in first")
			failure, _ := applied(t, s, second, closed)["error"].(map[string]any)
			assert.Equal(t, failure["reason"], any(gone), "the reason of the refusal in second")
		})

		t.Run("keeps the workspace of a call in flight open", func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				trusted := t.TempDir()
				first := project(t, trusted, "first", "First")
				s := started(t, app.Command{
					Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
				})
				handle := renamed(t, s, first, "First")
				locked, err := files.Open(first)
				assert.NoError(t, err, "the error of Open")
				t.Cleanup(func() { _ = locked.Close() })
				unlock, err := locked.Lock(t.Context())
				assert.NoError(t, err, "the error of Lock")

				apply, _ := s.Tools.Tool("apply.change")
				input, err := json.Marshal(map[string]any{"handle": handle, tool.WorkingDirectory: first})
				assert.NoError(t, err, "the encoding of the input")
				done := make(chan outcome, 1)
				go func() {
					result, err := apply.Execute(t.Context(), input)
					done <- outcome{result: result, err: err}
				}()
				synctest.Wait()

				second := project(t, trusted, "second", "Second")
				closed := renamed(t, s, second, "Second")
				for _, name := range []string{"third", "fourth"} {
					outlined(t, s, project(t, trusted, name, "Other"))
				}
				unlock()
				flown := <-done
				assert.NoError(t, flown.err, "the error of the change in flight")
				var got map[string]any
				assert.NoError(t, json.Unmarshal(flown.result.Payload, &got), "the payload of the change in flight")
				assert.Equal(t, got["applied"], any(true), "applied of the change in flight")
				failure, _ := applied(t, s, second, closed)["error"].(map[string]any)
				assert.Equal(t, failure["reason"], any(gone), "the reason of the refusal in second")
			})
		})

		t.Run("returns an error for a trusted folder that does not exist", func(t *testing.T) {
			t.Parallel()
			nowhere := filepath.Join(t.TempDir(), "nowhere")
			_, err := app.Open(t.Context(), app.Command{Root: t.TempDir(), Trusted: []string{nowhere}}, mockLanguage)
			assert.HasError(t, err, "the error of Open")
			assert.HasPrefix(t, err.Error(), `app: the trusted folder "`+nowhere+`": `, "the error of Open")
		})

		t.Run("returns an error for a trusted folder that is a file", func(t *testing.T) {
			t.Parallel()
			file := filepath.Join(project(t, t.TempDir(), "away", "Away"), projectFile)
			_, err := app.Open(t.Context(), app.Command{Root: t.TempDir(), Trusted: []string{file}}, mockLanguage)
			assert.HasError(t, err, "the error of Open")
			assert.Equal(t, err.Error(), `app: the trusted folder "`+file+`" is not a directory`, "the error of Open")
		})

		t.Run("returns an error for a root that does not exist", func(t *testing.T) {
			t.Parallel()
			_, err := app.Open(t.Context(), app.Command{Root: filepath.Join(t.TempDir(), "nowhere")}, mockLanguage)
			assert.HasError(t, err, "the error of Open")
			assert.HasPrefix(t, err.Error(), "app: the workspace root ", "the error of Open")
		})

		t.Run("returns an error for a specification of mock languages that Build refuses", func(t *testing.T) {
			t.Parallel()
			_, err := app.Open(t.Context(), app.Command{Root: t.TempDir()}, "alpha@resolvd")
			assert.HasError(t, err, "the error of Open")
			assert.HasPrefix(t, err.Error(), "app: the mock language alpha does not take the fidelity ",
				"the error of Open")
		})
	})

	t.Run("Close", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil after calls in the workspaces of two directories", func(t *testing.T) {
			t.Parallel()
			trusted := t.TempDir()
			s, err := app.Open(t.Context(), app.Command{
				Root: project(t, t.TempDir(), "home", "Home"), Trusted: []string{trusted},
			}, mockLanguage)
			assert.NoError(t, err, "the error of Open")
			for _, name := range []string{"first", "second"} {
				outlined(t, s, project(t, trusted, name, "Other"))
			}
			assert.NoError(t, s.Close(t.Context()), "the error of Close")
		})
	})
}
