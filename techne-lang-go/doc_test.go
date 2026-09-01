// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	golang "go.dokimi.dev/techne/lang/go"
)

// TestDoc runs the suite every language module runs.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: golang.Declaration(),
		Grammar:     golang.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/service.go": `package pkg

type Store struct{ n int }

func (s *Store) Get(id string) int { return s.n }

func New() *Store { return &Store{} }

func helper() int { return 1 }
`,
		},
		Declares: []conformance.Declared{
			{Name: "Store", Kind: sema.KindType, Visibility: sema.Exported},
			{Name: "Get", Kind: sema.KindMethod, Visibility: sema.Exported},
			{Name: "New", Kind: sema.KindFunction, Visibility: sema.Exported},
			{Name: "helper", Kind: sema.KindFunction, Visibility: sema.Unexported},
		},
	})
}
