// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package csharp_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/csharp"
)

// TestDoc runs the suite every language module runs.
//
// The fixture carries one of every declaration form C# has,
// because the suite compares the outline against it as a whole set: a
// form left out here is a form nothing checks.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: csharp.Declaration(),
		Grammar:     csharp.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/Store.cs": `using System;

namespace Shop
{
    public interface IReadable
    {
        int Get();
    }

    public enum Colour
    {
        Red,
        Green,
    }

    /// <summary>Store holds items by name.</summary>
    [Serializable, Obsolete]
    public class Store : IReadable
    {
        public const int Limit = 10;

        private int size;

        public int Size { get; set; }

        public Store(int start)
        {
            size = start;
        }

        /**
         * Returns the number of items.
         */
        public int Get()
        {
            int local = size;
            return local;
        }
    }
}
`,
		},
		Declares: []conformance.Declared{
			{Name: "Colour", Kind: sema.KindEnum},
			{Name: "Get", Kind: sema.KindMethod, Doc: "Returns the number of items."},
			{Name: "Get", Kind: sema.KindMethod},
			{Name: "Green", Kind: sema.KindEnumMember},
			{Name: "IReadable", Kind: sema.KindInterface},
			{Name: "Limit", Kind: sema.KindConstant, Modifiers: []string{"public", "const"}},
			{Name: "Red", Kind: sema.KindEnumMember},
			{Name: "Shop", Kind: sema.KindModule},
			{Name: "Size", Kind: sema.KindProperty},
			{Name: "Store", Kind: sema.KindConstructor},
			{
				Name: "Store", Kind: sema.KindStruct,
				Doc:         "<summary>Store holds items by name.</summary>",
				Annotations: []string{"Serializable", "Obsolete"},
				Modifiers:   []string{"public"},
			},
			{Name: "System", Kind: sema.KindImport},
			{Name: "local", Kind: sema.KindVariable},
			{Name: "size", Kind: sema.KindField},
			{Name: "start", Kind: sema.KindParameter},
		},
	})
}
