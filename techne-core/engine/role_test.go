// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

func TestRole(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("is the wire form of the role", func(t *testing.T) {
			t.Parallel()
			// A capability report names roles, so these strings reach a
			// caller and are pinned rather than derived.
			for role, want := range map[engine.Role]string{
				engine.RoleUnset:   "unset",
				engine.RoleOutline: "outline",
				engine.RoleSearch:  "search",
				engine.RoleResolve: "resolve",
				engine.RoleRelate:  "relate",
				engine.RolePlan:    "plan",
				engine.RoleFormat:  "format",
				engine.RoleCheck:   "check",
				engine.RoleVerify:  "verify",
				engine.RoleIndex:   "index",
			} {
				assert.Equal(t, role.String(), want,
					"a capability report names roles, so these strings reach a caller")
			}
		})

		t.Run("falls back to unset outside the set", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, engine.Role(200).String(), "unset",
				"a role outside the set reads as unset rather than as an empty name")
		})
	})

	t.Run("Roles", func(t *testing.T) {
		t.Parallel()

		t.Run("names a role every engine can be selected for", func(t *testing.T) {
			t.Parallel()
			// An engine that serves every port must be selectable for
			// every role. Selection is by type assertion in one switch,
			// so a role added without a case there can be asked for and
			// is served by nothing, whatever engine is registered.
			c := engine.NewCatalog()
			assert.NoError(t, c.Add(complete{}), "an engine serving every port registers")

			for _, role := range engine.Roles() {
				assert.Length(t, c.For(t.Context(), whole, role), 1,
					"a role nothing can be selected for is one a caller asks for and nothing serves")
			}
		})

		t.Run("excludes the unset role", func(t *testing.T) {
			t.Parallel()
			for _, r := range engine.Roles() {
				assert.NotEqual(t, r, engine.RoleUnset, "no port serves the unset role")
			}
		})

		t.Run("names each role once", func(t *testing.T) {
			t.Parallel()
			seen := map[engine.Role]bool{}
			for _, r := range engine.Roles() {
				assert.False(t, seen[r], "a role listed twice would be reported twice")
				seen[r] = true
			}
		})
	})

	t.Run("Cost", func(t *testing.T) {
		t.Parallel()

		t.Run("rises so a catalogue sorts cheapest first", func(t *testing.T) {
			t.Parallel()
			ordered := []engine.Cost{
				engine.CostMemory, engine.CostParse, engine.CostAnalyze,
				engine.CostSession, engine.CostProcess,
			}
			assert.Pairwise(t, ordered, func(earlier, later engine.Cost) bool {
				return earlier < later
			}, "the order rises, so a catalogue takes the cheapest of two equals first")
		})

		t.Run("is the wire form of the price", func(t *testing.T) {
			t.Parallel()
			for c, want := range map[engine.Cost]string{
				engine.CostMemory: "memory", engine.CostParse: "parse",
				engine.CostAnalyze: "analyze", engine.CostSession: "session",
				engine.CostProcess: "process",
			} {
				assert.Equal(t, c.String(), want, "a capability report carries this string to a caller")
			}
			assert.Equal(t, engine.Cost(200).String(), "process",
				"an unknown price reads as the dearest, so a catalogue does not prefer it by accident")
		})

		t.Run("prices a session below a subprocess", func(t *testing.T) {
			t.Parallel()
			// A language server is expensive once and cheap afterwards.
			// Sorted above a per-call subprocess, a caller would avoid
			// the fastest engine it has.
			assert.True(t, engine.CostSession < engine.CostProcess,
				"a server is expensive once and cheap after, so pricing it per call would hide the fastest engine")
		})
	})
}

// whole is the language the all-ports engine answers about.
const whole = source.Language("whole")

// complete serves every port, so a role no engine can be selected for
// is a gap in the selection switch rather than a gap in this engine.
type complete struct{}

func (complete) Name() string                        { return "complete" }
func (complete) Language() source.Language           { return whole }
func (complete) Fidelity(engine.Role) trust.Fidelity { return trust.Resolved }
func (complete) Cost(engine.Role) engine.Cost        { return engine.CostMemory }
func (complete) Granularity() engine.Invalidation    { return engine.InvalidateFile }
func (complete) Affected(source.Path) []source.Path  { return nil }

func (complete) Outline(context.Context, engine.Request) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{}, nil
}

func (complete) Search(context.Context, engine.Request, engine.Query) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{}, nil
}

func (complete) Resolve(context.Context, engine.Request, source.Position) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{}, nil
}

func (complete) Relate(
	context.Context,
	engine.Request,
	sema.ID,
	sema.RelationKind,
) (engine.Result[sema.Relation], error) {
	return engine.Result[sema.Relation]{}, nil
}

func (complete) Plan(
	context.Context,
	engine.Request,
	edit.Operation,
	edit.Target,
	edit.Args,
) (engine.Result[edit.Change], error) {
	return engine.Result[edit.Change]{}, nil
}

func (complete) Format(context.Context, []source.Path) (engine.Result[edit.Change], error) {
	return engine.Result[edit.Change]{}, nil
}

func (complete) Check(context.Context, map[source.Path][]byte) (engine.Result[edit.Finding], error) {
	return engine.Result[edit.Finding]{}, nil
}

func (complete) Verify(context.Context, engine.Request, []string) (engine.Result[edit.Finding], error) {
	return engine.Result[edit.Finding]{}, nil
}

func (complete) Index(context.Context, source.Path) (engine.Result[sema.Symbol], error) {
	return engine.Result[sema.Symbol]{}, nil
}
