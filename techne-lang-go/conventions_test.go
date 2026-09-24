// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	golang "go.dokimi.dev/techne/lang/go"
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
			{name: "returns true for a _test.go file", give: "pkg/store_test.go", want: true},
			{name: "returns false for a source file", give: "pkg/store.go"},
			{name: "returns false for a name that starts with test", give: "pkg/testing.go"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, golang.IsTest(tt.give), tt.want, "the test status of "+tt.give)
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
			{name: "returns Exported for an upper-case initial", give: "Store", want: sema.Exported},
			{name: "returns Exported for an upper-case initial outside ASCII", give: "Ωmega", want: sema.Exported},
			{name: "returns Unexported for a lower-case initial", give: "helper", want: sema.Unexported},
			{name: "returns Unexported for an underscore initial", give: "_Store", want: sema.Unexported},
			{name: "returns VisibilityUnknown for an empty name", give: "", want: sema.VisibilityUnknown},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, golang.Visibility(tt.give), tt.want, "the visibility of "+tt.give)
			})
		}
	})

	t.Run("Unit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the directory of a file", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Unit("core/trust/status.go"), "core/trust", "the unit of core/trust/status.go")
		})

		t.Run("returns the root for a file at the root", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, golang.Unit("main.go"), ".", "the unit of main.go")
		})
	})
}
