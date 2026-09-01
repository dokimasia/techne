// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package javascript_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/javascript"
)

// TestDoc runs the suite every language module runs.
//
// The fixture carries one of every declaration form JavaScript has,
// because the suite compares the outline against it as a whole set: a
// form left out here is a form nothing checks.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: javascript.Declaration(),
		Grammar:     javascript.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/store.js": `import fs from "node:fs";

export const LIMIT = 10;
let counter = 0;

/** Store holds items by name. */
export class Store {
  size = 0;

  constructor(start) {
    this.size = start;
  }

  get(key) {
    const local = key;
    return local;
  }
}

/**
 * Returns the first argument.
 */
export function helper(a, b) {
  return a;
}
`,
		},
		Declares: []conformance.Declared{
			{Name: "LIMIT", Kind: sema.KindConstant},
			{Name: "Store", Kind: sema.KindStruct, Doc: "Store holds items by name."},
			{Name: "a", Kind: sema.KindParameter},
			{Name: "b", Kind: sema.KindParameter},
			{Name: "constructor", Kind: sema.KindConstructor},
			{Name: "counter", Kind: sema.KindVariable},
			{Name: "get", Kind: sema.KindMethod},
			{
				Name: "helper", Kind: sema.KindFunction,
				Doc:       "Returns the first argument.",
				Modifiers: []string{"export"},
			},
			{Name: "key", Kind: sema.KindParameter},
			{Name: "local", Kind: sema.KindConstant},
			{Name: "node:fs", Kind: sema.KindImport},
			{Name: "size", Kind: sema.KindField},
			{Name: "start", Kind: sema.KindParameter},
		},
	})
}
