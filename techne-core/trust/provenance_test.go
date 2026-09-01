// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust_test

import (
	"testing"

	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

func TestProvenance(t *testing.T) {
	t.Parallel()

	t.Run("SupportsNegativeClaim", func(t *testing.T) {
		t.Parallel()

		t.Run("agrees with the package function", func(t *testing.T) {
			t.Parallel()
			// Two implementations of one rule drift. The method must
			// delegate rather than repeat the comparison.
			for _, f := range []trust.Fidelity{trust.None, trust.Syntactic, trust.Indexed, trust.Resolved} {
				for _, c := range []trust.Completeness{trust.ScopeUnknown, trust.ScopePartial, trust.ScopeTotal} {
					p := trust.Provenance{Fidelity: f, Completeness: c}
					if got, want := p.SupportsNegativeClaim(), trust.SupportsNegativeClaim(f, c); got != want {
						t.Errorf("Provenance{%d, %d}.SupportsNegativeClaim() = %v, want %v", f, c, got, want)
					}
				}
			}
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		t.Run("licenses nothing", func(t *testing.T) {
			t.Parallel()
			// A service that forgot to stamp provenance must not produce
			// an answer whose emptiness reads as proof of absence.
			var unset trust.Provenance
			if unset.SupportsNegativeClaim() {
				t.Error("an unstamped provenance must not license a negative claim")
			}
			if unset.Engine != "" {
				t.Errorf("zero Provenance.Engine = %q, want the empty string", unset.Engine)
			}
		})
	})

	t.Run("Caveat", func(t *testing.T) {
		t.Parallel()

		t.Run("names the files it is about", func(t *testing.T) {
			t.Parallel()
			// A caller told only that something drifted discards the
			// whole answer; one told which files moved keeps the rest.
			stale := trust.Caveat{
				Code:  trust.CaveatStale,
				Paths: []source.Path{"core/trust/status.go"},
			}
			if len(stale.Paths) == 0 {
				t.Error("a stale caveat that names no file cannot be acted on")
			}
		})
	})
}
