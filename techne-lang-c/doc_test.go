// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package c_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/c"
	"go.dokimi.dev/techne/lang/conformance"
)

// TestDoc runs the suite every language module runs.
//
// The fixture carries one of every declaration form C has,
// because the suite compares the outline against it as a whole set: a
// form left out here is a form nothing checks.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: c.Declaration(),
		Grammar:     c.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/store.c": `#include <stdio.h>

#define LIMIT 10
#define SQUARE(x) ((x) * (x))

/**
 * Store holds a count.
 */
typedef struct Store {
    int n;
} Store;

enum Colour { RED, GREEN };

union Bits {
    int raw;
};

/// Returns its argument.
static int helper(int a) {
    int local = a;
    return local;
}
`,
		},
		Declares: []conformance.Declared{
			{Name: "<stdio.h>", Kind: sema.KindImport},
			{Name: "Bits", Kind: sema.KindUnion},
			{Name: "Colour", Kind: sema.KindEnum},
			{Name: "GREEN", Kind: sema.KindEnumMember},
			{Name: "LIMIT", Kind: sema.KindConstant},
			{Name: "RED", Kind: sema.KindEnumMember},
			{Name: "SQUARE", Kind: sema.KindMacro, Signature: "#define SQUARE(x) ((x) * (x))"},
			{Name: "Store", Kind: sema.KindStruct, Doc: "Store holds a count."},
			{Name: "Store", Kind: sema.KindType, Doc: "Store holds a count."},
			{Name: "a", Kind: sema.KindParameter},
			{
				Name: "helper", Kind: sema.KindFunction,
				Doc:       "Returns its argument.",
				Modifiers: []string{"static"},
			},
			{Name: "local", Kind: sema.KindVariable},
			{Name: "n", Kind: sema.KindField},
			{Name: "raw", Kind: sema.KindField},
		},
	})
}
