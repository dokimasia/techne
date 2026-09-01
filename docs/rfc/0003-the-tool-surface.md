---
rfc: 0003
title: The tool surface
author: Roy Klopper
status: Accepted
created: 2026-09-01
updated: 2026-09-01
discussion: none
supersedes: none
superseded-by: none
produces-adr: none
---

# RFC-0003: The tool surface

## Summary

What an agent sees: how tools are named, what a request and an answer
look like, how an answer is kept small enough to be worth reading, and
how "this language cannot do that" reaches the caller. Every answer
carries its evidence, and an answer too large for the budget is thinned
before anything is dropped.

## Motivation

The consumer is a language model with a fixed context window, and that
changes what a good answer is. An outline of a 2000-line file is a
correct answer and a useless one: it costs more context than reading the
file, which is the thing the tool exists to avoid.

Nothing in the current documents says how large an answer may be, what
gets cut first, or what a caller is told when something was cut. Without
a rule, the first large repository turns every call into a context
overflow and an agent goes back to grep, which is the outcome the whole
project is trying to avoid.

The second gap is the capability report. The decision to expose one tool
per operation rests on an agent being able to ask what the system can do
rather than infer it from which tool names exist. That report has been
named in three documents and specified in none.

## Detailed design

### Naming

A tool is named for what it does. A subject is appended when the verb
alone is ambiguous.

| Tool | Question |
|---|---|
| `outline` | what does this file or directory declare |
| `search` | where is the thing called X |
| `resolve` | what does this name denote, and is it ambiguous |
| `relations` | how does this symbol connect to the rest |
| `verify` | does this build, and what does the linter say |
| `capabilities` | what can you answer, per language, and how well |
| `rename.symbol` | rename this declaration and every reference |
| `move.file` | move this file and fix what referred to it |

`outline` needs no subject. `rename` does, because renaming a symbol and
renaming a file are different operations with different preconditions.
The dot is not a read and write marker; it appears wherever the verb
needs one.

### Targeting is by name

Every tool that points at a symbol takes `symbol` as a string, scoped by
`package` or `path`. `file` and `line` are optional, and needed only when
the name alone matches more than one declaration.

```json
{ "symbol": "Status", "package": "core/trust" }
{ "symbol": "kind", "file": "core/query/dispatch.go", "line": 42 }
```

A position-first interface would make the agent read the file before it
could point at anything, which is a round trip spent on something it
already knew. The agent has the name: from an error message, from a
search result, or from the code it just wrote.

When a name matches several declarations the answer ranks the candidates
rather than refusing, and each carries what is needed to choose.

The engine ports take a position, because that is how a type checker and
a language server are both addressed. Converting a name to a position is
the service's job, and it is the same lookup `search` performs. Pushing
that conversion onto the agent would move a round trip from the server to
the conversation.

The protocol permits this. Tool names may hold ASCII letters, digits,
underscore, hyphen and dot, between 1 and 128 characters, and the
specification gives `admin.tools.list` as a valid example.

### Every answer carries its evidence

One output shape for every read tool.

```json
{
  "items": [ ... ],
  "status": "ok",
  "provenance": {
    "engine": "gopls",
    "fidelity": "resolved",
    "completeness": "total",
    "supportsNegativeClaim": true,
    "caveats": [
      { "code": "dynamic",
        "note": "reflection, struct tags and string-keyed dispatch are not visible",
        "paths": [] }
    ]
  }
}
```

`supportsNegativeClaim` is computed rather than stored, and it is in the
payload because an agent should not have to know that it means fidelity
`resolved` together with completeness `total`. An empty `items` with
`supportsNegativeClaim: true` means there are none. The same empty
`items` with it false means none were found.

Answers travel as `structuredContent` against a declared `outputSchema`,
with the same JSON serialised into a text block, which is what the
specification asks of a tool that returns structured content.

### Refusal is a result, not a protocol error

| What happened | `status` | MCP shape |
|---|---|---|
| An engine answered | `ok` | normal result |
| Answered below the fidelity asked for | `degraded` | normal result |
| Part of the scope was not covered | `partial` | normal result |
| No engine serves this language and role | `unsupported` | result with `isError: true` |
| Policy declined, or the target does not exist | `refused` | result with `isError: true` |

`unsupported` and `refused` are tool execution errors rather than
JSON-RPC errors, because the specification says clients should hand
execution errors to the model so it can correct itself, while protocol
errors are less likely to produce a recovery. "Python has no rename
planner" is something an agent can act on. It is not a malformed request.

### The output budget

Every read tool takes `max_tokens`, defaulting to 6000, estimated from
the serialised answer.

Tokens rather than bytes because that is the budget the caller spends. An
agent deciding whether it can afford a call counts context, and asking it
to convert from bytes makes it guess at a ratio the server can apply
itself. The estimate needs no real tokeniser: it only has to be good
enough to decide what to drop, and the caveat reports what came back.

When an answer exceeds it, thin every item before dropping any item:

1. Drop `doc` from every item.
2. Drop `snippet` from every item.
3. Drop items from the end, in the order the engine returned them.

Every truncated answer carries `CaveatTruncated` naming how many items
matched and how many are present.

```json
{ "code": "truncated", "note": "127 matched, 40 returned", "paths": [] }
```

