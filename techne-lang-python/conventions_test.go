// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package python_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/python"
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
			{name: "returns true for a test_ file", give: "tests/test_store.py", want: true},
			{name: "returns true for a _test.py file", give: "pkg/store_test.py", want: true},
			{name: "returns false for a source file", give: "pkg/store.py"},
			{name: "returns false for a name that starts with test without an underscore", give: "pkg/testing.py"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, python.IsTest(tt.give), tt.want, "the test status of "+tt.give)
			})
		}
	})

	t.Run("Visibility", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want sema.Visibility
		}{
			{name: "returns Exported for a name without a leading underscore", give: "Store", want: sema.Exported},
			{name: "returns Unexported for a name with a leading underscore", give: "_helper", want: sema.Unexported},
			{
				name: "returns Unexported for a name with two leading underscores",
				give: "__cache",
				want: sema.Unexported,
			},
			{name: "returns Exported for a special name", give: "__init__", want: sema.Exported},
			{name: "returns Unexported for a name of underscores only", give: "____", want: sema.Unexported},
			{name: "returns VisibilityUnknown for an empty name", give: "", want: sema.VisibilityUnknown},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, python.Visibility(tt.give), tt.want, "the visibility of "+tt.give)
			})
		}
	})
}
