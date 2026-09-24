// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock_test

import (
	"os"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang/mock"
)

// example returns the code block of the package documentation: the lines of doc.go that gofmt
// writes as //, a tab and the code.
func example(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile("doc.go")
	assert.NoError(t, err, "ReadFile of doc.go")
	var out strings.Builder
	for line := range strings.Lines(string(content)) {
		if code, found := strings.CutPrefix(line, "//\t"); found {
			out.WriteString(code)
		}
	}
	return out.String()
}

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Engine", func(t *testing.T) {
		t.Parallel()

		t.Run("serves the roles that the package documentation lists", func(t *testing.T) {
			t.Parallel()
			listed := map[engine.Role]bool{
				engine.RoleOutline: true, engine.RoleSearch: true, engine.RoleResolve: true,
				engine.RoleRelate: true, engine.RolePlan: true, engine.RoleCheck: true, engine.RoleVerify: true,
			}
			catalogue := engine.NewCatalog()
			assert.NoError(t, catalogue.Add(built(t)), "Add of the engine")
			for _, role := range engine.Roles() {
				served := len(catalogue.For(t.Context(), mock.Language, role)) == 1
				assert.Equal(t, served, listed[role], "the engine of the role "+role.String())
			}
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("reads every line of the example of the package documentation", func(t *testing.T) {
			t.Parallel()
			code := example(t)
			assert.NotEmpty(t, code, "the example of the package documentation")
			lines, broken := mock.Parse("example.mock", []byte(code))
			assert.Empty(t, broken, "the lines of the example that the language does not have")
			assert.Length(t, lines, 4, "the declarations and the uses of the example")
		})
	})
}
