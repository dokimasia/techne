// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package ruby_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/ruby"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("IsTest", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "returns true for a _spec.rb file", give: "lib/store_spec.rb", want: true},
			{name: "returns true for a _test.rb file", give: "test/store_test.rb", want: true},
			{name: "returns true for a file in a root spec directory", give: "spec/support/helpers.rb", want: true},
			{
				name: "returns true for a file in a nested spec directory",
				give: "engines/shop/spec/store.rb",
				want: true,
			},
			{name: "returns false for a source file", give: "lib/store.rb"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, ruby.IsTest(tt.give), tt.want, "the test status of "+tt.give)
			})
		}
	})
}
