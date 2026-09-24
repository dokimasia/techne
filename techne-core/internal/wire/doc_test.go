// Copyright ThesmOS B.V. 2026
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
			for _, v := range []colour{unknown, red, green} {
				encoded, err := names.Marshal(v)
				assert.NoError(t, err, "marshal")
				var decoded colour
				assert.NoError(t, names.Unmarshal(encoded, &decoded), "unmarshal")
				assert.Equal(t, decoded, v, "round trip")
			}
		})
	})
}
