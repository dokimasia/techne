// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// The files of a workspace in which a move runs. moveFrom exists, and moveTo does not.
const (
	moveFrom = "src/a.go"
	moveTo   = "dst/a.go"
	taken    = "src/b.go"
	other    = "notes.py"
)

// moveRoot returns a workspace that contains moveFrom, taken and other.
func moveRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{moveFrom, taken, other} {
		full := filepath.Join(root, filepath.FromSlash(name))
		assert.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755), "MkdirAll "+name)
		assert.NoError(t, os.WriteFile(full, nil, 0o644), "WriteFile "+name)
	}
	return root
}

// file returns the target of the file at p.
func file(p source.Path) edit.Target { return edit.Target{Kind: edit.TargetFile, Path: p} }

// to returns the arguments of a move to p.
func to(p string) edit.Args { return edit.Args{edit.ArgDestination: p} }

// goFiles are the extensions of Go.
var goFiles = []string{".go"}

func TestMove(t *testing.T) {
	t.Parallel()

	t.Run("Moving", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the two ends of a move", func(t *testing.T) {
			t.Parallel()
			from, dest, err := lang.Moving(moveRoot(t), file(moveFrom), to(" "+moveTo+" "), goFiles)
			assert.NoError(t, err, "Moving")
			assert.Equal(t, [2]source.Path{from, dest}, [2]source.Path{moveFrom, moveTo}, "the ends of the move")
		})

		t.Run("returns no source for a file of another language", func(t *testing.T) {
			t.Parallel()
			from, _, err := lang.Moving(moveRoot(t), file(other), to("docs/notes.py"), goFiles)
			assert.NoError(t, err, "Moving of a Python file")
			assert.Equal(t, from, source.Path(""), "the source of a move of another language")
		})

		refusals := []struct {
			name   string
			target edit.Target
			args   edit.Args
			// says is a part of the text of the refusal.
			says string
		}{
			{
				name:   "refuses a target that is not a file",
				target: edit.Target{Kind: edit.TargetSymbol, Path: moveFrom}, args: to(moveTo), says: "names a file",
			},
			{name: "refuses a file target without a path", target: file(""), args: to(moveTo), says: "names a file"},
			{
				name:   "refuses a move without a destination",
				target: file(moveFrom), args: to(" "), says: "needs " + string(edit.ArgDestination),
			},
			{
				name:   "refuses a move onto the file itself",
				target: file(moveFrom), args: to(moveFrom), says: "is already at",
			},
			{
				name:   "refuses a source outside the workspace",
				target: file("../a.go"), args: to(moveTo), says: "../a.go is outside it",
			},
			{
				name:   "refuses a destination outside the workspace",
				target: file(moveFrom), args: to("../a.go"), says: "../a.go is outside it",
			},
			{
				name:   "refuses a file that does not exist",
				target: file("src/c.go"), args: to(moveTo), says: "src/c.go does not exist",
			},
			{
				name:   "refuses a destination where a file exists",
				target: file(moveFrom), args: to(taken), says: taken + " exists",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, _, err := lang.Moving(moveRoot(t), tt.target, tt.args, goFiles)
				assert.ErrorIs(t, err, engine.ErrRefuse, "Moving")
				assert.Contains(t, err.Error(), tt.says, "the text of the refusal")
			})
		}
	})

	t.Run("Outside", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give source.Path
			want bool
		}{
			{name: "reports a relative path inside", give: "src/a.go", want: false},
			{name: "reports a name that starts with two dots inside", give: "..config.ts", want: false},
			{name: "reports a path that leaves and returns inside", give: "src/../a.go", want: false},
			{name: "reports the parent outside", give: "..", want: true},
			{name: "reports a path under the parent outside", give: "../a.go", want: true},
			{name: "reports a path that climbs above the root outside", give: "src/../../a.go", want: true},
			{name: "reports an absolute path outside", give: "/tmp/a.go", want: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, lang.Outside(tt.give), tt.want, "Outside of "+string(tt.give))
			})
		}
	})
}
