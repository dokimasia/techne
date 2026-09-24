// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package sema_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

func TestID(t *testing.T) {
	t.Parallel()

	status := sema.NewID(source.Language("go"), "./core/trust", "Status", sema.KindType)

	t.Run("NewID", func(t *testing.T) {
		t.Parallel()

		t.Run("formats the ID as language:unit#name:kind", func(t *testing.T) {
			t.Parallel()
			got := sema.NewID(source.Language("go"), "./internal/fsx", "Digest", sema.KindFunction)
			assert.Equal(t, got, sema.ID("go:./internal/fsx#Digest:function"), "ID")
		})

		tests := []struct {
			name string
			give sema.ID
		}{
			{
				name: "distinguishes two kinds of one name",
				give: sema.NewID(source.Language("go"), "./core/trust", "Status", sema.KindFunction),
			},
			{
				name: "distinguishes two units of one name",
				give: sema.NewID(source.Language("go"), "./core/gate", "Status", sema.KindType),
			},
			{
				name: "distinguishes two languages of one name",
				give: sema.NewID(source.Language("rust"), "./core/trust", "Status", sema.KindType),
			},
			{
				name: "distinguishes a renamed declaration",
				give: sema.NewID(source.Language("go"), "./core/trust", "State", sema.KindType),
			},
			{
				name: "distinguishes a member of the same name in a container",
				give: sema.NewID(source.Language("go"), "./core/trust", "Evidence.Status", sema.KindType),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.NotEqual(t, tt.give, status, "ID")
			})
		}
	})

	t.Run("Qualify", func(t *testing.T) {
		t.Parallel()

		t.Run("joins a container and a name with a dot", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.Qualify("Store", "Get"), "Store.Get", "Qualify")
		})

		t.Run("joins a qualified container and a name", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.Qualify("Store.Get", "key"), "Store.Get.key", "Qualify")
		})

		t.Run("returns a name without a container unchanged", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, sema.Qualify("", "Get"), "Get", "Qualify")
		})
	})

	t.Run("Base", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give sema.ID
			want string
		}{
			{
				name: "returns the name after the last dot",
				give: sema.NewID(source.Language("go"), ".", "Store.Get", sema.KindMethod),
				want: "Get",
			},
			{name: "returns a name without a dot whole", give: status, want: "Status"},
			{name: "returns an empty string for the zero ID", give: sema.ID("")},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, tt.give.Base(), tt.want, "Base")
			})
		}
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give sema.ID
			want string
		}{
			{name: "returns the qualified name", give: status, want: "Status"},
			{
				name: "keeps the qualifier of a qualified name",
				give: sema.NewID(source.Language("java"), ".", "Store.read", sema.KindMethod),
				want: "Store.read",
			},
			{name: "returns an empty string for a value without a name", give: sema.ID("not an identity")},
			{name: "returns an empty string for the zero ID", give: sema.ID("")},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, tt.give.Name(), tt.want, "Name")
			})
		}
	})
}
