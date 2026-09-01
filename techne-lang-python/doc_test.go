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
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: python.Declaration(),
		Grammar:     python.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/service.py": `class Store:
    def get(self, id):
        return 1


def new():
    return Store()


def _helper():
    return 1
`,
		},
		// The vendored query captures classes and functions. A method is
		// a function inside a class, which is why get is a function here
		// rather than a method.
		Declares: []conformance.Declared{
			{Name: "Store", Kind: sema.KindType, Visibility: sema.Exported},
			{Name: "new", Kind: sema.KindFunction, Visibility: sema.Exported},
			{Name: "_helper", Kind: sema.KindFunction, Visibility: sema.Unexported},
		},
	})
}
