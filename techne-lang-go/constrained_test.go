// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	golang "go.dokimi.dev/techne/lang/go"
	"go.dokimi.dev/techne/lang/go/checker"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// TestMain runs the scripted language server of lsptest when the test binary is started as
// one, and the tests otherwise.
func TestMain(m *testing.M) { lsptest.Main(m) }

// The files of [constrainedFiles] that the default build excludes: excludedFile, which no build
// includes without the tag never, and portedFile, a file of plan9, which every build of plan9
// includes.
const (
	excludedFile = "never.go"
	portedFile   = "p_plan9.go"
)

// constrainedFiles returns a module whose root package declares P, with the files excludedFile,
// which names Store, and portedFile, which uses P alone, and the file a.fake that the scripted
// server describes.
func constrainedFiles() map[string]string {
	return map[string]string{
		"go.mod":     "module example.com/p\n\ngo 1.24\n",
		"p.go":       "package p\n\nfunc P() int { return 1 }\n",
		excludedFile: "//go:build never\n\npackage p\n\nvar Store = P\n",
		portedFile:   "package p\n\nvar _ = P\n",
		"a.fake":     lsptest.Content,
	}
}

func TestConstrained(t *testing.T) {
	t.Parallel()

	// store identifies the struct Store of a.fake, which the scripted server declares.
	store := sema.NewID(lsptest.Language, ".", "Store", sema.KindStruct)

	t.Run("Relate", func(t *testing.T) {
		t.Parallel()

		t.Run("adds the excluded files that name the declaration to the relations of the server",
			func(t *testing.T) {
				t.Parallel()
				got, err := constrainedOver(t, lsptest.Default).(engine.Relator).Relate(t.Context(),
					engine.Request{Scope: "a.fake"}, store, sema.ReferencedBy)
				assert.NoError(t, err, "Relate of the uses of Store")
				assert.NotEmpty(t, got.Items, "the uses of Store")
				assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
				assert.Equal(t, excludedIn(got.Caveats, trust.CaveatInactiveBuild), []source.Path{excludedFile},
					"the files of the caveat")
			})

		t.Run("returns the relations of the server as they are when no excluded file names the declaration",
			func(t *testing.T) {
				t.Parallel()
				after := sema.NewID(lsptest.Language, ".", "After", sema.KindFunction)
				got, err := constrainedOver(t, lsptest.Default).(engine.Relator).Relate(t.Context(),
					engine.Request{Scope: "a.fake"}, after, sema.ReferencedBy)
				assert.NoError(t, err, "Relate of the uses of After")
				assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
				assert.Empty(t, excludedIn(got.Caveats, trust.CaveatInactiveBuild), "the files of the caveat")
			})

		t.Run("returns the relations of the server as they are when the files cannot be listed",
			func(t *testing.T) {
				t.Parallel()
				got, err := unlistedOver(t, lsptest.Default).(engine.Relator).Relate(t.Context(),
					engine.Request{Scope: "a.fake"}, store, sema.ReferencedBy)
				assert.NoError(t, err, "Relate of the uses of Store")
				assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the answer")
				assert.Empty(t, excludedIn(got.Caveats, trust.CaveatInactiveBuild), "the files of the caveat")
			})

		t.Run("returns a skipped result of the server without a caveat", func(t *testing.T) {
			t.Parallel()
			got, err := constrainedOver(t, lsptest.Default).(engine.Relator).Relate(t.Context(),
				engine.Request{Scope: "p.go"}, store, sema.ReferencedBy)
			assert.NoError(t, err, "Relate over p.go")
			assert.True(t, got.Skipped, "the answer is skipped")
			assert.Empty(t, got.Caveats, "the caveats of the answer")
		})

		t.Run("returns ErrDecline of the server", func(t *testing.T) {
			t.Parallel()
			missing := sema.NewID(lsptest.Language, ".", "Missing", sema.KindStruct)
			_, err := constrainedOver(t, lsptest.Default).(engine.Relator).Relate(t.Context(),
				engine.Request{Scope: "a.fake"}, missing, sema.ReferencedBy)
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of the uses of a declaration that a.fake lacks")
		})
	})

	t.Run("Plan", func(t *testing.T) {
		t.Parallel()

		t.Run("makes the rename of a declaration that an excluded file names partial", func(t *testing.T) {
			t.Parallel()
			got, err := constrainedOver(t, lsptest.Default).(engine.Planner).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: store}, edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err, "Plan of the rename of Store")
			assert.NotEmpty(t, got.Items, "the changes of the rename")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the plan")
			assert.Equal(t, excludedIn(got.Caveats, trust.CaveatUnrewritten), []source.Path{excludedFile},
				"the files of the caveat")
		})

		t.Run("returns the plan of a move as the server returns it", func(t *testing.T) {
			t.Parallel()
			got, err := constrainedOver(t, lsptest.Default).(engine.Planner).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.MoveFile,
				edit.Target{Kind: edit.TargetFile, Symbol: store, Path: "a.fake"},
				edit.Args{edit.ArgDestination: "b.fake"})
			assert.NoError(t, err, "Plan of the move of a.fake")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the plan")
			assert.Empty(t, excludedIn(got.Caveats, trust.CaveatUnrewritten), "the files of the caveat")
		})

		t.Run("returns the plan of the server as it is when the files cannot be listed", func(t *testing.T) {
			t.Parallel()
			got, err := unlistedOver(t, lsptest.Default).(engine.Planner).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: store}, edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err, "Plan of the rename of Store")
			assert.Empty(t, excludedIn(got.Caveats, trust.CaveatUnrewritten), "the files of the caveat")
		})

		t.Run("returns a skipped plan of the server without a caveat", func(t *testing.T) {
			t.Parallel()
			got, err := constrainedOver(t, lsptest.Default).(engine.Planner).Plan(t.Context(),
				engine.Request{Scope: "p.go"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: store}, edit.Args{edit.ArgNewName: "Vault"})
			assert.NoError(t, err, "Plan of a rename over p.go")
			assert.True(t, got.Skipped, "the plan is skipped")
			assert.Empty(t, got.Caveats, "the caveats of the plan")
		})

		t.Run("returns ErrRefuse of the server", func(t *testing.T) {
			t.Parallel()
			_, err := constrainedOver(t, lsptest.Default).(engine.Planner).Plan(t.Context(),
				engine.Request{Scope: "a.fake"}, edit.RenameSymbol,
				edit.Target{Kind: edit.TargetSymbol, Symbol: store}, edit.Args{})
			assert.ErrorIs(t, err, engine.ErrRefuse, "the error of a rename without a new name")
		})
	})

	t.Run("Verify", func(t *testing.T) {
		t.Parallel()

		t.Run("adds the excluded files of the scope that no build of another port includes", func(t *testing.T) {
			t.Parallel()
			got, err := constrainedOver(t, lsptest.Default).(engine.Verifier).Verify(t.Context(),
				engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of the root")
			assert.Equal(t, got.Completeness, trust.ScopePartial, "the completeness of the answer")
			assert.Equal(t, excludedIn(got.Caveats, trust.CaveatInactiveBuild), []source.Path{excludedFile},
				"the files of the caveat")
		})

		t.Run("returns the verify of the server as it is when the files cannot be listed", func(t *testing.T) {
			t.Parallel()
			got, err := unlistedOver(t, lsptest.Default).(engine.Verifier).Verify(t.Context(),
				engine.Request{Scope: "."}, nil)
			assert.NoError(t, err, "Verify of the root")
			assert.Empty(t, excludedIn(got.Caveats, trust.CaveatInactiveBuild), "the files of the caveat")
		})

		t.Run("returns a skipped verify of the server without a caveat", func(t *testing.T) {
			t.Parallel()
			got, err := constrainedOver(t, lsptest.Default).(engine.Verifier).Verify(t.Context(),
				engine.Request{Scope: excludedFile}, nil)
			assert.NoError(t, err, "Verify of "+excludedFile)
			assert.True(t, got.Skipped, "the answer is skipped")
			assert.Empty(t, got.Caveats, "the caveats of the answer")
		})

		t.Run("returns ErrDecline of the server", func(t *testing.T) {
			t.Parallel()
			_, err := constrainedOver(t, lsptest.Default).(engine.Verifier).Verify(t.Context(),
				engine.Request{Scope: "a.fake"}, []string{"unit"})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of a verify that names a suite")
		})
	})

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		t.Run("adds the excluded files that can use a changed package to the check of the server", func(t *testing.T) {
			t.Parallel()
			got, err := constrainedOver(t, lsptest.Compiles).(engine.Checker).Check(t.Context(),
				map[source.Path][]byte{"a.fake": []byte(lsptest.Content)})
			assert.NoError(t, err, "Check of a.fake")
			assert.Equal(t, got.Completeness, trust.ScopeTotal, "the completeness of the check")
			assert.Equal(t, excludedIn(got.Caveats, trust.CaveatDependents),
				[]source.Path{excludedFile, portedFile}, "the files of the caveat")
		})

		t.Run("returns the check of the server as it is when the files cannot be listed", func(t *testing.T) {
			t.Parallel()
			got, err := unlistedOver(t, lsptest.Compiles).(engine.Checker).Check(t.Context(),
				map[source.Path][]byte{"a.fake": []byte(lsptest.Content)})
			assert.NoError(t, err, "Check of a.fake")
			assert.Empty(t, excludedIn(got.Caveats, trust.CaveatDependents), "the files of the caveat")
		})

		t.Run("returns ErrDecline of the server", func(t *testing.T) {
			t.Parallel()
			_, err := constrainedOver(t, lsptest.Compiles).(engine.Checker).Check(t.Context(),
				map[source.Path][]byte{"p.go": []byte("package p\n")})
			assert.ErrorIs(t, err, engine.ErrDecline, "the error of a check without a file of the server")
		})
	})
}

