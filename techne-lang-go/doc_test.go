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
//
// The fixture carries one of every declaration form Go has,
// because the suite compares the outline against it as a whole set: a
// form left out here is a form nothing checks.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: golang.Declaration(),
		Grammar:     golang.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/service.go": `package pkg

import "fmt"

// Store holds items by name.
type Store struct {
	n int
	// Name is what a caller looks it up by.
	Name string ` + "`json:\"name\"`" + `
}

type Reader interface{ Read() int }

type ID = string

const (
	First = iota
	second
)

/*
Registry is the process-wide index.
*/
var Registry = map[string]int{}

func (s *Store) Get() int { return s.n }

func helper() int {
	local := 1
	return local
}

// Identity returns its argument.
// It is generic over T.
func Identity[T any](in T) T { return in }

var _ = fmt.Sprint
`,
		},
		Declares: []conformance.Declared{
			{Name: "First", Kind: sema.KindConstant, Visibility: sema.Exported},
			{Name: "Get", Kind: sema.KindMethod, Visibility: sema.Exported},
			{Name: "ID", Kind: sema.KindType, Visibility: sema.Exported},
			{
				Name: "Identity", Kind: sema.KindFunction, Visibility: sema.Exported,
				Doc: "Identity returns its argument.\nIt is generic over T.",
			},
			{
				Name: "Name", Kind: sema.KindField, Visibility: sema.Exported,
				Doc:         "Name is what a caller looks it up by.",
				Annotations: []string{"json"},
			},
			{Name: "Read", Kind: sema.KindMethod, Visibility: sema.Exported},
			{Name: "Reader", Kind: sema.KindInterface, Visibility: sema.Exported},
			{
				Name: "Registry", Kind: sema.KindVariable, Visibility: sema.Exported,
				Doc: "Registry is the process-wide index.",
			},
			{Name: "Store", Kind: sema.KindStruct, Visibility: sema.Exported, Doc: "Store holds items by name."},
			{Name: "T", Kind: sema.KindTypeParameter, Visibility: sema.Exported},
			{Name: "fmt", Kind: sema.KindImport, Visibility: sema.Unexported},
			{Name: "helper", Kind: sema.KindFunction, Visibility: sema.Unexported},
			{Name: "in", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "local", Kind: sema.KindVariable, Visibility: sema.Unexported},
			{Name: "n", Kind: sema.KindField, Visibility: sema.Unexported},
			{Name: "s", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "second", Kind: sema.KindConstant, Visibility: sema.Unexported},
		},
	})
}
