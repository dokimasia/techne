// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engine

// Role is one question an engine may answer. It names the port rather
// than the tool: several tools may rest on one role, and an engine
// declares a fidelity and a cost per role.
type Role uint8

const (
	// RoleUnset means no role was named. It is never valid on a request.
	RoleUnset Role = iota
	RoleOutline
	RoleSearch
	RoleResolve
	RoleRelate
	RolePlan
	RoleFormat
	RoleVerify
	RoleIndex
)

// roleNames is the single definition point for the wire form of each
// role. A capability report names roles, so these strings reach a
// caller.
var roleNames = map[Role]string{
	RoleUnset:   "unset",
	RoleOutline: "outline",
	RoleSearch:  "search",
	RoleResolve: "resolve",
	RoleRelate:  "relate",
	RolePlan:    "plan",
	RoleFormat:  "format",
	RoleVerify:  "verify",
	RoleIndex:   "index",
}

// String returns the wire form of the role, or the form of [RoleUnset]
// for a value outside the declared set.
func (r Role) String() string {
	if name, ok := roleNames[r]; ok {
		return name
	}
	return roleNames[RoleUnset]
}

// Roles returns every role a port exists for, excluding [RoleUnset].
func Roles() []Role {
	return []Role{
		RoleOutline, RoleSearch, RoleResolve, RoleRelate,
		RolePlan, RoleFormat, RoleVerify, RoleIndex,
	}
}

// costNames is the single definition point for the wire form of each
// price. A capability report names costs, so these strings reach a
// caller.
var costNames = map[Cost]string{
	CostMemory:  "memory",
	CostParse:   "parse",
	CostAnalyze: "analyze",
	CostSession: "session",
	CostProcess: "process",
}

// String returns the wire form of the price, or that of [CostProcess]
// for a value outside the set: an unknown price is treated as the
// dearest rather than the cheapest.
func (c Cost) String() string {
	if name, ok := costNames[c]; ok {
		return name
	}
	return costNames[CostProcess]
}

// Cost is what producing one answer takes.
//
// It is independent of [trust.Fidelity]: the same facts can be had at
// very different prices, so a catalogue orders by evidence first and by
// cost among equals. The order rises, so cheaper sorts first.
type Cost uint8

const (
	// CostMemory is a lookup in a structure already built.
	CostMemory Cost = iota
	// CostParse is parsing one file.
	CostParse
	// CostAnalyze is type-checking a graph of units.
	CostAnalyze
	// CostSession is expensive once and cheap afterwards, such as a
	// language server that indexes a workspace on startup. Pricing it at
	// its first call would make a caller avoid the fastest engine it
	// has.
	CostSession
	// CostProcess is a subprocess for every call.
	CostProcess
)
