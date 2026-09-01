// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/java"
)

// TestDoc runs the suite every language module runs.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: java.Declaration(),
		Grammar:     java.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/Store.java": `package pkg;

public interface Readable {
    int get(String id);
}

public class Store implements Readable {
    public int get(String id) { return 1; }

    private int helper() { return 1; }
}
`,
		},
		// Java spells visibility as a modifier, which a tags query does
		// not capture, so every declaration is reported unknown rather
		// than guessed as public.
		Declares: []conformance.Declared{
			{Name: "Readable", Kind: sema.KindInterface, Visibility: sema.VisibilityUnknown},
			{Name: "Store", Kind: sema.KindType, Visibility: sema.VisibilityUnknown},
			{Name: "get", Kind: sema.KindMethod, Visibility: sema.VisibilityUnknown},
			{Name: "helper", Kind: sema.KindMethod, Visibility: sema.VisibilityUnknown},
		},
	})
}
