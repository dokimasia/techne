---
milestone: 0003
title: A change is all or nothing
status: Planned
depends-on: 0002
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: RFC-0004, RFC-0005
---

# Milestone 0003: A change is all or nothing

## Goal

An agent can rename a symbol in any language that meets the operation's
minimum fidelity, and know that either every reference moved and the
workspace still builds, or no file changed.

## Done when

- [ ] Renaming a symbol updates every reference across the workspace, in
      every language that meets the operation's declared minimum
- [ ] A language below that minimum is refused with a reason, rather than
      attempted at a weaker tier
- [ ] An operation that rewrites references is refused unless the plan's
      evidence supports a negative claim, so a language server that is
      still indexing cannot rename
- [ ] A rename that leaves the workspace not building leaves every file
      byte-identical to what it was before the call
- [ ] A plan computed against a file that has changed since is refused on
      the content digest, before anything is written
- [ ] Two changes touching the same files from different callers do not
      interleave, and neither leaves a partial write
- [ ] A dry run reports the same edits the real call applies, and runs the
      gate against an overlay, so a passing dry run means applying for
      real compiles
- [ ] A failed gate carries a ready plan for each diagnostic that has one
      obvious fix
- [ ] The gate records which verifier answered, in-process or subprocess
- [ ] The suite passes under `go test -race`

## Why now

The write path is only safe on evidence milestone 0002 produces. Building
it while languages sit on both sides of the fidelity minimum is what
tests the refusal, which is the whole argument for one operation name
across every language.

## Scope

One rename planner per language that can meet the minimum, the refusal
for the languages that cannot, the build gate, and the batch.

The pipeline itself was built ahead of this milestone to ship
`document.symbol`, which needs no type checker: the edit plan, the
operation specs, the policy check, content preconditions, per-path
locking, the projection, the parse gate, the atomic apply and the
rollback all exist and are driven by a shipped tool. What is untested is
everything the parser cannot reach, which is every claim about references
this milestone is about.

## Not in this milestone

- The other operations in the catalogue: nobody has scheduled them yet
- Applying edits a language server proposed: nobody has scheduled it yet
- Batching several operations behind one gate: nobody has scheduled it yet

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| Rollback leaves the workspace in a state neither before nor after | 0004 | Hold the file locks across apply, gate and rollback, and test the crash path before adding a second operation |
| The build gate is too slow to run inside an agent's turn | 0004 | Gate on the smallest package set covering the edit, rather than the whole workspace |
| Formatting after apply rewrites files the plan never named | 0004 | Format only the paths the plan lists, and fail the gate if any other file changed |
| Only one language clears the fidelity minimum | nothing | Ship it, and record which four refuse and on what evidence |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-09-02 | Narrowed the scope to the rename | The pipeline landed early behind `document.symbol`, the one operation a parser can serve, so what is left here is what needs a type checker |
| 2026-09-01 | Renumbered from 0004 and widened past Go | Nothing was committed, and the write path covers every language that meets the minimum |
| 2026-09-01 | Created | First milestone that changes a file |
