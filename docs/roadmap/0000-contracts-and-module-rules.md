---
milestone: 0000
title: Contracts and module rules
status: Planned
depends-on: none
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: RFC-0001, RFC-0002, RFC-0004
---

# Milestone 0000: Contracts and module rules

## Goal

A contributor can write an engine against the ports, and the build tells
them when an import breaks a module boundary.

## Done when

- [ ] The five modules build, and `make check` passes in each
- [ ] An import that breaks a module dependency rule fails `make lint`,
      naming the rule it broke
- [ ] `core` builds with `CGO_ENABLED=0`
- [ ] `trust.Fidelity` orders none below syntactic below indexed below
      resolved
- [ ] `trust.SupportsNegativeClaim` returns true for resolved binding
      together with total completeness, and false for every other pair
- [ ] `trust.Status` tells apart an answer nothing could serve, an answer
      served below the fidelity asked for, and an answer that found
      nothing
- [ ] A type can satisfy one engine port without implementing any other
- [ ] Every operation declares the target kinds it accepts, the
      parameters it needs, and the weakest fidelity it can be correct on

## Why now

Nothing comes before it. The four milestones after it all name these
types, and the ordering of `Fidelity` is what the catalogue sorts by and
what the write path checks against. Enforcing the module rules now means
the first engine is written inside a boundary that already holds.

## Scope

The vocabulary packages, the engine ports and the catalogue, the five
`go.mod` files and `go.work`, and the depguard rules that enforce the
import direction.

## Not in this milestone

- Any engine that produces facts: milestone 0001
- The tool set and the MCP transport: milestone 0001
- The write pipeline: milestone 0003
- Serving answers from memory: milestone 0004

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The port set is wrong in a way only an implementation reveals | 0001, and everything after it | Change the ports during 0001, while one engine implements them, rather than after five do |
| The vocabulary split proves too fine and packages import each other in a cycle | 0001 | Merge the two packages that cycle and record why the split did not hold |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-09-01 | Created | First milestone on the roadmap |
