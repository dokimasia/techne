// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package wire_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/internal/wire"
)

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Names", func(t *testing.T) {
		t.Parallel()

		t.Run("round-trips every declared value through JSON", func(t *testing.T) {
			t.Parallel()
			names := wire.New(unknown, map[colour]string{unknown: "unknown", red: "red", green: "green"})
			decode := func(encoded []byte) (colour, error) {
				var decoded colour
				err := names.Unmarshal(encoded, &decoded)
				return decoded, err
			}
			for _, v := range []colour{unknown, red, green} {
				assert.RoundTrip(t, names.Marshal, decode, v, "round trip")
			}
		})
	})
}
