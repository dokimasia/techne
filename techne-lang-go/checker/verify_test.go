// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

func TestVerify(t *testing.T) {
	t.Parallel()

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("reports what the compiler objected to", func(t *testing.T) {
			t.Parallel()
			held := whole()
			held["broken.go"] = broken
			got, err := serving(t, held).Verify(t.Context(), engine.Request{Scope: "."}, nil)

			assert.NoError(t, err, "a workspace that does not compile is an answer, not a fault")
			assert.NotEmpty(t, got.Items, "the type checker objected")
			assert.Equal(t, got.Items[0].Diagnostic.Severity, diag.SeverityError,
				"everything it reports stops the build")
			assert.Equal(t, string(got.Items[0].Diagnostic.Span.Path), "broken.go",
				"and names the file it is in")
		})

		t.Run("finds nothing wrong with a workspace that builds", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Verify(t.Context(), engine.Request{Scope: "."}, nil)

			assert.NoError(t, err, "verifying succeeds")
			assert.Empty(t, got.Items, "nothing is wrong with it")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "over the whole module")
		})

		t.Run("narrows to the scope the request named", func(t *testing.T) {
			t.Parallel()
			// A fault somewhere else is a true report of a question
			// nobody asked.
			held := whole()
			held["broken.go"] = broken
			got, err := serving(t, held).Verify(t.Context(),
				engine.Request{Scope: "store.go"}, nil)

			assert.NoError(t, err, "verifying one file succeeds")
			assert.Empty(t, got.Items, "and the fault is not in it")
		})

		t.Run("says it read nothing where the scope holds no Go", func(t *testing.T) {
			t.Parallel()
			held := whole()
			held["docs/notes.md"] = "# notes\n"
			got, err := serving(t, held).Verify(t.Context(), engine.Request{Scope: "docs"}, nil)

			assert.NoError(t, err, "a scope with nothing to read is not a fault")
			assert.True(t, got.Skipped, "and the engine says it read nothing")
		})

		t.Run("says a suite it was named was answered from the one analysis", func(t *testing.T) {
			t.Parallel()
			// A type checker has one analysis and no linters to choose
			// between. Answering silently would read as having run them.
			got, err := serving(t, whole()).Verify(t.Context(),
				engine.Request{Scope: "."}, []string{"vet"})

			assert.NoError(t, err, "naming a suite is not a fault")
			assert.True(t, carries(got.Caveats, trust.CaveatUnsupported), "and the caveat says so")
		})
	})
}

func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("judges content the workspace does not hold", func(t *testing.T) {
			t.Parallel()
			// A gate runs before anything is written, so what it judges
			// exists only in memory. The loader reads the named paths
			// from there and everything else from disk, which is the
			// workspace as the change would leave it.
			got, err := serving(t, whole()).Check(t.Context(),
				map[source.Path][]byte{"use.go": []byte(broken)})

			assert.NoError(t, err, "checking content that is not on disk succeeds")
			assert.NotEmpty(t, got.Items, "the type checker objected to what it was shown")
		})

		t.Run("finds nothing wrong with content that compiles", func(t *testing.T) {
			t.Parallel()
			got, err := serving(t, whole()).Check(t.Context(),
				map[source.Path][]byte{"use.go": []byte(use)})

			assert.NoError(t, err, "checking succeeds")
			assert.Empty(t, got.Items, "nothing is wrong with it")
		})

		t.Run("catches a change that breaks a file it does not name", func(t *testing.T) {
			t.Parallel()
			// The failure a gate exists for. A rename that leaves one
			// caller behind breaks the caller's file rather than the one
			// that was edited, so every package is judged and not only
			// what was handed over.
			renamed := "package p\n\ntype Vault struct{ size int }\n\nfunc helper() int { return 1 }\n"
			got, err := serving(t, whole()).Check(t.Context(),
				map[source.Path][]byte{"store.go": []byte(renamed)})

			assert.NoError(t, err, "checking succeeds")
			assert.NotEmpty(t, got.Items, "use.go no longer compiles, and use.go was not named")
			assert.Equal(t, string(got.Items[0].Diagnostic.Span.Path), "use.go",
				"which is where the fault is")
		})

		t.Run("leaves alone what this language does not claim", func(t *testing.T) {
			t.Parallel()
			// One change can touch several languages and each engine
			// judges its own.
			_, err := serving(t, whole()).Check(t.Context(),
				map[source.Path][]byte{"notes.md": []byte("# notes\n")})

			assert.ErrorIs(t, err, engine.ErrDecline, "so another engine gets a turn")
		})

		t.Run("keeps nothing it type-checked", func(t *testing.T) {
			t.Parallel()
			// The view a gate builds describes a workspace that does not
			// exist. Cached, the next question would answer about a
			// change nobody applied.
			e := serving(t, whole())
			_, err := e.Check(t.Context(),
				map[source.Path][]byte{"use.go": []byte(broken)})
			assert.NoError(t, err, "checking succeeds")

			got, err := e.Verify(t.Context(), engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "verifying the workspace succeeds")
			assert.Empty(t, got.Items, "and the workspace on disk still compiles")
		})
	})
}
