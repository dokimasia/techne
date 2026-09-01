---
milestone: 0004
title: Answers survive a changing workspace
status: Planned
depends-on: 0003
ships-in: unscheduled
deadline: none
deadline-source: none
prd: none
rfc: RFC-0001
---

# Milestone 0004: Answers survive a changing workspace

## Goal

An agent gets answers from memory, and is told which files have changed
since those answers were indexed.

## Done when

- [ ] A boot scan indexes the workspace, and repeat reads are served from
      memory rather than reparsed
- [ ] A file edited outside techne is marked stale on the first event and
      re-indexed after the settle window
- [ ] An answer served while any file is stale returns `partial` and
      names which files drifted
- [ ] A change techne applied itself updates the index without waiting
      for a filesystem event
- [ ] Switching branches leaves the index consistent with what is on disk
- [ ] The index reports the fidelity of the engine that filled it, and
      never claims more
- [ ] Reads during a re-index pass `go test -race`

## Why now

The write path invalidates the index, and a change techne applied itself
is already known. Every read before this reparses, which is correct and
slow. Caching before the answers are right would only cache wrong answers
faster.

## Scope

The in-memory store and change source behind their ports, the boot scan,
the settle window, staleness naming, and the invalidation the write path
reports directly.

## Not in this milestone

- An index that survives a restart: nobody has scheduled it yet
- Indexing above the parser's fidelity: nobody has scheduled it yet
- Reading unsaved editor buffers: nobody has scheduled it yet

## Risks to the sequence

| Risk | What it delays | What we would do |
|---|---|---|
| A branch switch arrives as hundreds of unordered events and the index falls behind | nothing | Re-scan when the event count in one window passes a threshold, rather than processing each event |
| Staleness reporting is noisy enough that callers ignore it | nothing | Name the files rather than setting a flag, and measure how often a caller keeps a partial answer |

## Changes

| Date | What changed | Why |
|---|---|---|
| 2026-09-01 | Renumbered from 0005 | Nothing was committed, and two read milestones merged above it |
| 2026-09-01 | Created | Last milestone; the index is an optimisation over answers that are already correct |
