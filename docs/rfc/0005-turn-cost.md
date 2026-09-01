---
rfc: 0005
title: Turn cost
author: Roy Klopper
status: Draft
created: 2026-09-01
updated: 2026-09-01
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

# RFC-0005: Turn cost

## Summary

The number of round trips an agent spends is a first-class cost, next to
correctness and context size. Five mechanisms cut it: say when to use a
tool instead of the built-in one, expand an unambiguous result without
being asked, return ranked candidates rather than an error, carry the
input to the likely next call in the answer, and take the target by name
rather than by position.

## Motivation

An agent renaming a symbol with the built-in tools greps for it, reads
three files, edits each one, runs the build, reads the failure, and edits
again. Six round trips, and the result still misses references that grep
could not see. A type-checked rename is one round trip and cannot miss
them.

That gap is the reason to build this at all, and it is easy to give back.
A tool that answers exactly what was asked and nothing more turns one
round trip into two: search returns a symbol, then the agent asks what it
contains. A tool that errors on an ambiguous name turns one into three.

Turn cost is also not the same as context cost, and the two pull against
each other. Returning everything an agent might want next saves a turn
and spends context. The mechanisms below are chosen to spend context only
where the next call is near-certain.

techne is used to build techne, so this project is the first to feel it.

## Detailed design

### 1. Say when to use this instead of the built-in tool

An agent has `grep`, `read` and `edit` already, and reaches for them
unless told otherwise. Every tool description opens by naming the
built-in it replaces and the reason.

```
PREFER OVER grep for finding uses of a symbol. Type-checked: returns
every reference including method-on-receiver cases that grep cannot
disambiguate, and no false positives from same-named identifiers in
unrelated scopes.
```

Without that sentence the tool is not chosen, and a tool that is not
chosen has no turn cost to measure. This is the cheapest of the five and
the one that decides whether any of the others matter.

### 2. Expand an unambiguous result

Every read tool takes `expand`, default true. When a call produces
exactly one result and there is an obvious next question, the answer
carries it.

| Tool | One result means | `expand` adds |
|---|---|---|
| `search` | one symbol matched | that symbol's declaration, at `standard` detail |
| `resolve` | the name binds to one declaration | the declaration and its span |
| `relations` | one edge | the symbol at the other end |

An agent that searched for `foldPatch` wanted to see `foldPatch`. Making
it ask again is a round trip spent confirming what the search already
knew.

Nothing expands when the result is ambiguous, because the expansion would
be a guess about which match was meant, and paid for in context.

### 3. Ambiguity returns candidates, not an error

A name that matches several declarations produces `status: ok` with the
candidates ranked, each carrying enough to choose: kind, path, span, and
the one-line summary from its doc comment.

```json
{
  "items": [
    { "id": "go:./core/trust#Status:type", "kind": "type",
      "path": "core/trust/status.go", "summary": "Status is what happened to a request." },
    { "id": "go:./core/gate#Status:type", "kind": "type",
      "path": "core/gate/status.go", "summary": "Status reports whether the gate passed." }
  ],
  "status": "ok",
  "ambiguous": true
}
```

Refusing with "ambiguous symbol: Status" costs a turn and tells the agent
nothing it can act on. The candidate list lets it choose on the next call
with a disambiguator, and often lets it choose without another call at
all.

### 4. An answer carries the input to the next call

Where a result has a near-certain follow-up, the answer carries what that
follow-up needs.

`verify` returns diagnostics, and every diagnostic with a known fix
carries the fix as an `edit.Plan` ready to hand to the change tool. A
lint, fix and re-verify cycle becomes two round trips rather than five,
because the agent never has to work out the edit from the message.

The rule and its limit: carry the next input when the next call is
near-certain and the payload is small. Do not carry a fix nobody asked
for when the diagnostic has three plausible ones, because that is three
payloads to buy one turn that may not be taken.

### 5. Take the target by name, and by position only to disambiguate

Every tool that names a symbol takes `symbol` as a string, with `package`
or `path` to scope it. `file` and `line` are optional and only needed
when the name alone is ambiguous, which is the same case that produces a
candidate list.

A position-first interface reads well and costs a turn: the agent has to
read the file before it can point at anything. The agent already has the
name. It got the name from the error message, the search result, or the
code it just wrote.

### Reporting the cost

Every answer carries what it cost, so the claim in this document is
measurable rather than asserted.

```go
// Spent is what producing one answer cost.
type Spent struct {
	Millis    int  // wall clock
	Tokens    int  // estimated, on the serialised answer
	Expanded  bool // an unambiguous result was expanded without being asked
}
```

`Expanded` is the one that matters for this proposal. Counting how often
expansion happens, and how often the agent then asks the follow-up
anyway, is what tells us whether mechanism 2 is worth its context.

## Alternatives considered

### A. Composite tools for common chains

Ship `search_explore` alongside `search` and `explore`, so the chain is
one call.

**Why not:** it adds a tool per chain, and the tool list is the thing an
agent reads before every decision. One tool per operation was chosen so
the list stays proportional to what the system does rather than to the
ways of combining it. The `expand` flag buys the same turn on the common
case without a second entry in the list. A composite becomes worth it if
measurement shows agents chaining two specific tools most of the time and
ignoring `expand`.

### B. Return everything, always

Skip `detail` and the budget, and let the agent ignore what it does not
need.

**Why not:** it trades a turn for context, and context is the scarcer of
the two. An outline that costs more than reading the file has removed the
reason to call the tool.

### C. Let the agent ask for a plan of calls

Expose one tool that takes a goal in natural language and returns a
sequence of calls to make.

**Why not:** the model is better at that than we are, and it has the
conversation. It also puts a second planner in the system with no way to
check it against what the engines can do.

### D. Do nothing beyond mechanism 1

Publish routing hints and let the agent chain calls itself.

**Why not:** it is the honest baseline and it leaves the two-call pattern
in place for the commonest question there is, which is "what is this
thing I just found".

## Drawbacks

- `expand` spends context on every unambiguous result, including the ones
  the agent was not going to follow up. Nobody has measured how often
  that is.
- Mechanism 4 puts fix construction in the verifier, so a language that
  reports diagnostics without fixes gets none of the saving, and the
  shape of a carried fix has to match the change tool exactly or it is
  worse than nothing.
- Ranked candidates need a ranking, and a bad one is worse than an error
  because the agent trusts the first entry.
- Name-first targeting cannot address an anonymous function or a
  statement, so operations on a span still take a span.
- Reporting `Spent` on every answer adds a field nobody reads until
  someone is measuring.

## Open questions

1. Is `expand` right as a default-on flag, or should it trigger on a
   token estimate, expanding only when the expansion is small?
2. What ranks candidates? Same-package first is obvious. Whether an
   exported symbol outranks an unexported one, and whether recently
   edited files matter, is not obvious.
3. Does mechanism 4 extend past `verify`? A rename that fails on a
   precondition could carry the refreshed plan, which is either a large
   saving or a large payload nobody uses.

## Unresolved and future work

Measuring turn counts against a real agent on a real task is not proposed
here, and every number in this document is a claim until it happens.

Caching an answer so a repeated question inside one conversation costs
nothing is not proposed.

## References

| What | Where |
|---|---|
| MCP tool descriptions are what a model routes on | https://modelcontextprotocol.io/specification/2026-07-28/server/tools |
