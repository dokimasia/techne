// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package c_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/c"
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
			{name: "returns true for a _test.c file", give: "src/parser_test.c", want: true},
			{name: "returns true for a test_ file", give: "src/test_parser.c", want: true},
			{name: "returns true for a file in a root tests directory", give: "tests/parser.c", want: true},
			{name: "returns true for a file in a nested tests directory", give: "lib/tests/parser.c", want: true},
			{name: "returns false for a source file", give: "src/parser.c"},
			{name: "returns false for a name that contains test without a separator", give: "src/attest.c"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, c.IsTest(tt.give), tt.want, "the test status of "+tt.give)
			})
		}
	})
}
