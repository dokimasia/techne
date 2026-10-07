---
name: techne
description: >-
  Use techne's MCP tools in place of grep, find, cat, Read and sed for code: workspace, outline,
  search, resolve, relations and verify to learn what a repository declares, where a name is
  declared, and what calls, references, implements or imports it; rename.symbol, move.file,
  extract.function and document.symbol to change code through a checked preview that
  apply.change writes. Use this skill whenever a techne server is connected and a task finds,
  explains, traces, renames, moves or refactors code in C, C#, Go, Java, JavaScript, Python, Ruby,
  Rust, Scala or TypeScript, even when the user does not name techne. Read it before you conclude
  from a search that something does not exist.
---

# techne

techne is a server of the Model Context Protocol. It returns what a language's parser, language
server or type checker knows about code, so an answer comes from the declarations and the
bindings of the code, not from its text. Each answer states the evidence behind it.

Use techne for every question about declarations and their uses. Text tools match spelling: grep
for `Close` returns every `Close` in the repository and every comment that mentions one, and it
cannot tell you that it found them all. techne returns the declaration that a name binds to and
states whether its list is complete.

## Load the tools

- Claude Code names each tool `mcp__techne__<tool>`, with each dot of the name replaced by `_`:
  `rename.symbol` is `mcp__techne__rename_symbol`.
- A client can list the tools as deferred. In Claude Code, load them with ToolSearch before the
  first call, such as `select:mcp__techne__outline,mcp__techne__search,mcp__techne__relations`.
- `capabilities` lists, for each language and role, the engine that serves it, the strength of
  its evidence, and whether its program can run. Ask it when a tool returns `unsupported`.

## Pick the tool for the question

| Question | Tool | Instead of |
|---|---|---|
| What packages, modules and files does this directory contain? | `workspace` | `ls`, `find`, `tree` |
| What does this file or directory declare? | `outline` | reading the file |
| Where is this name declared? | `search` | `grep` |
| What does the name at this line and column refer to? | `resolve` | guessing from the text |
| What calls, references, implements, embeds or imports this? | `relations` | `grep` for the name |
| Does this code build and pass its checks? | `verify` | running the build or the linter |
| Rename a declaration everywhere | `rename.symbol` | find and replace |
| Move a file and fix its imports | `move.file` | `mv` and hand edits |
| Turn a run of lines into a function | `extract.function` | cutting and pasting |
| Write a doc comment onto a declaration | `document.symbol` | editing the file |

Keep text tools for text that is no declaration: string literals, log messages, comments,
configuration files, and files of a language that techne does not serve.

## Read the evidence before you conclude

The last line of every answer states the evidence:

```text
resolved, total coverage. an empty answer here means there are none.
syntactic, partial coverage. the grammar does not parse src/a.go in full, so a declaration in a part that does not parse is missing.
```

- **Fidelity** states how names were bound. `syntactic` means a parser matched the text, so a
  name matches every declaration of that spelling. `resolved` means a type checker or a language
  server bound each name to its declaration. `indexed` means an index resolved the name without
  a type checker.
- **Coverage** states how much of the scope the engine examined. `partial` comes with a caveat
  that lists what was left out, such as a file larger than 4 MiB, a file that the grammar does
  not parse in full, or a server that was still loading the workspace.
- **An empty answer proves absence only when the line states it**: `an empty answer here means
  there are none`. That needs resolved fidelity and total coverage. Without that sentence, an
  empty answer means that the engine found nothing in what it examined.
- **Caveat notes** follow on the same line. Read each one. A truncated answer states how many
  items it left out, and you can ask again with a higher `limit` or `max_tokens`.

When the coverage is partial and a caveat names files, read those files yourself before you
state that something is absent from them.

## Name a declaration

The tools that act on one declaration (`relations`, `rename.symbol`, `document.symbol`) take
`scope` and `name`:

- `scope` is the file or the directory of the declaration, relative to the workspace root.
- `name` is the name as written. Qualify a member as `Type.Method` when two types declare a
  member of that name.
- When the name still has more than one declaration, the call is refused with each candidate
  and its line. Call again with `line`, counted from one, or with `kind`, such as `method` or
  `struct`.

`resolve` takes a position instead: `line` and `column`, both counted from one, the column in
bytes.

## Search for declarations

- A name matches in this order: the exact name, the name with case folded, a prefix, then a
  substring. `Type.Method` matches the qualified name.
- A text of two or more words searches documentation: a declaration matches when its
  documentation contains every word.
- A text written as a declaration, such as `func NewServer(cfg Config)`, matches the name it
  declares. You can paste a signature from an outline.
- `kind` keeps one kind, `private` adds unexported declarations, `tests` adds test files, and
  `limit` caps the matches.
- `include` adds bindings that the default leaves out: `import`, `parameter`, `local` or `all`.

## Control the size of an answer

- `detail` sets how much each declaration shows: `names`, `summaries`, `signatures`, `docs` or
  `source`. An outline at `signatures` costs about a quarter of the tokens of the file. `docs`
  and `source` cost more than the file, so narrow them with `names`, `kind` or `prefix`.
- `max_tokens` caps an answer, 6000 by default. An answer cut to fit states it in a caveat.
- `relations` returns 50 relations by default. Raise `limit` when you need every site.

## Trace relations

`relations` takes `relation`: `calls`, `called-by`, `implements`, `implemented-by`,
`references`, `referenced-by`, `imports`, `imported-by`, `embeds` or `embedded-by`. Each site
comes with its line of code. Test files are included, because a test that calls a declaration
uses it.

For `imported-by`, name the package or module as an import writes it, such as its import path,
and set `scope` to its directory.

## Change code: preview, then apply

1. Call the write tool. `dry_run` is true by default, so the call returns a preview: the
   rewritten lines of each file, the check that judged the result, and a handle.
2. Read the preview. The check line states which engine judged the change, such as the
   compiler of the language. The check refuses a change whose result has more errors than the code it replaces,
   and lists the errors. A caveat states what the check could not confirm, such as a use that
   the plan leaves as it was.
3. Call `apply.change` with the handle. It writes every file of the change or none of them.

- `rename.symbol` and `move.file` refuse a change unless they found every reference. The reason
  of the refusal states what is missing.
- `apply.change` refuses a handle when a file changed after the preview. Make the preview again.
- Do not edit the files of a change between its preview and its apply.
- After you apply, run `verify` over the scope, or the build of the project, to confirm the
  result.

## Work in another directory

When techne runs with `--trust DIR`, every tool takes `wd`: a directory under a trusted folder.
The call runs in the workspace of that directory, and the paths of the call and of its answer
are relative to it.

techne keeps three such directories open with their language servers. A call in a fourth closes
the one used least recently, and the previews of a closed directory are gone. Apply a preview
before you work in other directories.

## Handle refusals and slow first calls

- When a tool returns `refused`, correct the request. techne refuses a path that does not
  exist, a name with more than one declaration, and a change that adds errors, and the reason
  states what to change.
- When a tool returns `unsupported`, no engine serves that language and role. Use text tools or
  the build of the project instead, and ask `capabilities` for the reason.
- The first call that needs a language server starts it. Most servers take a few seconds, and
  jdtls and metals take tens of seconds. Later calls reuse the running server.
