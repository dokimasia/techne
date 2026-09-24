// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package scala_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/scala"
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
				name: "returns true for a file of the src/test/scala tree",
				give: "src/test/scala/Store.scala",
				want: true,
			},
			{name: "returns true for a Spec.scala file", give: "app/StoreSpec.scala", want: true},
			{name: "returns true for a Test.scala file", give: "app/StoreTest.scala", want: true},
			{name: "returns false for a file of the src/main/scala tree", give: "src/main/scala/Store.scala"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, scala.IsTest(tt.give), tt.want, "the test status of "+tt.give)
			})
		}
	})
}
