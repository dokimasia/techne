// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package javascript_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/javascript"
)

// TestDoc runs the conformance suite over a fixture that declares every
// form of JavaScript declaration. The classes Store and Cache both declare
// the method get.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: javascript.Declaration(),
		Grammar:     javascript.Grammar(),
		Server:      javascript.Server(),
		Register:    javascript.Register,
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/store.js": `import fs from "node:fs";
import { total } from "./cart.js";

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

class Cache {
  get() {
    return counter;
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
			{Name: "./cart.js", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "cart"},
			{Name: "Cache", Kind: sema.KindStruct, Signature: "class Cache"},
			{Name: "LIMIT", Kind: sema.KindConstant},
			{
				Name: "Store", Kind: sema.KindStruct,
				Signature: "export class Store",
				Doc:       "Store holds items by name.",
			},
			{Name: "a", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "b", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "constructor", Kind: sema.KindConstructor},
			{Name: "counter", Kind: sema.KindVariable},
			{Name: "fs", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "fs"},
			{Name: "get", Kind: sema.KindMethod},
			{Name: "get", Kind: sema.KindMethod},
			{
				Name: "helper", Kind: sema.KindFunction,
				Doc:       "Returns the first argument.",
				Modifiers: []string{"export"},
			},
			{Name: "key", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "local", Kind: sema.KindConstant, Visibility: sema.Unexported},
			{Name: "node:fs", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "fs"},
			{Name: "size", Kind: sema.KindField},
			{Name: "start", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "total", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "total"},
		},
	})
}
