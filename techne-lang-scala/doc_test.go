// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package scala_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/scala"
)

// TestDoc runs the conformance suite over a fixture that declares every
// form of Scala declaration once, with each form of import.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: scala.Declaration(),
		Grammar:     scala.Grammar(),
		Server:      scala.Server(),
		Register:    scala.Register,
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/Store.scala": `package shop

import scala.collection.mutable
import scala.util.{Try, Success => Done}
import java.util._

trait Readable {
  def get: Int
}

/** Store holds items by name. */
final class Store(val name: String) extends Readable {
  val limit = 10
  val table: Map[String, Int] = Map(
    "one" -> 1,
    "two" -> 2,
  )
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
			{Name: "Done", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "Done"},
			{Name: "Readable", Kind: sema.KindInterface, Signature: "trait Readable"},
			{
				Name: "Store", Kind: sema.KindStruct,
				Doc:       "Store holds items by name.",
				Modifiers: []string{"final"},
			},
			{Name: "Store", Kind: sema.KindModule, Doc: "Store builds a store."},
			{Name: "Try", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "Try"},
			{Name: "counter", Kind: sema.KindVariable},
			{Name: "get", Kind: sema.KindFunction},
			{Name: "get", Kind: sema.KindFunction},
			{Name: "limit", Kind: sema.KindConstant},
			{
				Name: "table", Kind: sema.KindConstant,
				Signature: "val table: Map[String, Int]",
			},
			{Name: "make", Kind: sema.KindFunction},
			{Name: "mutable", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "mutable"},
			{Name: "n", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "name", Kind: sema.KindProperty},
			{Name: "shop", Kind: sema.KindModule},
			{Name: "util", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "util"},
		},
	})
}
