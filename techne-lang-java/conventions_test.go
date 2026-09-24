// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/java"
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
				name: "returns true for a file of the src/test/java tree",
				give: "src/test/java/pkg/Store.java",
				want: true,
			},
			{name: "returns true for a Test.java file", give: "pkg/StoreTest.java", want: true},
			{name: "returns true for a Tests.java file", give: "pkg/StoreTests.java", want: true},
			{name: "returns false for a file of the src/main/java tree", give: "src/main/java/pkg/Store.java"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, java.IsTest(tt.give), tt.want, "the test status of "+tt.give)
			})
		}
	})
}
