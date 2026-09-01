---
adr: 0001
title: One tool per operation, with fidelity on the answer
status: Accepted
date: 2026-09-01
supersedes: none
superseded-by: none
rfc: none
---

# ADR-0001: One tool per operation, with fidelity on the answer

## Status

Accepted

## Context

techne serves several languages through one MCP server, and an agent
picks a tool by reading its name and description. There are two ways to
lay out N operations across M languages: one tool per operation, which
works out the language from the target it is given, or one tool per
language-and-operation pair.

Two things pull against each other. An agent shown a long tool list
routes worse and falls back to the built-in read and edit tools it
already knows, which argues for the short list. But a rename that means
"type-checked and rolled back if the build breaks" in one language and
"text substitution that matched the same identifier" in another is
dangerous under a single name, and a short list makes that collision
easy to reach.

A check on the answer is what makes one name safe across languages. An
operation declares the weakest evidence it can be correct on, the system
compares that against what the engine had, and refuses when it falls
short. That only works if every answer carries the evidence behind it.

## Decision

We will expose one tool per operation across every language, with each
answer stating the fidelity of the evidence behind it, because putting
the language in the tool name makes an agent route on the file extension
instead of on what the system can answer.

An agent asks what techne can do by calling one capability tool, not by
reading which tools exist. Operations are declared even for languages
that cannot serve them, so a caller is told the language cannot do this
rather than that no such operation exists.

## Alternatives Considered

### Per-language tool namespaces

One tool per pair: `go_rename`, `python_rename`, `rust_rename`. Each
tool's description states exactly what that language guarantees, and an
agent working in a Go repository sees Go tools.

It lost on the shape of the question an agent has to answer. "Can you
rename this Python symbol" becomes "is there a tool called
`python_rename`", which an agent can only answer by scanning a list that
grows with every language added. The caller then works out capability
from the tool list instead of reading it from the system, and an absent
name cannot distinguish "not supported" from "not installed". The list
also grows as the product of operations and languages, which is the count
that pushes an agent back to grep and patch.

### One dispatch tool with the operation as a parameter

A single `techne` tool taking `operation: "rename.symbol"` and a bag of
arguments.

It lost on validation. The input schema becomes the union of every
operation's parameters, so nothing is required, nothing is checked before
the handler runs, and the agent gets a runtime error where a schema
mismatch should have been. It also loses the per-tool description, which
is where an agent learns when to use the operation at all.

### One binary per language

Separate servers, each serving one language, so a caller installs only
what they work in.

It lost on mixed repositories, which are the normal case. A caller runs
several servers, no single answer can span them, and capability again
becomes "which server did someone install", which the agent cannot ask
and the system cannot answer.

## Consequences

**Positive:**

- The system reports what it can do for each language, so a caller no
  longer works it out from which tool names exist. "We cannot do this
  here" becomes a different answer from "no such operation".
- The tool list stays proportional to the number of operations, not to
  operations times languages, so adding a language adds no tools.
- One dispatcher per read role and one write pipeline serve every
  language, so the rule about what counts as degraded cannot drift
  between two operations.

**Negative:**

- Every operation has to return a refusal as an ordinary result rather
  than as an error. The refusal is the whole safety argument for sharing
  a name across languages, so it needs testing per language and per
  operation, with the same weight as the success path.
- Every response carries provenance whether or not the caller reads it,
  which makes the output schema larger for callers that only ever use one
  language.
- An agent that ignores the fidelity field gets a syntactic answer and
  treats it as resolved. The design moves the guarantee into a field, and
  a field can be skipped in a way a tool name cannot.
- Naming a tool cannot signal that an operation is cheap in one language
  and expensive in another, so cost has to be reported rather than
  implied.

**Neutral:**

- The fidelity field is only worth carrying if operations declare a
  minimum to check it against, so this decision commits the write path to
  declaring one per operation.

## References

| What | Where |
|---|---|
| MCP tool definitions and the name field, revision 2026-07-28 | https://modelcontextprotocol.io/specification/2026-07-28/server/tools |
