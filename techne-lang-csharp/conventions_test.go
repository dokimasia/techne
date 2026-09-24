// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package csharp_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/csharp"
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
			{name: "returns true for a Tests.cs file", give: "src/StoreTests.cs", want: true},
			{name: "returns true for a Test.cs file", give: "src/StoreTest.cs", want: true},
			{name: "returns true for a file in a .Tests project", give: "Api.Tests/StoreFixture.cs", want: true},
			{name: "returns false for a source file", give: "src/Store.cs"},
			{name: "returns false for a name that contains Test before its end", give: "src/Testing.cs"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, csharp.IsTest(tt.give), tt.want, "the test status of "+tt.give)
			})
		}
	})
}
