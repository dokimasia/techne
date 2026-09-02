---
rfc: 0006
title: What a tool returns
author: Roy Klopper
status: Draft
created: 2026-09-01
updated: 2026-09-01
discussion: none
supersedes: none
superseded-by: none
produces-adr: none
---

# RFC-0006: What a tool returns

## Summary

The input and the answer of every tool, with a worked example of each.

RFC-0003 fixed the envelope an answer travels in, the budget that keeps
it small and the levels a caller selects between. It left what an item
holds to whoever wrote the handler, and what got written was the
engine's own record, serialised twice. This proposal gives the two
channels of a tool result two different renderings, replaces the item
with something shaped for the question, nests declarations instead of
pointing at them, and drops the fields that restate what is beside them.

It amends the detail table, the budget order and the status field in
RFC-0003. Everything else there stands.

## Motivation

An outline costs ten times the file it summarises.

For `core/sema`, 16,787 bytes of production Go, one `outline` call puts
171,052 bytes on the wire. The same question answered as a rendered
outline is 2,798 bytes. The tool's own description tells an agent to
prefer it over reading the file.

Four causes, and they compound.

**The answer is serialised twice.** `structuredContent` holds the JSON
and `content[0].text` holds the same JSON again, escaped inside a JSON
string. The copy the model reads is the less readable one, and it is
exactly as large as the copy it does not read.

**The item count is wrong.** `asdl.py` from CPython yields 191 items, of
which 58 declare anything. The rest are parameters, imports and bindings
local to a function body. Measured across five real files from the
corpora, the answer runs 2.4× to 4.1× the source at `standard`, before
the doubling is counted.

**The framing outweighs the content.** Over those files, `span` is 42.8%
of the payload, `id` 14.0%, `parent` 12.5%, `visibility` 7.8% and
`language` 6.5%. `name` and `kind` together are 12.6%. A `language`
identical on all 191 items is stated 191 times.

**The per-item cost is the one thing that was predicted.** RFC-0003
estimated 60 tokens at `standard`; the measurement is 67. That number
was never the problem.

Two further findings decide the design rather than merely motivating it.

**`id` identifies nothing.** It is built from language, unit, name and
kind, so two methods named `Get` on different types in one package are
one identity:

```text
go:pkg#Get:method   line 6
go:pkg#Get:method   line 9
```

Eight symbols, six distinct identities. `parent` does not rescue it: a
Go method is written outside its receiver's braces, so the span rule
that fills `parent` leaves both with nothing.

**The answer omits the signature.** An agent asking what a package
offers wants `NewID(lang source.Language, unit source.Path, qualified
string, kind Kind) ID`. There is no field for it. `snippet` is the whole
declaration including its body, which for a fifty-line function is fifty
lines.

## Detailed design

### Two channels, two renderings

A tool result has an unstructured `content` list and a structured
`structuredContent` object. They have different readers. The model reads
the first; a program reads the second. Sending one thing twice serves
neither.

- `structuredContent` carries the typed answer, conforming to the
  declared `outputSchema`.
- `content[0].text` carries the same answer rendered for reading: fixed
  columns, one declaration per line, no braces and no escaping.

The specification says a tool returning structured content SHOULD also
return the serialised JSON in a text block, for backwards compatibility.
This departs from that. The compatibility being preserved is with
clients that read only `content`, which is to say with language models,
and the specification's own example of unstructured content is a weather
report in prose. A model handed escaped JSON pays for the escaping and
reads the worse of the two forms.

Nothing is lost. A client that parses `structuredContent` is unaffected.
A client that does not gets an outline it can read rather than a JSON
document it must unescape first.

### An answer states shared facts once

```json
{
  "scope": { "language": "go", "unit": "core/sema" },
  "items": [],
  "provenance": {}
}
```

`language` is constant, because a request routes to one language. `unit`
is constant when the scope is one file or one package. `path` appears
per item only when the scope covers more than one file. An item repeats
none of them.

`scope` is not provenance. Provenance is how the answer was reached;
scope is what it is about.

