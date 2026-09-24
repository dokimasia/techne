// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package c_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/c"
	"go.dokimi.dev/techne/lang/conformance"
)

// TestDoc runs the conformance suite over a fixture that declares every
// form of C declaration once.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: c.Declaration(),
		Grammar:     c.Grammar(),
		Server:      c.Server(),
		Register:    c.Register,
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/store.c": `#include <stdio.h>
#include "store.h"

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

/// Returns a pointer to its argument.
static int *
borrow(int *a) {
    return a;
}

/// Returns its argument.
static int helper(int a) {
    int local = a;
    return local;
}

/// Reads a store, which names its type without declaring it.
static int read_store(struct Store *held, enum Colour tint) {
    return held->n + tint;
}
`,
		},
		Declares: []conformance.Declared{
			{Name: "<stdio.h>", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "stdio"},
			{Name: "store.h", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "store"},
			{Name: "Bits", Kind: sema.KindUnion},
			{Name: "Colour", Kind: sema.KindEnum},
			{Name: "GREEN", Kind: sema.KindEnumMember},
			{Name: "LIMIT", Kind: sema.KindConstant},
			{Name: "RED", Kind: sema.KindEnumMember},
			{Name: "SQUARE", Kind: sema.KindMacro, Signature: "#define SQUARE(x) ((x) * (x))"},
			{Name: "Store", Kind: sema.KindStruct, Doc: "Store holds a count."},
			{Name: "Store", Kind: sema.KindType, Doc: "Store holds a count."},
			{Name: "a", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "a", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{
				Name: "borrow", Kind: sema.KindFunction,
				Signature: "static int * borrow(int *a)",
				Doc:       "Returns a pointer to its argument.",
			},
			{
				Name: "helper", Kind: sema.KindFunction,
				Doc:       "Returns its argument.",
				Modifiers: []string{"static"},
			},
			{Name: "held", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "local", Kind: sema.KindVariable, Visibility: sema.Unexported},
			{
				Name: "read_store", Kind: sema.KindFunction,
				Doc:       "Reads a store, which names its type without declaring it.",
				Modifiers: []string{"static"},
			},
			{Name: "tint", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "n", Kind: sema.KindField},
			{Name: "raw", Kind: sema.KindField},
		},
	})
}
