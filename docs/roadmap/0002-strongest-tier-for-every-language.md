---
milestone: 0002
title: Every language answers at the strongest tier it can reach
status: Planned
depends-on: 0001
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: RFC-0002
---

# Milestone 0002: Every language answers at the strongest tier it can reach

## Goal

An agent gets the best evidence available for any registered language,
and can tell from the answer which tier produced it.

## Done when

- [ ] With its usual server on `PATH`, each of the five languages answers
      resolve and relations above `syntactic`
- [ ] With the server absent, the same request returns `syntactic` and
      names the server it could not run
- [ ] Go answers above `syntactic` with no server running
- [ ] An empty result is marked as supporting a negative claim only when
      a type checker bound the name and the engine reports it covered the
      whole scope
- [ ] A server still building its index reports partial completeness, so
      its empty results are never marked as authoritative
- [ ] With the workspace not compiling, requests return a lower tier and
      a caveat saying the build is broken
- [ ] Every answer above `syntactic` carries the caveat naming what no
      static analysis sees: reflection, string-keyed dispatch, struct
      tags, code behind inactive build flags
- [ ] Positions round-trip on a line holding a non-ASCII character
- [ ] Both server edit shapes are read, including the file operations the
      older one omits
- [ ] A server command that asks to write to the workspace has its edits
      captured as a proposal, not applied

## Why now

Milestone 0001 gives every language one tier. This gives each the
strongest tier it can reach. Both engines arrive together because the
catalogue orders them against each other, and neither the ordering nor
the fallback can be tested while only one tier exists.

## Scope

The shared language-server engine and per-language server declarations.
The in-process type-checking engine for Go, with workspace and module
loading. The caveats each engine declares about what it cannot see.

## Not in this milestone

- Anything that changes a file: milestone 0003
- Serving answers from memory: milestone 0004
- Keeping a server warm across calls: nobody has scheduled it yet
- An in-process engine for any language other than Go: nobody has
  scheduled it yet

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| Servers disagree enough that one engine cannot serve them uniformly | 0003 | Keep the engine to the subset every server implements, and record what each one refuses |
| No server is installable in CI, so the tier is untested there | 0003, 0004 | Install one server in CI and mark the rest as covered locally only |
| Server startup cost makes the tier too slow to prefer | 0004 | Report it as session cost so the catalogue orders around it, rather than hiding it |
| Loading a large Go workspace is too slow to answer inside an agent's turn | 0003, 0004 | Report it as analyse cost, then measure before optimising |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-09-01 | Merged the language-server tier and the Go engine into one milestone | Splitting them made Go a milestone of its own, where every language moves together |
| 2026-09-01 | Created | Second read milestone |
