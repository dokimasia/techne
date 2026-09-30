// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/tool"
)

// resolved runs the resolve tool over [addressable] with input, and decodes the answer.
func resolved(t *testing.T, input string) tool.Answer {
	t.Helper()
	return resolvedOver(t, addressable(), input)
}

// resolvedOver runs the resolve tool over service with input, and decodes the answer.
func resolvedOver(t *testing.T, service *reads, input string) tool.Answer {
	t.Helper()
	built, err := tool.Resolve(service)
	assert.NoError(t, err, "the error of Resolve")
	result, err := built.Execute(t.Context(), json.RawMessage(input))
	assert.NoError(t, err, "the error of Execute")
	var out tool.Answer
	assert.NoError(t, json.Unmarshal(result.Payload, &out), "the decoding of the answer")
	return out
}

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("starts its description with PREFER OVER", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Resolve(addressable())
			assert.NoError(t, err, "the error of Resolve")
			assert.HasPrefix(t, built.Description(), "PREFER OVER ", "the description")
		})

		t.Run("returns the declarations that the engine binds at the position", func(t *testing.T) {
			t.Parallel()
			got := resolved(t, `{"scope":"a.fx","line":3,"column":10}`)
			assert.False(t, got.Failed(), "the failure of the answer")
			assert.NotEmpty(t, got.Items, "the declarations of the answer")
		})

		t.Run("returns an error for a line or a column of zero", func(t *testing.T) {
			t.Parallel()
			built, err := tool.Resolve(addressable())
			assert.NoError(t, err, "the error of Resolve")
			for at, want := range map[string]string{
				`{"scope":"a.fx","line":0,"column":1}`: `"line" of at least 1, not 0`,
				`{"scope":"a.fx","line":1,"column":0}`: `"column" of at least 1, not 0`,
			} {
				_, err = built.Execute(t.Context(), json.RawMessage(at))
				assert.HasError(t, err, "the error of Execute for "+at)
				assert.Contains(t, err.Error(), want, "the error of Execute for "+at)
			}
		})

		t.Run("refuses a path that leaves the workspace", func(t *testing.T) {
			t.Parallel()
			got := resolved(t, `{"scope":"../b.fx","line":1,"column":1}`)
			assert.Equal(t, got.Error.Code, "refused", "the code of the failure")
			assert.Equal(t, got.Error.Reason, `"../b.fx" leaves the workspace root`, "the reason of the failure")
		})

		t.Run("returns an unsupported failure for a scope that no engine serves", func(t *testing.T) {
			t.Parallel()
			got := resolved(t, `{"scope":"notes.md","line":1,"column":1}`)
			assert.Equal(t, got.Error.Code, "unsupported", "the code of the failure")
		})

		t.Run("supports no negative claim at the syntactic tier", func(t *testing.T) {
			t.Parallel()
			got := resolved(t, `{"scope":"a.fx","line":3,"column":10}`)
			assert.False(t, got.Provenance.SupportsNegativeClaim, "the negative claim of the answer")
		})

		t.Run("returns a parameter that the name denotes without include", func(t *testing.T) {
			t.Parallel()
			got := resolvedOver(t, serving(declared("ctx", sema.KindParameter, "", 2, 10)),
				`{"scope":"a.fx","line":3,"column":11}`)
			assert.Equal(t, names(got.Items), []string{"ctx"}, "the declarations of the answer")
		})

		t.Run("returns an import that the name denotes without include", func(t *testing.T) {
			t.Parallel()
			got := resolvedOver(t, serving(declared("clock", sema.KindImport, "", 2, 10)),
				`{"scope":"a.fx","line":3,"column":11}`)
			assert.Equal(t, names(got.Items), []string{"clock"}, "the declarations of the answer")
		})

		t.Run("returns the path of a declaration in another file", func(t *testing.T) {
			t.Parallel()
			elsewhere := declared("Store", sema.KindStruct, "", 2, 10)
			elsewhere.Span.Path = "b.fx"
			got := resolvedOver(t, serving(elsewhere), `{"scope":"a.fx","line":1,"column":1}`)
			assert.Equal(t, got.Items[0].Path, "b.fx", "the path of Store")
			assert.Contains(t, got.Render(), "b.fx:3  struct Store", "the render")
		})
	})
}
