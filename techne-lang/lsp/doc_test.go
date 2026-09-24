// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/lsp/lsptest"
)

// satisfies reports whether e implements the port of role.
func satisfies(e any, role engine.Role) bool {
	switch role {
	case engine.RoleOutline:
		_, ok := e.(engine.Outliner)
		return ok
	case engine.RoleSearch:
		_, ok := e.(engine.Searcher)
		return ok
	case engine.RoleResolve:
		_, ok := e.(engine.Resolver)
		return ok
	case engine.RoleRelate:
		_, ok := e.(engine.Relator)
		return ok
	case engine.RolePlan:
		_, ok := e.(engine.Planner)
		return ok
	case engine.RoleFormat:
		_, ok := e.(engine.Formatter)
		return ok
	case engine.RoleCheck:
		_, ok := e.(engine.Checker)
		return ok
	case engine.RoleVerify:
		_, ok := e.(engine.Verifier)
		return ok
	case engine.RoleIndex:
		_, ok := e.(engine.Indexer)
		return ok
	}
	return false
}

func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("Engine", func(t *testing.T) {
		t.Parallel()

		t.Run("implements the ports of six roles", func(t *testing.T) {
			t.Parallel()
			var e any = serving(t, lsptest.Default, sample())
			served := map[engine.Role]bool{
				engine.RoleResolve: true, engine.RoleRelate: true, engine.RolePlan: true,
				engine.RoleFormat: true, engine.RoleCheck: true, engine.RoleVerify: true,
			}
			for _, role := range engine.Roles() {
				assert.Equal(t, satisfies(e, role), served[role], "the Engine implements the port of "+role.String())
			}
		})

		t.Run("starts one server for questions of three roles", func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "starts")
			e := lsptest.Engine(t, lsptest.Workspace(t, sample()),
				lsptest.Server(lsptest.Default, lsptest.RecordStarts(log)))
			request := engine.Request{Scope: "a.fake"}

			_, err := e.Resolve(t.Context(), request, store())
			assert.NoError(t, err, "Resolve")
			_, err = e.Relate(t.Context(), request, declared("Store", sema.KindStruct), sema.ReferencedBy)
			assert.NoError(t, err, "Relate")
			_, err = e.Verify(t.Context(), request, nil)
			assert.NoError(t, err, "Verify")
			assert.Equal(t, starts(t, log), 1, "the number of servers started")
		})

		t.Run("writes no file when it plans a change", func(t *testing.T) {
			t.Parallel()
			root := lsptest.Workspace(t, sample())
			e := lsptest.Engine(t, root, lsptest.Server(lsptest.Default))
			_, err := renameWith(t, e)
			assert.NoError(t, err, "Plan of a rename")

			content, err := os.ReadFile(filepath.Join(root, "a.fake"))
			assert.NoError(t, err, "the test reads a.fake")
			assert.Equal(t, string(content), lsptest.Content, "a.fake after the plan")
		})
	})
}
