// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/typescript"
)

// TestDoc runs the suite every language module runs.
//
// The fixture carries one of every declaration form TypeScript has,
// because the suite compares the outline against it as a whole set: a
// form left out here is a form nothing checks.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: typescript.Declaration(),
		Grammar:     typescript.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/store.ts": `import { Injectable } from "@nestjs/common";

export const LIMIT = 10;
let counter = 0;

/// <reference types="node" />
export type Id = string;

export interface Readable {
  size: number;
  get(): number;
}

export enum Colour {
  Red,
  Green,
}

/**
 * Store holds items by name.
 */
@Injectable()
export class Store implements Readable {
  size: number = 0;

  constructor(private readonly start: number) {}

  get(): number {
    return this.size;
  }
}

export function make<T>(value: T): T {
  return value;
}
`,
		},
		Declares: []conformance.Declared{
			{Name: "@nestjs/common", Kind: sema.KindImport},
			{Name: "Colour", Kind: sema.KindEnum},
			{Name: "Green", Kind: sema.KindEnumMember},
			{Name: "Id", Kind: sema.KindType},
			{Name: "Injectable", Kind: sema.KindImport},
			{Name: "LIMIT", Kind: sema.KindConstant},
			{Name: "Readable", Kind: sema.KindInterface},
			{Name: "Red", Kind: sema.KindEnumMember},
			{
				Name: "Store", Kind: sema.KindStruct,
				Doc:         "Store holds items by name.",
				Annotations: []string{"Injectable"},
				Modifiers:   []string{"export"},
			},
			{Name: "T", Kind: sema.KindTypeParameter},
			{Name: "constructor", Kind: sema.KindConstructor},
			{Name: "counter", Kind: sema.KindVariable},
			{Name: "get", Kind: sema.KindMethod},
			{Name: "get", Kind: sema.KindMethod},
			{Name: "make", Kind: sema.KindFunction},
			{Name: "size", Kind: sema.KindField},
			{Name: "size", Kind: sema.KindField},
			{Name: "start", Kind: sema.KindField, Modifiers: []string{"private", "readonly"}},
			{Name: "value", Kind: sema.KindParameter},
		},
	})
}
