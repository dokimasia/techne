// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package ruby_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/ruby"
)

// TestDoc runs the suite every language module runs.
//
// The fixture carries one of every declaration form Ruby has,
// because the suite compares the outline against it as a whole set: a
// form left out here is a form nothing checks.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: ruby.Declaration(),
		Grammar:     ruby.Grammar(),
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/store.rb": `require "json"
require_relative "helper"

LIMIT = 10

module Shop
  # Store holds items by name.
  class Store
    attr_reader :name

    def initialize(start)
      @size = start
    end

=begin
scratch notes, which RDoc passes over
=end
    def get(key)
      local = key
      local
    end
  end
end
`,
		},
		Declares: []conformance.Declared{
			{Name: "helper", Kind: sema.KindImport},
			{Name: "json", Kind: sema.KindImport},
			{Name: ":name", Kind: sema.KindProperty},
			{Name: "@size", Kind: sema.KindField},
			{Name: "LIMIT", Kind: sema.KindConstant},
			{Name: "Shop", Kind: sema.KindModule, Signature: "module Shop"},
			{Name: "Store", Kind: sema.KindStruct, Doc: "Store holds items by name."},
			{Name: "get", Kind: sema.KindMethod},
			{Name: "initialize", Kind: sema.KindMethod},
			{Name: "key", Kind: sema.KindParameter},
			{Name: "local", Kind: sema.KindVariable},
			{Name: "start", Kind: sema.KindParameter},
		},
	})
}
