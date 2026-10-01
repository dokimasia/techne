---
rfc: 0006
title: What a tool returns
author: Roy Klopper
status: Accepted
created: 2026-09-01
updated: 2026-09-30
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
engine's own record, serialised twice. This proposal sends a tool result
as text rendered for a model, with the typed answer as structured
content for a client that asks for it. It replaces the item with
something shaped for the question, nests declarations instead of
pointing at them, and drops the fields that restate what is beside them.

It amends the text block, the detail table, the budget order and the
status field in RFC-0003. Everything else there stands.

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

### Text by default, JSON on request

A tool result has an unstructured `content` list and a structured
`structuredContent` object. The specification intends the text for the
model and the structured content for a program. When a result has
structured content, Claude Code hands the model that content in place of
the text.

- By default the server sends `content[0].text` alone: the answer
  rendered for reading, in fixed columns, one declaration per line, with
  no braces and no escaping. The server declares no `outputSchema`,
  because the specification requires structured content from a tool that
  declares one.
- Started with `--structured`, the server also sends `structuredContent`,
  the typed answer, and declares the `outputSchema` it conforms to. A
  client that validates, filters or passes an answer on starts the server
  this way, and so does the corpus harness.

We measured nine calls of the read tools on 2026-09-30. Their rendered
text is 21,298 characters and their JSON 37,847, so the text is 0.56
times the JSON. `path` on every item is about 27% of the JSON, and 33%
for `outline` and `relations`. Provenance is 3.5% of the JSON overall,
and about half of a small `resolve` or `search` answer.

The specification says a tool returning structured content SHOULD also
return the serialised JSON in a text block, for backwards compatibility.
This departs from that. The compatibility being preserved is with
clients that read only `content`, which is to say with language models,
and the specification's own example of unstructured content is a weather
report in prose. A model handed escaped JSON pays for the escaping and
reads the worse of the two forms.

### An answer states shared facts once

```json
{
  "scope": { "language": "go", "unit": "core/sema" },
  "items": [],
  "provenance": {}
}
```

`language` is constant, because a request routes to one language. `unit`
is constant when the scope is one file or one package. An answer about a
directory of two or more units states `directory` in place of `unit`. `path` appears on
an item only when the item is not in the file of the scope: in an answer
about a directory, and on a declaration that `resolve` finds in another
file. An item repeats none of them.

`scope` is not provenance. Provenance is how the answer was reached;
scope is what it is about.

The text ends with one line of evidence: the fidelity, the completeness,
and the note of each caveat that is not `dynamic`. The note of a dynamic
caveat restates the limit of the fidelity, so the text writes it only for
an empty answer, beside the negative claim, where it states why the answer
can be empty. The structured answer keeps every caveat.

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
  a caller slices bytes, which is `source` and the write path. The span
  at `source` counts its lines and columns from one, as `line` does. Its
  offsets count bytes from zero. It does not repeat the path of its item.
- **`signature`** is the declaration without its body. Every supported
  grammar names that field `body`, so the text is the declaration's span
  minus the body's. A declaration with no body yields itself.
- **`visibility`** is stated when `unexported` or `unknown` and omitted
  when `exported`, because a default request returns exported
  declarations and the scope says so.
- **`id`** is gone. Nothing is addressable by it that is not addressable
  by name, kind and container, and it was never unique.

Nesting by span leaves a Go method outside its receiver, at depth 0
beside its type. Belonging is not containment: the Go query captures the
receiver, so the qualified name of the method is `Store.Get`, and a
caller addresses the method by that name.

### Levels are named for what they carry

| `detail` | Adds | Answers |
|---|---|---|
| `names` | name, kind, line | is X here, and where |
| `summaries` | summary, the first sentence of doc | what is each thing here for |
| `signatures` | signature, visibility, modifiers, annotations | how do I call or implement it |
| `docs` | doc | what is it for |
| `source` | snippet, span | what does it do |

- The summary is the first sentence of the first paragraph of the doc,
  before any list, cut at 160 bytes.
- It is on the line of the name, so a package costs one line per
  declaration.
- `summaries` adds it to `names` alone. From `signatures` on a
  declaration has no summary, and from `docs` on its doc contains it.
- At `names` and `summaries` a field or a property shows its
  signature, which is its name and its type.
- At `summaries` a member without a summary, a type or members of its
  own is left out, because its line would state only its name.

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

