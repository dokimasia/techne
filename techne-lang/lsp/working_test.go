// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

func TestWorking(t *testing.T) {
	t.Parallel()

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("waits for a server that reports a loading job", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Loading, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with a loading server")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
		})

		t.Run("returns a total answer after the server settles", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Loading, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with a loading server")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatIndexWarming), "the answer has a warming caveat")
		})

		t.Run("waits for a job whose token the server created before the job began", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Created, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with a server that creates the token of a job first")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
		})

		t.Run("returns a total answer while a check on disk runs", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.DiskStuck, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with a check on disk that never ends")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatIndexWarming), "the answer has a warming caveat")
		})

		t.Run("waits for a job that begins BurstGap after another job ends", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Burst, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with a server that loads in a burst of jobs")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
		})

		t.Run("waits for a loading job of a server that declares a diagnosis", func(t *testing.T) {
			t.Parallel()
			server := lsptest.Server(lsptest.Loading)
			server.Diagnosis = lsptest.DiagnosisPrefix
			got, err := lsptest.Engine(t, lsptest.Workspace(t, sample()), server).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with a loading server that declares a diagnosis")
			assert.Equal(t, edges(got.Items), []string{"Get", "After"}, "the declarations that use Store")
		})

		t.Run("returns a total answer while a diagnosis runs", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.DiagnosisStuck, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with a diagnosis that never ends")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
			assert.False(t, hasCaveat(got.Caveats, trust.CaveatIndexWarming), "the answer has a warming caveat")
		})

		t.Run("returns a partial answer from a server that never settles", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, lsptest.Stuck, sample()).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate with a stuck server")
			assert.Empty(t, got.Items, "the relations from a stuck server")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.True(t, hasCaveat(got.Caveats, trust.CaveatIndexWarming), "the answer has a warming caveat")
			assert.False(t, trust.SupportsNegativeClaim(trust.Resolved, got.Completeness),
				"SupportsNegativeClaim of the answer")
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a warm answer within 100 milliseconds", func(t *testing.T) {
			t.Parallel()
			e := serving(t, lsptest.Default, sample())
			_, err := e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			assert.NoError(t, err, "the first Resolve")

			began := time.Now()
			_, err = e.Resolve(t.Context(), engine.Request{Scope: "a.fake"}, store())
			took := time.Since(began)
			assert.NoError(t, err, "the second Resolve")
			assert.InRange(t, took, -1<<63, float64(100*time.Millisecond-1), "the second Resolve took "+took.String())
		})
	})
}

// edges returns the names of the far ends of relations, in order.
func edges(relations []sema.Relation) []string {
	out := make([]string, 0, len(relations))
	for _, one := range relations {
		out = append(out, one.To.Name)
	}
	return out
}
