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
- [x] Outlining a file in C, C#, Go, Java, JavaScript, Python, Ruby,
      Rust, Scala and TypeScript returns its declarations with fidelity
      `syntactic`
- [x] A declaration carries the documentation, the annotations and the
      keywords written on it, in the forms that language writes them
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
      other nine answering and touches no other module
- [x] A query file that fails to compile stops startup, rather than
      returning no results at run time
- [x] The conformance suite runs against all ten language modules from
      one test target, comparing each fixture's outline as a whole set

## Why now

Milestone 0000 declares the ports; this implements them, and it is the
first point at which anything outside the repository can use techne. All
ten languages register together because the declaration has to hold for
ten grammars with ten different ideas of what a declaration, a test file,
a namespace and a documentation comment are.

## Scope

The language declaration and the shared grammar engine, the conformance
suite, ten language modules holding a declaration and query files, the
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
| Ten grammars make the declaration too general to be useful | 0002 | Cut the language set and record which languages were dropped and why |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-09-01 | Created | First milestone that produces something an agent can call |
| 2026-09-01 | Reworded the language-removal criterion | Removing a language takes four edits in the root module, not two; the property being tested is that no other module is touched |
| 2026-09-01 | Left open | The tags queries capture a fraction of each language: Go finds no constant and no interface, Python no method, and eight of the fourteen kinds are produced by nothing. The conformance fixtures are ~9 lines each and check found symbols against a subset, so none of this failed a test |
| 2026-09-01 | Widened the queries, the vocabulary and the language set | Each query is now upstream's extended rather than upstream's alone; the vocabulary carries 23 kinds rather than 14, plus the annotations and keywords written on a declaration; and the set grew from five languages to ten. The fixtures compare as whole sets, so a declaration form nothing captures fails a test rather than passing one |
| 2026-09-01 | Closed | Every criterion is met and the suite passes for all ten modules |
