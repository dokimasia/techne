// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

import "go.dokimi.dev/techne/core/internal/wire"

// Role is a question an engine can answer. Each role has its own port
// interface, and engines declare a fidelity and a cost per role. The zero
// value is RoleUnset.
type Role uint8

const (
	// RoleUnset is no role. It is never valid on a request.
	RoleUnset Role = iota
	// RoleOutline lists the declarations in a scope. See Outliner.
	RoleOutline
	// RoleSearch finds declarations by name. See Searcher.
	RoleSearch
	// RoleResolve finds the declaration a name denotes. See Resolver.
	RoleResolve
	// RoleRelate finds the edges of a declaration. See Relator.
	RoleRelate
	// RolePlan computes the edits of an operation. See Planner.
	RolePlan
	// RoleFormat computes formatting edits. See Formatter.
	RoleFormat
	// RoleCheck reports problems in content that is not on disk. See
	// Checker.
	RoleCheck
	// RoleVerify reports problems in a scope on disk. See Verifier.
	RoleVerify
	// RoleIndex produces the facts an index stores. See Indexer.
	RoleIndex
)

var roleWords = wire.New(RoleUnset, map[Role]string{
	RoleUnset:   "unset",
	RoleOutline: "outline",
	RoleSearch:  "search",
	RoleResolve: "resolve",
	RoleRelate:  "relate",
	RolePlan:    "plan",
	RoleFormat:  "format",
	RoleCheck:   "check",
	RoleVerify:  "verify",
	RoleIndex:   "index",
})

// String returns the wire string of r, or "unset" if r is not a declared
// Role.
func (r Role) String() string { return roleWords.String(r) }

// Roles returns every Role except RoleUnset, in declaration order.
func Roles() []Role {
	return []Role{
		RoleOutline, RoleSearch, RoleResolve, RoleRelate,
		RolePlan, RoleFormat, RoleCheck, RoleVerify, RoleIndex,
	}
}

// Cost is what producing an answer takes, cheapest first. It is independent
// of fidelity: the catalogue orders engines by fidelity and breaks ties by
// cost.
type Cost uint8

const (
	// CostMemory is a lookup in a structure already in memory.
	CostMemory Cost = iota
	// CostParse is a parse of each file in scope.
	CostParse
	// CostAnalyze is a type check of a package graph.
	CostAnalyze
	// CostSession is expensive once and cheap afterwards, such as a language
	// server that loads the workspace when it starts.
	CostSession
	// CostProcess is a subprocess per call.
	CostProcess
)

var costWords = wire.New(CostProcess, map[Cost]string{
	CostMemory:  "memory",
	CostParse:   "parse",
	CostAnalyze: "analyze",
	CostSession: "session",
	CostProcess: "process",
})

// String returns the wire string of c, or "process" if c is not a declared
// Cost, so an unknown cost reads as the most expensive.
func (c Cost) String() string { return costWords.String(c) }
