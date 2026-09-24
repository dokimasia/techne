// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/rust"
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
			{
				name: "returns true for a file in the tests directory of a package",
				give: "tests/integration.rs",
				want: true,
			},
			{
				name: "returns true for a file in the tests directory of a member",
				give: "crates/api/tests/routes.rs",
				want: true,
			},
			{name: "returns true for a file named tests.rs", give: "src/store/tests.rs", want: true},
			{name: "returns false for a source file with a test module", give: "src/store.rs"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, rust.IsTest(tt.give), tt.want, "the test status of "+tt.give)
			})
		}
	})
}