`tests` defaults to false. An engine then leaves out the files that the
`IsTest` rule of the language declaration names, and `tests` true keeps
them. The request passes the choice to the engine as
`engine.Request.Tests`.

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
how a reader recognises it. `kind` narrows an ambiguous name, and so
does `line`, counted from one, to the declarations whose span contains
it. The line picks one of the overloads of a method. A name that is
still ambiguous is refused, and the reason lists the kind and the site of
each candidate.

### Numbers in an input

Every integer of an input is a line, a column or a count. A line or a
column counts from one, and the input schema states the minimum 1 for
one that a tool requires. A count of 0 selects its default, as an
omitted count does, and the schema states the minimum 0. A number below
its minimum is an input error, as a missing required field is.

### Kinds in an input

The input schema states `kind` as a string without an enum. Five tools
take a kind, and the enum of every kind in each of them was an eighth of
the tool list. A word outside the kinds is refused, and the reason lists
every kind.

### Aliases in an input

A field can have a second name that the input schema does not list. A
tool reads the value of the second name when the field is absent, and
ignores it when both are given. Agents wrote `query` for the `text` of
`search` in 7 of 12 searches over kubernetes on 2026-09-30.

| Tool | Field | Second name |
|---|---|---|
| `search` | `text` | `query` |
| `relations`, `rename.symbol`, `document.symbol` | `name` | `symbol` |

`relation` takes `implementations` and `implementation` as
`implemented-by`. textDocument/implementation is the request of LSP for
that relation.

## The tools

Every read tool but `capabilities` and `workspace` takes `scope`,
`language` and `preferred_fidelity`. `workspace` takes `scope` and
`language`. Only what a tool adds is listed.

### `outline` — what does this declare

Adds `detail`, `names`, `kind`, `prefix`, `private`, `include`, `tests`,
`max_tokens`.

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

syntactic, total coverage.
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

The answer for a directory is at `names`, grouped by file. The summary
of the doc of its unit follows the heading when the directory is one
unit. Each file is named on a line of its own, and each declaration and
member under it states its line alone:

```text
core/sema — go, 88 declarations
Package sema describes declarations, the edges between them, and the IDs that identify them across processes.

core/sema/id.go
   15  type ID
   24  function NewID
core/sema/kind.go
   17  type Kind
   21  constant KindUnknown
```

At `summaries` each line also has the first sentence of the doc:

```text
core/sema/id.go
   15  type ID  ID identifies a declaration across processes and runs.
```

A declaration with members in the answer states their count by kind on
its line, such as `2 methods, 1 field`, unless the answer shows its
source.

### `workspace` — what units does this contain

Adds `private`, `tests`, `max_tokens`. `scope` is a directory, and the
workspace root when omitted.

```json
{ "name": "workspace", "arguments": {} }
```

Text:

```text
. — 36 units

techne-core/engine  go  11 files  76 declarations  Package engine defines the ports an engine implements and the catalogue that selects engines for a request.
techne-core/sema    go   6 files  69 declarations  Package sema describes declarations, the edges between them, and the IDs that identify them across processes.

syntactic, total coverage.
```

A unit is the unit of the IDs of its declarations, such as a Go package.
Each unit states its language, its files, the declarations at the top
level that other units can use, or every one with `private`, and the
summary of the doc of the package or the module that it declares. A
map that does not fit the budget keeps the units with the most
declarations, in the order of their paths, and a caveat counts the units
it leaves out. The first units by path of a large repository are often
its build files. It is the first call in a workspace, and `outline` at
`summaries` is the second.

### `search` — where is the thing called X

Adds `text`, `kind`, `private`, `limit`, `detail`, `include`, `tests`,
`max_tokens`.

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

