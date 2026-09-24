// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package wire_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/internal/wire"
)

type colour uint8

const (
	unknown colour = iota
	red
	green
)

func colours() wire.Names[colour] {
	return wire.New(unknown, map[colour]string{unknown: "unknown", red: "red", green: "green"})
}

func TestWire(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("panics when two values share a string", func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, func() {
				wire.New(unknown, map[colour]string{unknown: "unknown", red: "red", green: "red"})
			}, "duplicate string")
		})

		t.Run("panics when the fallback has no string", func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, func() {
				wire.New(unknown, map[colour]string{red: "red"})
			}, "fallback without a string")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the string of a declared value", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, colours().String(green), "green", "declared value")
		})

		t.Run("returns the fallback string for an undeclared value", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, colours().String(colour(200)), "unknown", "undeclared value")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the value of a known string", func(t *testing.T) {
			t.Parallel()
			got, ok := colours().Parse("red")
			assert.True(t, ok, "known string")
			assert.Equal(t, got, red, "parsed value")
		})

		t.Run("returns the fallback for an unknown string", func(t *testing.T) {
			t.Parallel()
			got, ok := colours().Parse("blue")
			assert.False(t, ok, "unknown string")
			assert.Equal(t, got, unknown, "parsed value")
		})
	})

	t.Run("Marshal", func(t *testing.T) {
		t.Parallel()

		t.Run("encodes the wire string as JSON", func(t *testing.T) {
			t.Parallel()
			got, err := colours().Marshal(red)
			assert.NoError(t, err, "marshal")
			assert.Equal(t, string(got), `"red"`, "encoding")
		})
	})

	t.Run("Unmarshal", func(t *testing.T) {
		t.Parallel()

		t.Run("decodes a known string", func(t *testing.T) {
			t.Parallel()
			var got colour
			assert.NoError(t, colours().Unmarshal([]byte(`"green"`), &got), "unmarshal")
			assert.Equal(t, got, green, "decoded value")
		})

		t.Run("decodes an unknown string to the fallback", func(t *testing.T) {
			t.Parallel()
			got := red
			assert.NoError(t, colours().Unmarshal([]byte(`"blue"`), &got), "unmarshal")
			assert.Equal(t, got, unknown, "decoded value")
		})

		t.Run("rejects a JSON number without changing the target", func(t *testing.T) {
			t.Parallel()
			got := red
			assert.HasError(t, colours().Unmarshal([]byte(`7`), &got), "unmarshal of a number")
			assert.Equal(t, got, red, "target after a failed unmarshal")
		})
	})
}