### An answer that ran carries no status

RFC-0003 gives every answer a status of `ok`, `degraded`, `partial`,
`unsupported` or `refused`. None of the five holds anything the answer
does not already state.

`partial` is `provenance.completeness` written a second time in the same
word. `degraded` compares `provenance.fidelity` against the
`preferred_fidelity` the caller sent, which is a comparison of two
values the caller is holding. `ok` is the absence of the other four.
`unsupported` and `refused` are the two that carry something, and both
are the `isError` an execution failure already travels as.

It does not work either. An answer cut from 191 items to 68 by the
budget reports `ok`, because truncation is not one of the five. What
told the caller was the caveat, which is where it belongs and stays.

A call that could not be served carries an error instead, and only then:

```json
{ "error": { "code": "unsupported",
             "reason": "no engine serves rename for python" } }
```

`code` is `unsupported` when nothing serves the language and role, and
`refused` when the system declined or the target does not exist. An
agent acts on the two differently: the first is permanent and it routes
around, the second is a request it can correct. The field is absent from
every answer that ran, so the common path pays nothing for it.

### An item is a declaration, and holds its children

```json
{
  "name": "Store",
  "kind": "struct",
  "line": 31,
  "signature": "type Store struct",
  "doc": "Store holds items by name.",
  "members": [
    { "name": "Name", "kind": "field", "line": 33,
      "signature": "Name string", "annotations": ["json"] },
    { "name": "Get", "kind": "method", "line": 41,
      "signature": "func (s *Store) Get(id string) int" }
  ]
}
```

- **`members`** replaces the `parent` pointer. A file's declarations are
  a tree and the answer is one, which removes a field from every nested
  item and lets a reader see the shape rather than reconstruct it. A
  declaration carrying its source text carries no members, because its
  text already holds them and sending both says a struct's fields twice.
- **Depth is the filter a kind cannot be.** A package-level `var` and a
  `local := 1` inside a function are both `variable`; what separates
  them is that one is nested in a callable. An outline returns depth 0
  and its members, and reaches deeper only when asked.
- **`line`** replaces `span` at every level but the last. A span costs
  six numbers and a path; an agent navigates by one. Offsets stay where
  a caller slices bytes, which is `source` and the write path.
- **`signature`** is the declaration without its body. Every supported
  grammar names that field `body`, so the text is the declaration's span
  minus the body's. A declaration with no body yields itself.
- **`visibility`** is stated when `unexported` or `unknown` and omitted
  when `exported`, because a default request returns exported
  declarations and the scope says so.
- **`id`** is gone. Nothing is addressable by it that is not addressable
  by name, kind and container, and it was never unique.

Nesting by span leaves a Go method outside its receiver. Belonging is
not containment: every language with methods names the receiver or the
enclosing type in the declaration, so a query can capture it. Until one
does, a method sits at depth 0 beside its type rather than inside it.

### Levels are named for what they carry

| `detail` | Adds | Answers |
|---|---|---|
| `names` | name, kind, line | is X here, and where |
| `signatures` | signature, visibility, modifiers, annotations | how do I call or implement it |
| `docs` | doc | what is it for |
| `source` | snippet, span, annotation text | what does it do |

The default follows the scope. A file is asked about because someone
means to work in it, and `signatures` is where the answer replaces
reading it. A directory is asked about to find the right file, and
`names` answers that for a fraction of the cost. One default for both
would be wrong for one of them.

Measured over one file of 3,924 bytes, against reading it:

| Level | Reading half | Structured half | Against the file |
|---|---|---|---|
| `names` | 707 | 2,337 | 0.18× |
| `signatures` | 1,319 | 4,128 | 0.34× |
| `docs` | 4,208 | 6,784 | 1.07× |
| `source`, whole file | 4,476 | 5,620 | 1.14× |
| `source`, one declaration | 2,378 | 2,632 | 0.61× |