syntactic, total coverage.
```

Structured:

```json
{
  "scope": { "language": "go" },
  "text": "Outranks",
  "items": [
    { "name": "Outranks", "kind": "function", "line": 145,
      "path": "lang/treesitter/capture.go",
      "signature": "func Outranks(candidate, held sema.Kind) bool",
      "doc": "Outranks reports whether one kind says more about a declaration than another, and so should replace it." }
  ],
  "provenance": { "engine": "treesitter/go", "fidelity": "syntactic",
                  "completeness": "total", "supportsNegativeClaim": false }
}
```

One exact match answers at `docs` rather than `names`, so the common
case needs no second call. An empty `text` matches every name, so
`search` refuses it and names `outline`, which lists every declaration.

- A search without `private` that matches no exported declaration runs
  again over the unexported ones. A caller who names a declaration wants
  it wherever it is declared.
- A package or a module is one match, however many files state its
  clause. The match is the declaration with a doc, or the first one when
  none has a doc.

A `text` with white space is a description, because no name contains a
space. It matches the declarations whose doc or name contains each of its
words of three letters or more, case folded, and the declarations with
the most occurrences of the words come first. The parser reads the doc
of every declaration in scope for it, which took 2.6 s over the 13,392
Go files of kubernetes for the same read by `workspace`.

A description that matches nothing and that is written as a declaration
runs again with the name that it declares. The name is the first name
outside brackets that a parenthesis follows without a space, as `Get` in
`func (s *Store) Get() int`. A text without such a name declares its first
name outside brackets that is no keyword, when it contains a keyword of
the language, as `Store` in `type Store struct`.

`score` and `matched` are not carried. Both belong to the engine that
ranked, and `Searcher` returns a declaration with no room for either, so
deriving them in the tool would be a second ranking able to disagree
with the one that fixed the order. Neither is worth that yet: the
engines here match a name and nothing else, so `matched` would read
`name` on every item. The order is the ranking, and nothing re-ranks it.

### `resolve` — what does this name denote

Adds `line`, `column`, `detail`, `include`, `max_tokens`.

```json
{ "name": "resolve",
  "arguments": { "scope": "tool/outline.go", "line": 123, "column": 14 } }
```

Text:

```text
tool/outline.go:123:14 — "Fit" denotes 1 declaration

tool/budget.go:66
  func Fit(a engine.Answer[sema.Symbol], b Budget) engine.Answer[sema.Symbol]

resolved, total coverage.
```

Two items mean the name is ambiguous and the caller chooses.

The answer is the declaration that the name denotes, whatever `include`
selects. A parameter, an import and a local declaration are answers, and
`include` selects the members of each.

### `relations` — how does this connect

Adds `name`, `kind`, `line`, `relation`, `limit`, `max_tokens`.

```json
{ "name": "relations",
  "arguments": { "scope": "lang/treesitter", "name": "Outranks",
                 "relation": "called-by" } }
```

Text:

```text
called-by Outranks — 1 site

