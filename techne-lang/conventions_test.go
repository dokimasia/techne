// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang"
)

func TestConventions(t *testing.T) {
	t.Parallel()

	t.Run("Stem", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "removes the extension", give: "a/b/c.py", want: "a/b/c"},
			{name: "removes only the last extension", give: "a/b.test.ts", want: "a/b.test"},
			{name: "returns a path without an extension unchanged", give: "a/Makefile", want: "a/Makefile"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, lang.Stem(tt.give), tt.want, "the stem of "+tt.give)
			})
		}
	})

	t.Run("VisibilityByModifier", func(t *testing.T) {
		t.Parallel()

		t.Run("returns VisibilityUnknown for every name", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"Exported", "unexported", "_private", ""} {
				assert.Equal(t, lang.VisibilityByModifier(name), sema.VisibilityUnknown, "the visibility of "+name)
			}
		})
	})

	t.Run("JavaScriptTest", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "returns true for a .test file", give: "src/app.test.ts", want: true},
			{name: "returns true for a .spec file", give: "src/app.spec.jsx", want: true},
			{name: "returns true for a -test file", give: "src/app-test.mjs", want: true},
			{name: "returns true for a _test file", give: "src/app_test.cjs", want: true},
			{name: "returns true for a test- file", give: "src/test-app.js", want: true},
			{name: "returns true for a file named test", give: "src/test.js", want: true},
			{name: "returns true for a file named spec", give: "src/spec.tsx", want: true},
			{name: "returns true for a file in a root test directory", give: "test/app.js", want: true},
			{
				name: "returns true for a file below a nested test directory",
				give: "packages/a/test/unit/app.mts",
				want: true,
			},
			{name: "returns true for a file in a root __tests__ directory", give: "__tests__/app.ts", want: true},
			{name: "returns true for a file in a nested __tests__ directory", give: "src/__tests__/app.ts", want: true},
			{name: "returns false for a source file", give: "src/app.ts"},
			{name: "returns false for a name that contains test without a separator", give: "src/latest.ts"},
			{name: "returns false for a directory whose name contains test", give: "src/tests/app.js"},
			{name: "returns false for a declaration file of a test module", give: "src/app.test.d.ts"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, lang.JavaScriptTest(tt.give), tt.want, "the test status of "+tt.give)
			})
		}
	})
}
