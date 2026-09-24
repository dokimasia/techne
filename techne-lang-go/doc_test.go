// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	golang "go.dokimi.dev/techne/lang/go"
)

// TestDoc runs the conformance suite over a fixture that declares every
// form of Go declaration. The struct types Store and Cache both declare a
// field n and a method Get.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: golang.Declaration(),
		Grammar:     golang.Grammar(),
		Server:      golang.Server(),
		Register:    golang.Register,
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/service.go": `package pkg

import (
	"encoding/json"
	"fmt"
)

// Store holds items by name.
type Store struct {
	n int
	// Name is what a caller looks it up by.
	Name string ` + "`json:\"name\" doc:\"the display name\"`" + `
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

// Get returns the count.
func (s *Store) Get() int { return s.n }

type Cache struct{ n int }

func (c *Cache) Get() int { return c.n }

func helper() int {
	local := 1
	return local
}

// Identity returns its argument.
// It is generic over T.
func Identity[T any](in T) T { return in }

var _ = fmt.Sprint
var _ = json.Marshal
`,
		},
		Declares: []conformance.Declared{
			{Name: "Cache", Kind: sema.KindStruct, Visibility: sema.Exported},
			{Name: "First", Kind: sema.KindConstant, Visibility: sema.Exported},
			{
				Name: "Get", Kind: sema.KindMethod, Visibility: sema.Exported,
				Doc: "Get returns the count.",
			},
			{Name: "Get", Kind: sema.KindMethod, Visibility: sema.Exported, Signature: "func (c *Cache) Get() int"},
			{Name: "ID", Kind: sema.KindType, Visibility: sema.Exported},
			{
				Name: "Identity", Kind: sema.KindFunction, Visibility: sema.Exported,
				Doc: "Identity returns its argument.\nIt is generic over T.",
			},
			{
				Name: "Name", Kind: sema.KindField, Visibility: sema.Exported,
				Doc: "Name is what a caller looks it up by.",
				Annotations: []conformance.Annotated{
					{Name: "json", Text: `json:"name"`},
					{Name: "doc", Text: `doc:"the display name"`},
				},
			},
			{
				Name: "Read", Kind: sema.KindMethod, Visibility: sema.Exported,
				Signature: "Read() int",
			},
			{Name: "Reader", Kind: sema.KindInterface, Visibility: sema.Exported},
			{
				Name: "Registry", Kind: sema.KindVariable, Visibility: sema.Exported,
				Doc: "Registry is the process-wide index.",
			},
			{Name: "Store", Kind: sema.KindStruct, Visibility: sema.Exported, Doc: "Store holds items by name."},
			{Name: "T", Kind: sema.KindTypeParameter, Visibility: sema.Unexported},
			{Name: "c", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "encoding/json", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "json"},
			{Name: "fmt", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "fmt"},
			{Name: "helper", Kind: sema.KindFunction, Visibility: sema.Unexported},
			{Name: "in", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "local", Kind: sema.KindVariable, Visibility: sema.Unexported},
			{Name: "n", Kind: sema.KindField, Visibility: sema.Unexported},
			{Name: "n", Kind: sema.KindField, Visibility: sema.Unexported},
			{Name: "s", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "second", Kind: sema.KindConstant, Visibility: sema.Unexported},
		},
	})
}