The top two are where an outline replaces reading. The bottom two are
not, over a whole file, and cannot be: the source text of every
declaration is the file, plus the structure around it. They are levels
for a narrowed scope, which is what `names`, `kind` and `prefix` are
for, and the last row is what they cost when used that way.

The reading half is the smaller one at every level. The margin narrows
as the levels rise, because documentation and source text are prose and
code, which a structured form inflates only by escaping and by naming
the field each sits in.

Each level contains the ones above it, and a level never emits a field
it does not carry. Today `summary` writes a span whose offsets, line and
column are all zero: about a hundred bytes per item to say nothing,
while dropping the navigation the level exists for.

### What counts as an item

`include` selects kinds beyond the default, which is the kinds that
declare something other code can refer to.

| `include` | Items |
|---|---|
| omitted | declarations at depth 0, and their members |
| `["import"]` | and the file's imports |
| `["parameter"]` | and the bindings in each signature |
| `["local"]` | and declarations inside callable bodies |
| `["all"]` | every name the file binds |

`tests` defaults to false and drops the paths a language module calls
tests. Every module states the rule and nothing consults it.

### The budget drops the cheapest evidence first

RFC-0003 thins every item before dropping any item, which is right, then
drops items from the end, which is not: the tail of a file is not its
least valuable part. `asdl.py` over budget returns items 1 to 68 of 191,
so every class in the bottom two thirds is missing.

The order becomes:

1. Drop `doc` from every item.
2. Drop `snippet` from every item.
3. Drop items whose kind does not declare, cheapest first: labels, then
   imports, then type parameters, then parameters.
4. Drop members, deepest first.
5. Drop unexported items.
6. Drop items from the end.

A truncation caveat names what went, by kind, rather than only how many:

```json
{ "code": "truncated",
  "note": "191 matched, 58 returned; dropped 71 parameter, 59 local, 3 import" }
```

### Addressing is by name

Every tool that names a declaration takes the same fields, none of them
an identity:

```json
{ "scope": "core/sema", "name": "Kind.Declares", "kind": "method" }
```

`name` accepts the qualified form the language writes, because that is
how a reader recognises it. `kind` narrows an ambiguous name. An
ambiguous name is answered with the candidates rather than refused.

## The tools

Every read tool takes `scope`, `language`, `detail`, `include`, `tests`,
`max_tokens` and `preferred_fidelity`. Only what a tool adds is listed.

### `outline` — what does this declare

Adds `names`, `kind`, `prefix`, `private`.

```json
{ "name": "outline", "arguments": { "scope": "core/sema/kind.go" } }
```

Text:

```text
core/sema/kind.go — go, core/sema

   17  type Kind uint8
   21  const KindUnknown
   24  const KindModule
   31  const KindType
  118  func (Kind) String() string
  127  func (Kind) MarshalJSON() ([]byte, error)
  136  func Kinds() []Kind
  151  func (Kind) Declares() bool

syntactic, whole file. a parser matched text: a name resolved across
files is coincidence.
```

Structured:

```json
{
  "scope": { "language": "go", "unit": "core/sema", "path": "core/sema/kind.go" },
  "items": [
    { "name": "Kind", "kind": "type", "line": 17, "signature": "type Kind uint8" },
    { "name": "KindUnknown", "kind": "constant", "line": 21,
      "signature": "KindUnknown Kind = iota" },
    { "name": "String", "kind": "method", "line": 118,
      "signature": "func (k Kind) String() string" }
  ],
  "provenance": {
    "engine": "treesitter/go", "fidelity": "syntactic",
    "completeness": "total", "supportsNegativeClaim": false,
    "caveats": [ { "code": "dynamic",
      "note": "a parser matched text: a name resolved across files is coincidence" } ]
  }
}
```

A directory answers at `names`, grouped by file:

```text
core/sema — go, 6 files, 88 declarations

id.go          15 type ID · 24 func NewID
kind.go        17 type Kind · 21 const KindUnknown · 26 more
symbol.go      18 struct Annotation · 33 struct Symbol · 88 struct Unit
visibility.go  20 type Visibility · 24 const VisibilityUnknown · 5 more
```

