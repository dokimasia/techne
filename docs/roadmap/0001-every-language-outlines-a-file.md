---
milestone: 0001
title: Every language outlines a file over MCP
status: Done
depends-on: 0000
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: RFC-0001, RFC-0002, RFC-0003, RFC-0005
---

# Milestone 0001: Every language outlines a file over MCP

## Goal

An agent can ask techne what a file declares, in any registered language,
and read from the answer how the system knows.

## Done when

- [x] `tools/list` over stdio returns the outline tool and the capability
      tool
- [x] Outlining a file in Go, Python, Java, Rust and TypeScript returns
      its declarations with fidelity `syntactic`
- [x] Outlining a file in a language nothing is registered for returns
      `unsupported` with a reason, rather than an empty list
- [x] The capability tool reports, per language and per role, what can be
      answered, at what fidelity, and whether the engine can run
- [x] An answer over the token budget drops documentation, then snippets,
      then items, and says how many matched against how many were
      returned
- [x] Every path in a request and an answer is relative to the workspace
      root; an absolute path in a request is refused
- [x] A search matching exactly one symbol returns that symbol's
      declaration, without a second call
- [x] A name matching several declarations returns the candidates ranked,
      each with enough to choose, rather than an error
- [x] Every tool description names the built-in tool it replaces and why
- [x] Removing one language — its directory, its `go.work` line, and its
      require, replace and registration in the root module — leaves the
      other four answering and touches no other module
- [x] A query file that fails to compile stops startup, rather than
      returning no results at run time
- [x] The conformance suite runs against all five language modules from
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
| 2026-09-01 | Reworded the language-removal criterion | Removing a language takes four edits in the root module, not two; the property being tested is that no other module is touched |
| 2026-09-01 | Closed | Every criterion met and checked against the running binary |