lang/treesitter/outline.go:123:6  in Engine.declarations
  if Outranks(kind, out[seen].Kind) {

resolved, total coverage.
```

Structured:

```json
{
  "scope": { "language": "go", "unit": "lang/treesitter" },
  "of": "Outranks",
  "relation": "called-by",
  "items": [
    { "name": "declarations", "kind": "method", "in": "Engine",
      "path": "lang/treesitter/outline.go", "line": 123, "column": 6,
      "via": "if Outranks(kind, out[seen].Kind) {" }
  ],
  "provenance": { "engine": "gopls", "fidelity": "resolved",
                  "completeness": "total", "supportsNegativeClaim": true }
}
```

`via` is the line the edge was found on. A caller asking who calls this
wants to see the call; fetching each one is a turn per caller. `column`
counts bytes from one, and tells apart two sites on one line, such as
the opening and the closing tag of a JSX element.

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
      "fix": [ { "path": "treesitter/metadata.go", "line": 227,
                 "now": "for part := range strings.FieldsSeq(unquoted) {" } ] }
  ],
  "provenance": { "engine": "golangci-lint", "fidelity": "resolved",
                  "completeness": "total", "supportsNegativeClaim": true }
}
```

An issue with one obvious fix states the text that the fix writes, with
its file and its line. No tool takes a fix, so a caller applies it with
its own edit.

An engine declines a request for a suite that it does not run, because
an answer without that suite states nothing about it. The reason of the
decline lists the suites that the engine runs. A request that every
engine declines is `unsupported`, with the reason of each engine.

### `capabilities` — what can you answer

Takes `language` alone.

```json
{ "name": "capabilities", "arguments": {} }
```

Text:

```text
csharp     treesitter/csharp          syntactic parse   outline search relate plan check index
csharp     csharp-ls                  resolved  session resolve relate plan format check verify
           lsp: csharp-ls: csharp-ls is not on PATH
go         treesitter/go              syntactic parse   outline search relate plan check index
go         gopls                      resolved  session resolve relate plan format check verify
go         go/types                   resolved  session resolve relate check verify
python     treesitter/python          syntactic parse   outline search relate plan check index
```

Structured:

```json
{
  "items": [
    { "language": "go", "role": "relate", "engine": "gopls",
      "fidelity": "resolved", "cost": "session", "available": true },
    { "language": "csharp", "role": "resolve", "engine": "csharp-ls",
      "fidelity": "resolved", "cost": "session", "available": false,
      "unavailable": "lsp: csharp-ls: csharp-ls is not on PATH" }
  ]
}
```

An unavailable engine states what would make it available rather than
only that it is not.

### The write tools

Every one but `apply.change` takes `language` and `dry_run`. `dry_run`
defaults to true, and a dry run returns the changes that the tool would
make. Applying is a second call with `dry_run` false. A preview whose gate
passed is guaranteed to apply.

| Tool | Names | Takes |
|---|---|---|
| `rename.symbol` | scope, name, kind, line | `new_name` |
| `move.file` | path | `to` |
| `document.symbol` | scope, name, kind, line | `doc` |
| `extract.function` | path, first_line, last_line | `new_name` |
| `apply.change` | — | `handle` |

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
  "operation": "rename.symbol",
  "target": "Kind",
  "applied": false,
  "items": [
    { "path": "core/sema/kind.go", "sites": 18,
      "changes": [ { "line": 18, "was": "type Kind uint8",
                     "now": "type Category uint8" } ] }
  ],
  "verified": { "gate": "compile", "engine": "gopls", "result": "pass" },
  "provenance": { "engine": "gopls", "fidelity": "resolved",
                  "completeness": "total", "supportsNegativeClaim": true },
  "handle": "7939cc5a281ab6f839d5902ca5c2f355"
}
```

`extract.function` adds the new declaration to its item list.

`apply.change` takes the handle a preview returned, and the changes stay
where they were computed. Its answer states the operation, the target
and the scope of the preview. Sending them back would mean a caller
reproducing several kilobytes exactly for a rename over thirty sites,
and reproducing bytes exactly is the least reliable thing a model does.
A handle is thirty-two characters.

The plan is held for the session and fetched once. A handle that has been
used, or that the service no longer holds, is refused with the
instruction to preview again — which is the call the caller just made, so
the cost of losing one is a turn. What the preview pinned is checked
before anything is written: a file that changed in between describes code
the plan was not computed against, and byte ranges over other bytes
usually still compile.

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
  "operation": "document.symbol",
  "target": "Store",
  "applied": false,
  "items": [
    { "path": "src/store.rs", "sites": 1,
      "changes": [ { "line": 41, "was": "/// Holds things.",
                     "now": "/// Store holds items by name.\n///\n/// It is safe to share." } ] }
  ],
  "verified": { "gate": "parse", "engine": "treesitter/rust", "result": "pass" },
  "provenance": { "engine": "treesitter/rust", "fidelity": "syntactic",
                  "completeness": "total", "supportsNegativeClaim": false },
  "handle": "3b41a0c9e2d84f6aa15c7e90d2b8f613"
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

### E. Send structured content with every result

Declare the output schema of every tool and send `structuredContent` by
default, so every client can validate an answer.

**Why not:** Claude Code hands the model the structured content of a
result that has it. The model then reads JSON of about 1.8 times the
characters of the render, and never reads the render. A client that
validates, filters or passes on an answer starts the server with
`--structured` and loses nothing.

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
- A server started without `--structured` does not declare an output
  schema. A client that checks answers against one has to start the
  server with the flag.
- Four levels rather than three, with different names, so every caller
  that named one has to change.
- `signature` is a field the parser tier has to produce and cannot
  always: a grammar naming no body field yields the whole declaration,
  which makes `signatures` cost what `source` costs for that language.
- Nesting fights the budget. Dropping a parent drops its members, so the
  order has to drop members first, and a deep tree truncates less
  gracefully than a flat list.
- Addressing by name is ambiguous where a language allows two
  declarations to share a name, kind and container. The refusal lists the
  site of each, and the caller asks again with a line, which is a turn.
- The `in` of a Go method needs the parser to read its receiver, a
  pattern of the Go query rather than the rule over spans that serves the
  other languages.

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
