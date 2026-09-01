// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/lang"
)

func TestDeclaration(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("declares nothing and is refused", func(t *testing.T) {
			t.Parallel()
			var unset lang.Declaration
			assert.Empty(t, string(unset.Language), "an unset declaration names no language")
			assert.Nil(t, unset.IsTest, "an unset declaration states no convention")
			assert.Nil(t, unset.Namespace, "an unset declaration states no convention")
			assert.Nil(t, unset.Visibility, "an unset declaration states no convention")
		})
	})

	t.Run("Blank", func(t *testing.T) {
		t.Parallel()

		t.Run("is empty where every identifier binds a name", func(t *testing.T) {
			t.Parallel()
			var unset lang.Declaration
			assert.False(t, unset.Blank["_"],
				"a language where _ is an ordinary name must not have its declarations dropped")
		})

		t.Run("names the identifiers that bind nothing", func(t *testing.T) {
			t.Parallel()
			d := lang.Declaration{Blank: map[string]bool{"_": true}}
			assert.True(t, d.Blank["_"],
				"a declaration whose name binds nothing is not a symbol a caller can act on")
		})
	})
}
