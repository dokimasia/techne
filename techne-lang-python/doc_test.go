// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package python_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/python"
)

// TestDoc runs the conformance suite over a fixture that declares every
// form of Python declaration once.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: python.Declaration(),
		Grammar:     python.Grammar(),
		Server:      python.Server(),
		Register:    python.Register,
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/service.py": `import os
from typing import List

LIMIT = 10
FIRST, SECOND = 1, 2


class Store:
    """Store holds items by name."""

    size = 0

    def __init__(self, start):
        self.size = start

    def get(self, key):
        """Return the value for key."""
        local = key
        return local

    @property
    def name(self):
        """Name the store answers to."""
        return "store"


def helper(a, b=1, *rest, **named):
    '''Return the first argument.'''
    return a
`,
		},
		Declares: []conformance.Declared{
			{
				Name: "FIRST", Kind: sema.KindVariable, Visibility: sema.Exported,
				Signature: "FIRST",
			},
			{Name: "LIMIT", Kind: sema.KindVariable, Visibility: sema.Exported},
			{
				Name: "SECOND", Kind: sema.KindVariable, Visibility: sema.Exported,
				Signature: "SECOND",
			},
			{Name: "List", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "List"},
			{
				Name: "Store", Kind: sema.KindStruct, Visibility: sema.Exported,
				Doc: "Store holds items by name.",
			},
			{Name: "__init__", Kind: sema.KindMethod, Visibility: sema.Exported},
			{Name: "a", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "b", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{
				Name: "get", Kind: sema.KindMethod, Visibility: sema.Exported,
				Doc: "Return the value for key.",
			},
			{
				Name: "helper", Kind: sema.KindFunction, Visibility: sema.Exported,
				Signature: "def helper(a, b=1, *rest, **named)",
				Doc:       "Return the first argument.",
			},
			{Name: "key", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "local", Kind: sema.KindVariable, Visibility: sema.Unexported},
			{
				Name: "name", Kind: sema.KindMethod, Visibility: sema.Exported,
				Doc:         "Name the store answers to.",
				Annotations: []conformance.Annotated{{Name: "property", Text: "@property"}},
			},
			{Name: "named", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "os", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "os"},
			{Name: "rest", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "self", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "self", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "self", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "size", Kind: sema.KindField, Visibility: sema.Exported},
			{Name: "start", Kind: sema.KindParameter, Visibility: sema.Unexported},
		},
	})
}
