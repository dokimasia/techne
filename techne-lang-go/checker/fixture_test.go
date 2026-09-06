// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/go/checker"
)

// The cases load a real module with the real go command, because that is
// where the interesting answers are: which files a package holds under
// which build tags is the toolchain's answer, and a fixture that faked
// it would be testing the fake.
//
// The module is tiny, so a load is a fraction of a second.

// store is a type with a method, used from a second file, and an
// interface the type satisfies. It is enough to ask every direction of.
const store = `package p

// Store holds a size.
type Store struct {
	size  int
	Named string
}

// Total is the whole of it.
func (s *Store) Total() int { return s.size + helper() }

func helper() int { return 1 }

type Reader interface{ Sum() int }

// Sum is what the interface asks for.
func (s *Store) Sum() int { return s.Total() }

type Wrapped struct {
	Store
	extra int
}
`

const use = `package p

func Use() int {
	held := Store{size: 3}
	return held.Total()
}
`

// broken does not compile: the field does not exist.
const broken = `package p

func Broken() int {
	held := Store{}
	return held.missing
}
`

// workspace writes a module and returns the directory holding it.
func workspace(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	held := map[string]string{"go.mod": "module example.com/p\n\ngo 1.24\n"}
	maps.Copy(held, files)
	for name, body := range held {
		path := filepath.Join(dir, name)
		assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755), "the case can lay the module out")
		assert.NoError(t, os.WriteFile(path, []byte(body), 0o644),
			"the case can prepare the workspace")
	}
	return dir
}

// serving builds an engine over a module, and drops what it loaded when
// the case is done.
func serving(t *testing.T, files map[string]string) *checker.Engine {
	t.Helper()
	e, err := checker.New(workspace(t, files), golang.Declaration())
	assert.NoError(t, err, "an engine builds over a module on disk")
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer stop()
		assert.NoError(t, e.Close(ctx), "closing drops what was type-checked")
	})
	return e
}

// whole is the module every case that does not need a fault uses.
func whole() map[string]string {
	return map[string]string{"store.go": store, "use.go": use}
}

// edges is what an answer pointed at, in order.
func edges(held []sema.Relation) []string {
	out := make([]string, 0, len(held))
	for _, one := range held {
		out = append(out, one.To.Name)
	}
	return out
}

// names is what an answer found, in order.
func names(held []sema.Symbol) []string {
	out := make([]string, 0, len(held))
	for _, one := range held {
		out = append(out, one.Name)
	}
	return out
}

// carries reports whether an answer named a caveat.
func carries(held []trust.Caveat, code trust.CaveatCode) bool {
	for _, one := range held {
		if one.Code == code {
			return true
		}
	}
	return false
}

// at is where the last word of an anchor is written, which is what a
// caller pointing at a name sends.
//
// An anchor rather than the name alone, because a name is written in the
// documentation above the declaration as often as in the declaration,
// and a case pointing at prose is asking a question with no answer.
func at(t *testing.T, body, anchor string) source.Position {
	t.Helper()
	held := strings.Index(body, anchor)
	assert.True(t, held >= 0, "the case can point at what it is about: "+anchor)

	name := anchor[strings.LastIndexAny(anchor, " \t*(.")+1:]
	return source.Position{Offset: held + len(anchor) - len(name)}
}
