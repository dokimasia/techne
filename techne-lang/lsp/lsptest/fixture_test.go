// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// line returns line n of text, without its line ending.
func line(text string, n int) string { return strings.Split(text, "\n")[n] }

func TestFixture(t *testing.T) {
	t.Parallel()

	t.Run("Content", func(t *testing.T) {
		t.Parallel()

		t.Run("declares Store on line 2", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, line(lsptest.Content, 2)[5:10], "Store", "line 2, characters 5 to 10")
		})

		t.Run("declares the method Get on line 6", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, line(lsptest.Content, 6)[16:19], "Get", "line 6, characters 16 to 19")
		})

		t.Run("declares After on line 8 with a call of Get", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, line(lsptest.Content, 8)[5:10], "After", "line 8, characters 5 to 10")
			assert.Equal(t, line(lsptest.Content, 8)[37:40], "Get", "line 8, characters 37 to 40")
		})

		t.Run("uses Store on line 6", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, line(lsptest.Content, 6)[9:14], "Store", "line 6, characters 9 to 14")
		})

		t.Run("uses Store on line 8", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, line(lsptest.Content, 8)[28:33], "Store", "line 8, characters 28 to 33")
		})
	})

	t.Run("Emoji", func(t *testing.T) {
		t.Parallel()

		t.Run("starts Störe at byte 19 of line 2", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, line(lsptest.Emoji, 2)[19:25], "Störe", "line 2, bytes 19 to 25")
		})

		t.Run("starts Störe at code unit 17 of line 2", func(t *testing.T) {
			t.Parallel()
			units := utf16.Encode([]rune(line(lsptest.Emoji, 2)))
			assert.Equal(t, string(utf16.Decode(units[17:22])), "Störe", "line 2, code units 17 to 22")
		})

		t.Run("starts Störe at byte 30 of the file", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, strings.Index(lsptest.Emoji, "Störe"), 30, "the offset of Störe")
		})
	})

	t.Run("Faulty", func(t *testing.T) {
		t.Parallel()

		t.Run("contains Broken once", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, strings.Count(lsptest.Faulty, lsptest.Broken), 1, "the occurrences of Broken")
		})
	})

	t.Run("Twins", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the Get of Store at character 16 of line 4", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, line(lsptest.Twins, 4)[16:19], "Get", "line 4, characters 16 to 19")
			assert.Contains(t, line(lsptest.Twins, 4), "*Store", "the receiver on line 4")
		})

		t.Run("declares the Get of Cache at character 16 of line 8", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, line(lsptest.Twins, 8)[16:19], "Get", "line 8, characters 16 to 19")
			assert.Contains(t, line(lsptest.Twins, 8), "*Cache", "the receiver on line 8")
		})
	})

	t.Run("Bundle", func(t *testing.T) {
		t.Parallel()

		t.Run("declares n functions before After on line 1", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, strings.Count(line(lsptest.Bundle(3), 1), "func "), 4, "the functions on line 1")
		})

		t.Run("declares F0 at character 5 of line 1", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, line(lsptest.Bundle(3), 1)[5:7], "F0", "line 1, characters 5 to 7")
		})

		t.Run("ends line 1 with a call of F0", func(t *testing.T) {
			t.Parallel()
			assert.True(t, strings.HasSuffix(line(lsptest.Bundle(3), 1), "return F0() }"), "the end of line 1")
		})
	})

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("claims the extension of the language", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, lsptest.Declaration().Extensions, []string{lsptest.Extension}, "the extensions")
		})

		t.Run("treats the file Test as a test", func(t *testing.T) {
			t.Parallel()
			d := lsptest.Declaration()
			assert.True(t, d.IsTest("pkg/"+lsptest.Test), "IsTest of "+lsptest.Test)
			assert.False(t, d.IsTest("pkg/a.fake"), "IsTest of a.fake")
		})

		t.Run("exports a name with an upper-case initial", func(t *testing.T) {
			t.Parallel()
			d := lsptest.Declaration()
			assert.Equal(t, d.Visibility("Store"), sema.Exported, "the visibility of Store")
			assert.Equal(t, d.Visibility("size"), sema.Unexported, "the visibility of size")
		})
	})

	t.Run("Workspace", func(t *testing.T) {
		t.Parallel()

		t.Run("creates the directories of a nested file", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, map[string]string{"one/two/a.fake": lsptest.Content})
			_, err := os.Stat(filepath.Join(root, "one", "two", "a.fake"))
			assert.NoError(t, err, "the test finds one/two/a.fake")
		})
	})
}
