// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package scala_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/scala"
)

// TestDoc runs the suite every language module runs.
//
// The fixture carries one of every declaration form Scala has,
// because the suite compares the outline against it as a whole set: a
// form left out here is a form nothing checks.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: scala.Declaration(),
		Grammar:     scala.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/Store.scala": `package shop

import scala.collection.mutable

trait Readable {
  def get: Int
}

/** Store holds items by name. */
final class Store(val name: String) extends Readable {
  val limit = 10
  var counter = 0

  def get: Int = limit
}

/**
  * Store builds a store.
  */
object Store {
  def make(n: String): Store = new Store(n)
}
`,
		},
		Declares: []conformance.Declared{
			{Name: "Readable", Kind: sema.KindInterface, Signature: "trait Readable"},
			{
				Name: "Store", Kind: sema.KindStruct,
				Doc:       "Store holds items by name.",
				Modifiers: []string{"final"},
			},
			{Name: "Store", Kind: sema.KindStruct, Doc: "Store builds a store."},
			{Name: "counter", Kind: sema.KindVariable},
			{Name: "get", Kind: sema.KindFunction},
			{Name: "get", Kind: sema.KindFunction},
			{Name: "limit", Kind: sema.KindConstant},
			{Name: "make", Kind: sema.KindFunction},
			{Name: "n", Kind: sema.KindParameter},
			{Name: "name", Kind: sema.KindProperty},
			{Name: "scala", Kind: sema.KindImport},
			{Name: "shop", Kind: sema.KindModule},
		},
	})
}
