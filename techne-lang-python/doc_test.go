// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package python_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/python"
)

// TestDoc runs the suite every language module runs.
//
// The fixture carries one of every declaration form Python has,
// because the suite compares the outline against it as a whole set: a
// form left out here is a form nothing checks.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: python.Declaration(),
		Grammar:     python.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/service.py": `import os
from typing import List

LIMIT = 10


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
        return "store"


def helper(a, b=1, *rest, **named):
    '''Return the first argument.'''
    return a
`,
		},
		Declares: []conformance.Declared{
			{Name: "LIMIT", Kind: sema.KindVariable, Visibility: sema.Exported},
			{Name: "List", Kind: sema.KindImport, Visibility: sema.Exported},
			{
				Name: "Store", Kind: sema.KindStruct, Visibility: sema.Exported,
				Doc: "Store holds items by name.",
			},
			{Name: "__init__", Kind: sema.KindMethod, Visibility: sema.Unexported},
			{Name: "a", Kind: sema.KindParameter, Visibility: sema.Exported},
			{Name: "b", Kind: sema.KindParameter, Visibility: sema.Exported},
			{
				Name: "get", Kind: sema.KindMethod, Visibility: sema.Exported,
				Doc: "Return the value for key.",
			},
			{
				Name: "helper", Kind: sema.KindFunction, Visibility: sema.Exported,
				Signature: "def helper(a, b=1, *rest, **named)",
				Doc:       "Return the first argument.",
			},
			{Name: "key", Kind: sema.KindParameter, Visibility: sema.Exported},
			{Name: "local", Kind: sema.KindVariable, Visibility: sema.Exported},
			{
				Name: "name", Kind: sema.KindMethod, Visibility: sema.Exported,
				Annotations: []conformance.Annotated{{Name: "property", Text: "@property"}},
			},
			{Name: "named", Kind: sema.KindParameter, Visibility: sema.Exported},
			{Name: "os", Kind: sema.KindImport, Visibility: sema.Exported},
			{Name: "rest", Kind: sema.KindParameter, Visibility: sema.Exported},
			{Name: "self", Kind: sema.KindParameter, Visibility: sema.Exported},
			{Name: "self", Kind: sema.KindParameter, Visibility: sema.Exported},
			{Name: "self", Kind: sema.KindParameter, Visibility: sema.Exported},
			{Name: "size", Kind: sema.KindField, Visibility: sema.Exported},
			{Name: "start", Kind: sema.KindParameter, Visibility: sema.Exported},
		},
	})
}