### `search` — where is the thing called X

Adds `text`, `kind`, `private`, `limit`.

```json
{ "name": "search", "arguments": { "text": "Outranks" } }
```

Text:

```text
"Outranks" — 1 match

lang/treesitter/capture.go:145  matched name, doc
  func Outranks(candidate, held sema.Kind) bool
  Outranks reports whether one kind says more about a declaration than
  another, and so should replace it.

syntactic, whole workspace.
```

Structured:

```json
{
  "scope": { "language": "go" },
  "items": [
    { "name": "Outranks", "kind": "function", "line": 145,
      "path": "lang/treesitter/capture.go", "unit": "lang/treesitter",
      "signature": "func Outranks(candidate, held sema.Kind) bool",
      "doc": "Outranks reports whether one kind says more about a declaration than another, and so should replace it.",
      "score": 1.0, "matched": ["name", "doc"] }
  ],
  "provenance": { "engine": "treesitter/go", "fidelity": "syntactic",
                  "completeness": "total", "supportsNegativeClaim": false }
}
```

One exact match answers at `docs` rather than `names`, so the common
case needs no second call.

`score` and `matched` are not carried. Both belong to the engine that
ranked, and `Searcher` returns a declaration with no room for either, so
deriving them in the tool would be a second ranking able to disagree
with the one that fixed the order. Neither is worth that yet: the
engines here match a name and nothing else, so `matched` would read
`name` on every item. The order is the ranking, and nothing re-ranks it.

### `resolve` — what does this name denote

Adds `line`, `column`.

```json
{ "name": "resolve",
  "arguments": { "scope": "tool/outline.go", "line": 123, "column": 14 } }
```

Text:

```text
tool/outline.go:123:14 — "Fit" denotes 1 declaration

tool/budget.go:66
  func Fit(a engine.Answer[sema.Symbol], b Budget) engine.Answer[sema.Symbol]

resolved, whole workspace. reflection, string-keyed dispatch and struct
tags are not visible to any engine here.
```

Two items mean the name is ambiguous and the caller chooses.

### `relations` — how does this connect

Adds `name`, `kind`, `relation`, `limit`.

```json
{ "name": "relations",
  "arguments": { "scope": "lang/treesitter", "name": "Outranks",
                 "relation": "called-by" } }
```

Text:

```text
called-by Outranks — 1 site

lang/treesitter/outline.go:123  in Engine.declarations
  if Outranks(kind, out[seen].Kind) {

resolved, whole workspace. an empty answer here means there are none.
```

Structured:

```json
{
  "scope": { "language": "go", "unit": "lang/treesitter" },
  "items": [
    { "name": "declarations", "kind": "method", "line": 123,
      "path": "lang/treesitter/outline.go", "in": "Engine",
      "at": 123, "via": "if Outranks(kind, out[seen].Kind) {" }
  ],
  "provenance": { "engine": "gopls", "fidelity": "resolved",
                  "completeness": "total", "supportsNegativeClaim": true }
}
```

`via` is the line the edge was found on. A caller asking who calls this
wants to see the call; fetching each one is a turn per caller.

### `verify` — does this build

Adds `suites`, `max_issues`.

```json
{ "name": "verify",
  "arguments": { "scope": "techne-lang", "suites": ["lint"] } }
```

Text:

```text
techne-lang — lint, 1 issue

treesitter/metadata.go:227  warning  modernize/stringsseq
  Ranging over FieldsSeq is more efficient
  for part := range strings.Fields(unquoted) {
  fix available
```

Structured:

```json
{
  "scope": { "language": "go", "unit": "techne-lang" },
  "items": [
    { "severity": "warning", "code": "stringsseq", "source": "modernize",
      "message": "Ranging over FieldsSeq is more efficient",
      "path": "treesitter/metadata.go", "line": 227,
      "at": "for part := range strings.Fields(unquoted) {",
      "change": { "path": "treesitter/metadata.go", "line": 227,
                  "was": "for part := range strings.Fields(unquoted) {",
                  "now": "for part := range strings.FieldsSeq(unquoted) {" } }
  ],
  "provenance": { "engine": "golangci-lint", "fidelity": "resolved",
                  "completeness": "total", "supportsNegativeClaim": true }
}
```

