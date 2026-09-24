// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/go/checker"
)

// The tests load real modules with the go command, because the files of a package and its
// build constraints are the go command's.

// store declares a type with two methods, an interface that the type satisfies, a function and
// a type that embeds the type.
const store = `package p

// Store holds a size.
type Store struct {
	size  int
	Named string
}

// Total returns the size and one.
func (s *Store) Total() int { return s.size + helper() }

func helper() int { return 1 }

type Reader interface{ Sum() int }

// Sum returns the total.
func (s *Store) Sum() int { return s.Total() }

type Wrapped struct {
	Store
	extra int
}
`

// use declares a function that uses Store and calls Total.
const use = `package p

func Use() int {
	held := Store{size: 3}
	return held.Total()
}
`

// broken declares a function that reads a field that Store does not have, on a line that does
// not write Store.
const broken = `package p

func Broken() int {
	held := Store{}
	return held.missing
}
`

// module is the go.mod file of the fixtures of one module.
const module = "module example.com/p\n\ngo 1.24\n"

// written writes files to a new directory and returns it.
func written(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		assert.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755), "MkdirAll "+name)
		assert.NoError(t, os.WriteFile(full, []byte(body), 0o644), "WriteFile "+name)
	}
	return dir
}

// workspace writes a module of files, with the go.mod file of module, and returns its
// directory.
func workspace(t *testing.T, files map[string]string) string {
	t.Helper()
	held := map[string]string{"go.mod": module}
	maps.Copy(held, files)
	return written(t, held)
}

// over returns an engine over the workspace at root, which the test closes when it ends.
func over(t *testing.T, root string) *checker.Engine {
	t.Helper()
	e, err := checker.New(root, golang.Declaration())
	assert.NoError(t, err, "New over "+root)
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer stop()
		assert.NoError(t, e.Close(ctx), "Close")
	})
	return e
}

// serving returns an engine over a module of files.
func serving(t *testing.T, files map[string]string) *checker.Engine {
	t.Helper()
	return over(t, workspace(t, files))
}

// whole returns the files of the module that compiles.
func whole() map[string]string {
	return map[string]string{"store.go": store, "use.go": use}
}

// edges returns the name of the far end of each relation, in order.
func edges(held []sema.Relation) []string {
	out := make([]string, 0, len(held))
	for _, one := range held {
		out = append(out, one.To.Name)
	}
	return out
}

// places returns the path and the zero-based line of the site of each relation, in order.
func places(held []sema.Relation) []string {
	out := make([]string, 0, len(held))
	for _, one := range held {
		out = append(out, fmt.Sprintf("%s:%d", one.At.Path, one.At.Start.Line))
	}
	return out
}

// names returns the name of each declaration, in order.
func names(held []sema.Symbol) []string {
	out := make([]string, 0, len(held))
	for _, one := range held {
		out = append(out, one.Name)
	}
	return out
}

// carries reports whether caveats contain a caveat of code.
func carries(caveats []trust.Caveat, code trust.CaveatCode) bool {
	for _, one := range caveats {
		if one.Code == code {
			return true
		}
	}
	return false
}

// at returns the position of the last word of anchor in body. An anchor locates a use or a
// declaration apart from the same name in a comment.
func at(t *testing.T, body, anchor string) source.Position {
	t.Helper()
	held := strings.Index(body, anchor)
	assert.True(t, held >= 0, "the index of "+anchor)
	name := anchor[strings.LastIndexAny(anchor, " \t*(.")+1:]
	return source.Position{Offset: held + len(anchor) - len(name)}
}

// subject returns the ID of the declaration name of kind in the package at the root.
func subject(name string, kind sema.Kind) sema.ID {
	return sema.NewID(golang.Language, ".", name, kind)
}

func TestEngine(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a declaration without a language", func(t *testing.T) {
			t.Parallel()
			_, err := checker.New(t.TempDir(), lang.Declaration{})
			assert.HasError(t, err, "New without a language")
		})

		t.Run("returns an error for a root that is a file", func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(workspace(t, whole()), "store.go")
			_, err := checker.New(root, golang.Declaration())
			assert.HasError(t, err, "New over a file")
		})

		t.Run("returns an error for a root that does not exist", func(t *testing.T) {
			t.Parallel()
			_, err := checker.New(filepath.Join(t.TempDir(), "absent"), golang.Declaration())
			assert.HasError(t, err, "New over a missing directory")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("returns go/types", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, serving(t, whole()).Name(), "go/types", "Name")
		})
	})

	t.Run("Fidelity", func(t *testing.T) {
		t.Parallel()

		t.Run("returns resolved for relate", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, serving(t, whole()).Fidelity(engine.RoleRelate), trust.Resolved, "Fidelity of relate")
		})

		t.Run("returns none for outline", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, serving(t, whole()).Fidelity(engine.RoleOutline), trust.None, "Fidelity of outline")
		})
	})

	t.Run("Cost", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a session for relate", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, serving(t, whole()).Cost(engine.RoleRelate), engine.CostSession, "Cost of relate")
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil with the go command on PATH", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, serving(t, whole()).Available(t.Context()), "Available")
		})
	})

	t.Run("Binding", func(t *testing.T) {
		t.Parallel()

		t.Run("implements the port of each role that it serves", func(t *testing.T) {
			t.Parallel()
			var held any = serving(t, whole())
			for _, role := range engine.Roles() {
				_, serves := ported(held, role)
				assert.Equal(t, serves, checker.Binding()[role] != trust.None, "the port of "+role.String())
			}
		})
	})
}

// ported returns the port of role that held implements, and reports whether it does.
func ported(held any, role engine.Role) (any, bool) {
	switch role {
	case engine.RoleOutline:
		one, ok := held.(engine.Outliner)
		return one, ok
	case engine.RoleSearch:
		one, ok := held.(engine.Searcher)
		return one, ok
	case engine.RoleResolve:
		one, ok := held.(engine.Resolver)
		return one, ok
	case engine.RoleRelate:
		one, ok := held.(engine.Relator)
		return one, ok
	case engine.RolePlan:
		one, ok := held.(engine.Planner)
		return one, ok
	case engine.RoleFormat:
		one, ok := held.(engine.Formatter)
		return one, ok
	case engine.RoleCheck:
		one, ok := held.(engine.Checker)
		return one, ok
	case engine.RoleVerify:
		one, ok := held.(engine.Verifier)
		return one, ok
	case engine.RoleIndex:
		one, ok := held.(engine.Indexer)
		return one, ok
	}
	return nil, false
}
