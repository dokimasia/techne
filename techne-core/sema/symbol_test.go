// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
)

func TestSymbol(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("sits at the top level of no unit", func(t *testing.T) {
			t.Parallel()
			var unset sema.Symbol
			assert.Empty(t, string(unset.Parent), "an unset symbol is not nested inside anything")
			assert.Equal(t, unset.Kind, sema.KindUnknown, "an unset symbol claims no kind")
			assert.False(t, unset.Exported, "an unset symbol claims no visibility")
		})
	})

	t.Run("Doc", func(t *testing.T) {
		t.Parallel()

		t.Run("is not needed to identify the declaration", func(t *testing.T) {
			t.Parallel()
			thinned := sema.Symbol{ID: "go:./a#F:function", Name: "F", Kind: sema.KindFunction}
			assert.NotEmpty(t, string(thinned.ID),
				"the output budget drops Doc first, so identity survives without it")
			assert.NotEmpty(t, thinned.Name,
				"the output budget drops Doc first, so identity survives without it")
			assert.NotEqual(t, thinned.Kind, sema.KindUnknown,
				"the output budget drops Doc first, so identity survives without it")
		})
	})
}