A diagnostic that carries a fix carries it in the shape the write path
takes, so applying it needs no translation.

### `capabilities` — what can you answer

Adds `language`.

```json
{ "name": "capabilities", "arguments": {} }
```

Text:

```text
go          outline search resolve relations  gopls          resolved   session
go          outline search                    treesitter/go  syntactic  parse
python      outline search                    treesitter/py  syntactic  parse
csharp      —                                 lsp/csharp     unavailable
            omnisharp is not on PATH
```

Structured:

```json
{
  "items": [
    { "language": "go", "role": "relations", "engine": "gopls",
      "fidelity": "resolved", "cost": "session", "available": true },
    { "language": "csharp", "role": "outline", "engine": "lsp/csharp",
      "fidelity": "resolved", "cost": "session", "available": false,
      "unavailable": "omnisharp is not on PATH" }
  ]
}
```

An unavailable engine states what would make it available rather than
only that it is not.

### The write tools

Every one takes `dry_run`, defaulting to true, and answers with the
changes it would make. Applying is a second call with `dry_run` false. A
preview whose gate passed is guaranteed to apply.

| Tool | Names | Takes |
|---|---|---|
| `rename.symbol` | scope, name, kind | `new_name` |
| `move.file` | path | `to` |
| `document.symbol` | scope, name, kind | `doc` |
| `extract.function` | scope, path, lines | `new_name`, `receiver` |
| `apply.change` | — | the changes from a preview |

```json
{ "name": "rename.symbol",
  "arguments": { "scope": "core/sema", "name": "Kind", "new_name": "Category" } }
```

Text:

```text
rename Kind → Category — preview, 4 files, 31 sites

core/sema/kind.go           18 sites
core/sema/symbol.go          4 sites
tool/tool.go                 2 sites
lang/treesitter/capture.go   7 sites

build passes. apply with dry_run false.
```

Structured:

```json
{
  "scope": { "language": "go", "unit": "core/sema" },
  "items": [
    { "path": "core/sema/kind.go", "sites": 18,
      "changes": [ { "line": 18, "was": "type Kind uint8",
                     "now": "type Category uint8" } ] }
  ],
  "verified": { "gate": "build", "engine": "go build", "result": "pass" },
  "provenance": { "engine": "gopls", "fidelity": "resolved",
                  "completeness": "total", "supportsNegativeClaim": true }
}
```

`extract.function` adds the new declaration to its item list.
`apply.change` takes the `changes` from a preview verbatim, so a caller
never rewrites them.

`document.symbol` answers the same way with one file and one site, and
sends the documentation as prose with no comment markers: which markers
the language writes, how they are indented and whether they go above the
declaration or inside its body are what the tool is for.

```json
{ "name": "document.symbol",
  "arguments": { "scope": "src/store.rs", "name": "Store", "kind": "struct",
                 "doc": "Store holds items by name.\n\nIt is safe to share." } }
```

Text, where a change is read as a diff because that is how a change is
read:

```text
document Store — preview

src/store.rs:41
- /// Holds things.
+ /// Store holds items by name.
+ ///
+ /// It is safe to share.

parses (treesitter/rust). apply by calling again with dry_run false
```

Structured:

```json
{
  "scope": { "language": "rust", "path": "src/store.rs" },
  "symbol": "Store",
  "applied": false,
  "items": [
    { "path": "src/store.rs", "sites": 1,
      "changes": [ { "line": 41, "was": "/// Holds things.",
                     "now": "/// Store holds items by name.\n///\n/// It is safe to share." } ] }
  ],
  "verified": { "gate": "parse", "engine": "treesitter/rust", "result": "pass" },
  "provenance": { "engine": "treesitter/rust", "fidelity": "syntactic",
                  "completeness": "total", "supportsNegativeClaim": false }
}
```