// constrainedOver returns the engine of [golang.Constrained] over [constrainedFiles]: the
// scripted server in mode, which serves the fake language, and the type checker of the module.
func constrainedOver(t *testing.T, mode lsptest.Mode) engine.Engine {
	t.Helper()
	root := lsptest.Workspace(t, constrainedFiles())
	files, err := checker.New(root, golang.Declaration())
	assert.NoError(t, err, "checker.New over "+root)
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer stop()
		assert.NoError(t, files.Close(ctx), "Close of the type checker")
	})
	return golang.Constrained(lsptest.Engine(t, root, lsptest.Server(mode)), files)
}

// unlistedOver returns the engine of [golang.Constrained] over [constrainedFiles] with a type
// checker over a directory that no longer exists, so the type checker cannot list the files
// that the build constraints exclude.
func unlistedOver(t *testing.T, mode lsptest.Mode) engine.Engine {
	t.Helper()
	gone := t.TempDir()
	files, err := checker.New(gone, golang.Declaration())
	assert.NoError(t, err, "checker.New over "+gone)
	assert.NoError(t, os.Remove(gone), "Remove of "+gone)
	root := lsptest.Workspace(t, constrainedFiles())
	return golang.Constrained(lsptest.Engine(t, root, lsptest.Server(mode)), files)
}

// excludedIn returns the paths of the caveats of code in caveats, in order.
func excludedIn(caveats []trust.Caveat, code trust.CaveatCode) []source.Path {
	var out []source.Path
	for _, one := range caveats {
		if one.Code == code {
			out = append(out, one.Paths...)
		}
	}
	return out
}
