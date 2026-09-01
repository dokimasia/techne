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
//
// The fixture carries one of every declaration form Java has,
// along with the documentation, annotations and modifiers written on
// them, because the suite compares the outline against it as a whole
// set: a form left out here is a form nothing checks.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: java.Declaration(),
		Grammar:     java.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/Service.java": `package pkg;

import java.util.List;

/**
 * Readable is anything with a size.
 */
@FunctionalInterface
public interface Readable {
    int get();
}

public enum Colour {
    RED,
    GREEN
}

@Component
public class Store implements Readable {
    public static final int LIMIT = 10;

    private int size;

    public Store(int start) {
        this.size = start;
    }

    /// Returns the number of items.
    @Override
    public int get() {
        return size;
    }

    static <T> T pick(T first, T second) {
        List<T> both = null;
        return first;
    }
}
`,
		},
		Declares: []conformance.Declared{
			{Name: "pkg", Kind: sema.KindPackage},
			{Name: "java.util.List", Kind: sema.KindImport},

			{
				Name: "Readable", Kind: sema.KindInterface,
				Annotations: []conformance.Annotated{{Name: "FunctionalInterface", Text: "@FunctionalInterface"}},
				Doc:         "Readable is anything with a size.",
			},
			{Name: "get", Kind: sema.KindMethod},

			{Name: "Colour", Kind: sema.KindEnum},
			{Name: "RED", Kind: sema.KindEnumMember},
			{Name: "GREEN", Kind: sema.KindEnumMember},

			{
				Name:        "Store",
				Kind:        sema.KindStruct,
				Annotations: []conformance.Annotated{{Name: "Component", Text: "@Component"}},
			},
			{Name: "LIMIT", Kind: sema.KindConstant, Modifiers: []string{"static", "final"}},
			{Name: "size", Kind: sema.KindField, Modifiers: []string{"private"}},

			{Name: "Store", Kind: sema.KindConstructor},
			{Name: "start", Kind: sema.KindParameter},

			{
				Name: "get", Kind: sema.KindMethod,
				Annotations: []conformance.Annotated{{Name: "Override", Text: "@Override"}},
				Doc:         "Returns the number of items.",
			},

			{Name: "pick", Kind: sema.KindMethod, Modifiers: []string{"static"}},
			{Name: "T", Kind: sema.KindTypeParameter},
			{Name: "first", Kind: sema.KindParameter},
			{Name: "second", Kind: sema.KindParameter},
			{Name: "both", Kind: sema.KindVariable},
		},
	})
}