`verified` names the gate that ran and not only its verdict. A parser
says the file is still the language it was; a build says it still
compiles. A caller told "pass" and nothing else cannot tell which promise
it was given, and the two are worth different amounts. A change nothing
could gate carries no `verified` at all rather than one saying it
passed.

## Alternatives considered

### A. Let the caller pass a field list

RFC-0003 rejected this and it stays rejected. A field list makes every
call a schema negotiation, and an agent that has to name fields spends
its first call learning which exist.

### B. Keep dropping items from the end

RFC-0003's order was chosen so that fifty names beat eight complete
entries, which is right and is kept. What it did not anticipate is items
never worth returning. Dropping a parameter before a type is the same
argument one level further in.

### C. Keep `id` and make it unique

Add a position or a counter so two `Get` methods differ.

**Why not:** position in an identity is what RFC-0002 rejected, because
an index storing it is invalidated by an edit that moves a declaration
down a file. A counter is stable only within one answer.

### D. Serialise the same JSON into both channels

Follow the specification's SHOULD exactly.

**Why not:** it doubles every answer to serve a compatibility case that
is a language model, and hands that model escaped JSON. The measurement
is 171,052 bytes where 2,798 answers the question.

### E. Return only the rendered text and no structured content

If the model reads the text, drop the JSON.

**Why not:** an answer that cannot be validated against a schema cannot
be checked, filtered or fed to another tool, and the output schema is
what lets a client type the result. The rendering is a view of the
items, not a replacement for them.

### F. Keep a status and let it grow a value for truncation

**Why not:** six values, five restating a field beside them, to carry
one bit the caveat already carries with its counts.

### G. Hand back the next call, pre-filled

Return the arguments for the call a caller is about to make.

**Why not:** it is a guess about intent made by the side with less
context. The caller knows why it searched; the server knows only what
matched. Turn cost is better paid down by answering the first question
completely, which is what the levels and the signature are for.

### H. Keep `span` on every item

**Why not:** correct and unused. An agent navigates by line and slices
by offset, and it slices in the write path, which has the span.

## Drawbacks

- Two renderings is two things to keep in step, and a bug in the text
  one is invisible to a schema.
- Departing from the specification's SHOULD means a client that reads
  only `content` and expects JSON there gets prose instead.
- Four levels rather than three, with different names, so every caller
  that named one has to change.
- `signature` is a field the parser tier has to produce and cannot
  always: a grammar naming no body field yields the whole declaration,
  which makes `signatures` cost what `source` costs for that language.
- Nesting fights the budget. Dropping a parent drops its members, so the
  order has to drop members first, and a deep tree truncates less
  gracefully than a flat list.
- Addressing by name is ambiguous where a language allows two
  declarations to share a name, kind and container. The answer is a list
  the caller picks from, which is a turn.
- `in` needs the parser to read a receiver, a pattern per language
  rather than one rule over spans. Until it does, two methods sharing a
  name and kind are told apart only by their line.

## Unresolved and future work

The signature of a type is not the signature of a function. A struct's
fields are its interface, and the level giving a function its parameters
should probably give a struct its fields rather than the word `struct`.
What that costs is unmeasured.

Whether the rendered form should be stable enough to diff between two
calls, so an agent sees what changed in a file it has already read, is a
question nobody has asked yet.

Ranking is unspecified beyond the tie-breaks already implemented.
Whether one scorer serves names and documentation together is a question
RFC-0002 left open and this does not close.

## References

| What | Where |
|---|---|
| MCP tool results, structured content, and the SHOULD this departs from | https://modelcontextprotocol.io/specification/2025-06-18/server/tools |
| tree-sitter field names, including `body` | https://tree-sitter.github.io/tree-sitter/using-parsers#node-field-names |
| `reflect.StructTag`, the grammar a Go struct tag follows | https://pkg.go.dev/reflect#StructTag |