Fifty names with no documentation answers "what calls this". Eight
complete entries that do not say forty-two more exist answers a
different question, wrongly. The order above is what keeps the first
outcome and rules out the second.

`detail` selects what an item carries before the budget is applied:

| `detail` | Each item holds | Estimated per item |
|---|---|---|
| `summary` | id, name, kind, path | 20 tokens |
| `standard` | adds span, parent, exported (the default) | 60 tokens |
| `full` | adds doc and snippet | 300 tokens |

The per-item figures are estimates nobody has measured, published because
an agent choosing a `detail` level needs some number to plan against. The
answer reports what it spent, so real calls correct these rather than
anyone arguing about them.

An agent that asks for `full` on a large scope gets `standard` back with
a truncation caveat rather than an error.

### Paths are relative

Every path in a request and an answer is relative to the workspace root
and slash-separated. An absolute path in a request is an error, because
answering it would leak the machine's directory layout into an agent's
context and make the answer unusable on any other machine.

### Capabilities

```json
{
  "items": [
    { "language": "go", "role": "relate", "engine": "gopls",
      "fidelity": "resolved", "cost": "session", "available": true },
    { "language": "python", "role": "relate", "engine": "treesitter",
      "fidelity": "syntactic", "cost": "parse", "available": true },
    { "language": "python", "role": "plan", "engine": "",
      "available": false,
      "unavailable": "no planner for python: rename.symbol needs resolved evidence" }
  ]
}
```

A capability carries no coverage claim. What an engine will cover
depends on the scope it is asked about and on how far its index has got,
so completeness is a property of an answer rather than of an engine.

An agent calls this once and knows what it can rely on. It answers "can
you rename this Python symbol" without the agent scanning a tool list,
and it distinguishes a language nothing serves from one whose engine is
not installed.

### Which tools exist

Every operation with a planner in at least one language is a tool.
Calling it for a language that cannot serve it returns `unsupported` with
the reason, which is the whole argument for one tool per operation.

An operation nothing implements anywhere is not a tool. It appears in
`capabilities` with `available: false`, so an agent is told the operation
exists and cannot run rather than that no such operation exists.

That rule keeps the list proportional to what works. Eight tools serve
ten languages today.

`verify` runs a language's gate without changing anything, so an agent
can check its own work before asking for a change. It is the same
verifier the write path runs, reached directly, and a failed run carries
its fixes the same way.

## Alternatives considered

### A. One tool per language and operation

`go_rename`, `python_outline`, and so on.

**Why not:** this was settled when the decision was taken to expose one
tool per operation with the fidelity on the answer. Reopening it needs a
new decision record rather than a section here.

### B. A byte budget rather than a token budget

Measure the serialised answer in bytes, which the server knows exactly.

**Why not:** it is exact about the wrong quantity. The caller spends
context, so a byte figure makes it convert, and it will convert with a
worse ratio than the server has. An estimate of the right number beats an
exact count of the wrong one.

### C. Drop items first, keep every field

Return fewer, richer results.

**Why not:** the questions these tools answer are mostly "what is there",
and a name answers that where a snippet does not. Dropping items first
also loses the information that there were more, unless the caveat is
added anyway, at which point the thinning was free.

### D. Let the caller pass a field list

Instead of three `detail` levels, take the fields wanted.

**Why not:** it moves a decision to the agent that the agent has no basis
for, and it makes every response schema conditional on the request. Three
named levels are a smaller surface and cover the cases seen so far. If a
fourth shape is needed, adding a level is cheaper than adding a
projection language.

## Drawbacks

- `max_tokens` has a default nobody has measured against a real agent.
  6000 is a guess, and the right number depends on the client's context
  window and how much of it the caller has already spent.
- The token count is estimated, so an answer can exceed the budget it
  reported staying inside. The estimate has to be conservative, which
  means dropping items that would have fitted.
- Computing `supportsNegativeClaim` in the payload duplicates a rule that
  also lives in the trust package. Two implementations can disagree, so
  the presenter has to call the library function rather than reimplement
  the comparison.
- Byte-counting the serialised answer means serialising, measuring,
  thinning and serialising again. For a large outline that is two or
  three passes over the same data.
- Three `detail` levels multiply the output schema by three unless the
  schema declares every field optional, which weakens what the schema
  tells the agent.
- Relative paths mean the workspace root is a server-level setting, and a
  request that spans two workspaces cannot be expressed.

## Unresolved and future work

Every figure in the budget is an estimate: the default ceiling, the
per-item costs, and how much surrounding code a snippet should carry.
The first real agent on a real repository settles all of them, and the
answer reports what it spent so they can be corrected from calls rather
than argued about.

One server serves one workspace. A caller working across two checkouts
runs two servers, and no answer spans them. Nothing has asked for more.

Prompts and resources, the other two things an MCP server can offer, are
not proposed here. A resource per workspace file is an obvious idea and
would compete with `outline` for the same job.

Streaming a large answer instead of truncating it is not proposed. It
would change the shape of every read tool.

## References

| What | Where |
|---|---|
| Tool names: allowed characters, length, and the dotted example | https://modelcontextprotocol.io/specification/2026-07-28/server/tools |
| Tool execution errors are given to the model for self-correction | https://modelcontextprotocol.io/specification/2026-07-28/server/tools |
| Structured content should also be serialised into a text block | https://modelcontextprotocol.io/specification/2026-07-28/server/tools |
