// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package ruby_test

import (
	"testing"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/conformance"
	"go.dokimi.dev/techne/lang/ruby"
)

// TestDoc runs the conformance suite over a fixture that declares every
// form of Ruby declaration. The classes Store and Cache both declare the
// method get.
func TestDoc(t *testing.T) {
	t.Parallel()

	conformance.Run(t, conformance.Suite{
		Declaration: ruby.Declaration(),
		Grammar:     ruby.Grammar(),
		Server:      ruby.Server(),
		Register:    ruby.Register,
		Unclaimed:   "notes.md",
		Files: map[string]string{
			"pkg/store.rb": `require "json"
require_relative "../lib/helper"

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

  class Cache
    def get
      nil
    end
  end
end
`,
		},
		Declares: []conformance.Declared{
			{Name: "../lib/helper", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "helper"},
			{Name: "json", Kind: sema.KindImport, Visibility: sema.Unexported, Simple: "json"},
			{Name: ":name", Kind: sema.KindProperty},
			{Name: "@size", Kind: sema.KindField},
			{Name: "Cache", Kind: sema.KindStruct, Signature: "class Cache"},
			{Name: "LIMIT", Kind: sema.KindConstant},
			{Name: "Shop", Kind: sema.KindModule, Signature: "module Shop"},
			{Name: "Store", Kind: sema.KindStruct, Doc: "Store holds items by name."},
			{Name: "get", Kind: sema.KindMethod},
			{Name: "get", Kind: sema.KindMethod},
			{Name: "initialize", Kind: sema.KindMethod},
			{Name: "key", Kind: sema.KindParameter, Visibility: sema.Unexported},
			{Name: "local", Kind: sema.KindVariable, Visibility: sema.Unexported},
			{Name: "start", Kind: sema.KindParameter, Visibility: sema.Unexported},
		},
	})
}
