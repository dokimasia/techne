// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package csharp_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/csharp"
)

// TestDoc runs the conformance suite over a fixture that declares every
// form of C# declaration once.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: csharp.Declaration(),
		Grammar:     csharp.Grammar(),
		Server:      csharp.Server(),
		Register:    csharp.Register,
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/Store.cs": `using System;
using System.Collections.Generic;

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
			{
				Name: "Get", Kind: sema.KindMethod,
				Signature: "public int Get()",
				Doc:       "Returns the number of items.",
			},
			{Name: "Get", Kind: sema.KindMethod},
			{Name: "Green", Kind: sema.KindEnumMember},
			{Name: "IReadable", Kind: sema.KindInterface},
			{Name: "Limit", Kind: sema.KindConstant, Modifiers: []string{"public", "const"}},
			{Name: "Red", Kind: sema.KindEnumMember},
			{Name: "Shop", Kind: sema.KindModule},
			{Name: "Size", Kind: sema.KindProperty},
			{Name: "Store", Kind: sema.KindConstructor},
			{
				Name: "Store",
				Kind: sema.KindStruct,
				Doc:  "Store holds items by name.",
				Annotations: []conformance.Annotated{
					{Name: "Serializable", Text: "Serializable"},
					{Name: "Obsolete", Text: "Obsolete"},
				},
				Modifiers: []string{"public"},
			},
			{Name: "System", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "System"},
			{
				Name: "System.Collections.Generic", Kind: sema.KindImport, Visibility: sema.Unexported,
				Simple: "Generic",
			},
			{Name: "local", Kind: sema.KindVariable, Visibility: sema.Unexported},
			{Name: "size", Kind: sema.KindField},
			{Name: "start", Kind: sema.KindParameter, Visibility: sema.Unexported},
		},
	})
}
