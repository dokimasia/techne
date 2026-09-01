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
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: typescript.Declaration(),
		Grammar:     typescript.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"src/service.ts": `export interface Readable {
  get(id: string): number;
}

export class Store implements Readable {
  get(id: string): number { return 1; }
}

export function make(): Store { return new Store(); }

function helper(): number { return 1; }
`,
		},
		// TypeScript spells visibility with an export keyword, which the
		// tags query does not capture, so visibility is unknown.
		Declares: []conformance.Declared{
			{Name: "Readable", Kind: sema.KindInterface, Visibility: sema.VisibilityUnknown},
			{Name: "Store", Kind: sema.KindType, Visibility: sema.VisibilityUnknown},
			{Name: "make", Kind: sema.KindFunction, Visibility: sema.VisibilityUnknown},
			{Name: "helper", Kind: sema.KindFunction, Visibility: sema.VisibilityUnknown},
		},
	})
}
