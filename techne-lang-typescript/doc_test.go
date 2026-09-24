// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/typescript"
)

// TestDoc runs the conformance suite over a fixture that declares every
// form of TypeScript declaration once. The namespace import declares the
// path and the binding under one name on one line.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: typescript.Declaration(),
		Grammar:     typescript.Grammar(),
		Server:      typescript.Server(),
		Register:    typescript.Register,
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/store.ts": `import { Injectable } from "@nestjs/common";
import * as fs from "fs";

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

export function make<T>(value: T): Promise<T> {
  return Promise.resolve(value);
}
`,
		},
		Declares: []conformance.Declared{
			{Name: "@nestjs/common", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "common"},
			{Name: "Colour", Kind: sema.KindEnum},
			{Name: "Green", Kind: sema.KindEnumMember},
			{Name: "Id", Kind: sema.KindType},
			{Name: "Injectable", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "Injectable"},
			{Name: "LIMIT", Kind: sema.KindConstant},
			{Name: "Readable", Kind: sema.KindInterface, Signature: "export interface Readable"},
			{Name: "Red", Kind: sema.KindEnumMember},
			{
				Name: "Store", Kind: sema.KindStruct,
				Doc:         "Store holds items by name.",
				Annotations: []conformance.Annotated{{Name: "Injectable", Text: "@Injectable()"}},
				Modifiers:   []string{"export"},
			},
			{Name: "T", Kind: sema.KindTypeParameter, Visibility: sema.Unexported},
			{Name: "constructor", Kind: sema.KindConstructor},
			{Name: "counter", Kind: sema.KindVariable},
			{Name: "fs", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "fs"},
			{Name: "fs", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "fs"},
			{Name: "get", Kind: sema.KindMethod},
			{Name: "get", Kind: sema.KindMethod},
			{
				Name: "make", Kind: sema.KindFunction,
				Signature: "export function make<T>(value: T): Promise<T>",
			},
			{Name: "size", Kind: sema.KindField},
			{Name: "size", Kind: sema.KindField},
			{Name: "start", Kind: sema.KindField, Modifiers: []string{"private", "readonly"}},
			{Name: "value", Kind: sema.KindParameter, Visibility: sema.Unexported},
		},
	})
}
