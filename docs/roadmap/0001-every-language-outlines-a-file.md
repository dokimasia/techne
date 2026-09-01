---
milestone: 0001
title: Every language outlines a file over MCP
status: Planned
depends-on: 0000
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: RFC-0001
---

# Milestone 0001: Every language outlines a file over MCP

## Goal

An agent can ask techne what a file declares, in any registered language,
and read from the answer how the system knows.

## Done when

- [ ] `tools/list` over stdio returns the outline tool and the capability
      tool
- [ ] Outlining a file in Go, Python, Java, Rust and TypeScript returns
      its declarations with fidelity `syntactic`
- [ ] Outlining a file in a language nothing is registered for returns
      `unsupported` with a reason, rather than an empty list
- [ ] The capability tool reports, per language and per role, what can be
      answered and at what fidelity
- [ ] Deleting one language directory and its `go.work` line removes that
      language and leaves the other four answering
- [ ] A query file that fails to compile stops startup, rather than
      returning no results at run time
- [ ] The conformance suite runs against all five language modules from
      one test target

## Why now

Milestone 0000 declares the ports; this implements them, and it is the
first point at which anything outside the repository can use techne. All
five languages register together because the declaration has to hold for
five grammars with five different ideas of what a declaration, a test
file and a namespace are.

## Scope

The language declaration and the shared grammar engine, the conformance
suite, five language modules holding a declaration and query files, the
read dispatcher, the tool interface, and the MCP transport with the
composition root that registers everything.

## Not in this milestone

- Any tier above `syntactic`: milestone 0002
- Anything that changes a file: milestone 0003
- Serving answers from memory: milestone 0004

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| The vendored tag queries disagree on capture names across grammars | 0002, and everything after | Fix the captures in the language modules and keep the engine's contract fixed, rather than special-casing per grammar in the engine |
| cgo builds fail on a contributor platform | 0002, 0003 | Record the platform and the toolchain that works, before the language set grows further |
| Five grammars make the declaration too general to be useful | 0002 | Cut the language set to three and record which two were dropped and why |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-09-01 | Created | First milestone that produces something an agent can call |
