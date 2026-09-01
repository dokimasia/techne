---
adr: 0003
title: Evidence has two axes, not one
status: Accepted
date: 2026-09-01
supersedes: none
superseded-by: none
rfc: RFC-0002
---

# ADR-0003: Evidence has two axes, not one

## Status

Accepted

## Context

An agent that reads an empty caller list as "there are none" and deletes
the code is the failure this system exists to prevent. Something has to
say when that reading is safe.

One ordered tier per answer looked like enough. A parser matches text, an
index resolves a reference, a type checker binds a name, and only the
last licenses reading emptiness as absence.

Serving five languages broke it. A language server binds names through
the type system, so on the single scale it belongs at the top. But it
answers from an index that may still be building, and an index that has
read half the workspace has found half the references. Placing it below
the top means Python, Java, Rust and TypeScript can never support a
negative claim whatever their server knows, which is four of five
languages. Placing it at the top means a server answering mid-index says
"no callers" as confidently as one that has finished.

Neither position is right, because the two are different questions.

## Decision

We will describe evidence with two independent values, how an answer was
bound and how much of the scope was examined, because reading emptiness
as absence needs both and no single ordering expresses that.

`trust.Fidelity` says how an answer was bound. `trust.Completeness` says
what the engine covered. `trust.SupportsNegativeClaim` requires resolved
binding together with total coverage.

## Alternatives Considered

### One ordered tier, with language servers below the top

Keep a single scale and place a language server at the indexed tier.

It lost because four of the five languages then never support a negative
claim, whatever their server knows. "Find every caller" is the question
an agent asks before deleting code, and answering it usefully in one
language only would make the others read-only in practice.

### One ordered tier, with language servers at the top

Keep a single scale and let a server claim the resolved tier for the
roles where it binds through types.

It lost because a server still building its index reports the same tier
as one that has finished. That is the original failure, moved from the
tier boundary to the timing of the request, where it is harder to see.

### A fifth tier between indexed and resolved

Add a value meaning type-checked over an incomplete scope.

It lost because the enum stops being an ordering. Sorting engines by
fidelity is what the catalogue does, and a tier stronger on binding and
weaker on coverage has no correct position in that sort.

### A boolean on the engine

Let each engine report whether its own empty answers are authoritative.

It lost because it puts the judgement in the component with the strongest
reason to be optimistic about itself, and no service can catch an engine
that overstates.

## Consequences

**Positive:**

- A language server reaches the strongest binding it earns and still
  cannot claim absence while its index is warming.
- The write path gets a precise admission rule: an operation that
  rewrites references needs both values, because finding every reference
  is itself a claim that no others exist.
- An engine reports only what it knows. Coverage is the one judgement it
  makes about its own answer, and a service supplies the rest.

**Negative:**

- Every answer carries two values and every service compares both. A
  caller reading only the tier gets a subtly wrong idea of what an empty
  list means, which is why the tool surface computes the conclusion for
  it.
- Completeness is an engine's claim about itself and nothing here catches
  an engine that reports total coverage after skipping a file. The
  conformance suite has to test it against a known count.
- Two values sort independently, so the catalogue orders by fidelity
  alone and coverage plays no part in choosing an engine.

**Neutral:**

- The names needed a prefix. `trust.Status` already had a `Partial`, so
  the coverage values are `ScopeUnknown`, `ScopePartial` and
  `ScopeTotal`, and `Status` keeps the bare names because it appears on
  every answer.

## References

| What | Where |
|---|---|
| LSP servers index asynchronously and report progress | https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#workDoneProgress |
