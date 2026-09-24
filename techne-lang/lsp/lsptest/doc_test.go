// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest_test

import (
	"os"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// TestMain runs the scripted server when a test starts this binary as one, and the tests
// otherwise.
func TestMain(m *testing.M) { lsptest.Main(m) }

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("runs the current test binary", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, lsptest.Server(lsptest.Default).Command, []string{os.Args[0]},
				"the command of the scripted server")
		})
	})

	t.Run("Workspace", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the fixtures that the script describes", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"a.fake": lsptest.Content})
			content, err := os.ReadFile(root + "/a.fake")
			assert.NoError(t, err, "the test reads a.fake")
			assert.Equal(t, string(content), lsptest.Content, "the content of a.fake")
		})
	})
}
