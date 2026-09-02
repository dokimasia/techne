# Package layout

Where a file goes, and what each package may import. The module
boundaries and the rules between modules are settled in RFC-0001; this
document covers the packages inside them.

## Two rules place every file

**Vocabulary is split by layer of meaning, never pooled.** The types
every other package names live in five packages rather than one. Each has
a single reason to change: `trust` gaining a fidelity tier has nothing to
do with `edit` gaining an operation.

**Machinery is placed by reuse, not by role.** A package belongs in
`lang` when more than one language can use it, and in a language module
when only one can. A type-checker-backed engine is engine-shaped and
still belongs to its language, because filing by role scatters a language
across the tree and puts its implementation where a reader asking what
techne does about that language would not look.

## Dependency position

Every package declares its position in its `doc.go` under a
`# Dependency position` heading, naming what it imports and what may
import it. The tables below are the same information collected in one
place; the `doc.go` copy is the one a reader meets first and the one that
has to stay true.

Positions run 0 to 2 and are scoped to a module. A package may import
anything at a lower position and nothing at a higher one. Packages at the
same position may import each other only where the table says so. Across
modules the seven rules in RFC-0001 apply instead, and a linter enforces
them.

## techne-core

Everything an engine implements, and the vocabulary those ports speak.
Nothing that consumes a port: the services are `techne-service` and the
surface an agent calls is `techne-tool`.

### Position 0: vocabulary

| Package | Holds | Imports |
|---|---|---|
| `source` | Where code lives: `Language`, `Path`, `Position`, `Span` | stdlib |
| `sema` | What it means: `Symbol`, `ID`, `Kind`, `Relation`, `RelationKind`, `Unit`, `Visibility`, `Annotation` | `source` |
| `trust` | How sure we are: `Fidelity`, `Completeness`, `Status`, `Caveat`, `Provenance` | `source` |
| `edit` | How it changes: `Operation`, `Family`, `Target`, `Spec`, `Change`, `Plan`, `Policy`, `Request`, `Outcome` | `source`, `sema`, `trust`, `diag` |
| `diag` | What is wrong with it: `Diagnostic`, `Severity` | `source` |

Belongs here: a type a port signature names. Does not: anything that
*does* something. These packages depend on the standard library and on
each other. If a type here needs a service, it is not vocabulary.

`edit` holds one thing that is not a type: `Policy.Admit`, the rule
deciding whether a plan may be applied. It is a pure decision over the
vocabulary with no I/O, and RFC-0004 requires it be reviewable in one
place rather than repeated per language.

### Position 1: the plug-in contract

| Package | Holds | Imports |
|---|---|---|
| `engine` | `Role`, `Cost`, `Available`, the port interfaces, `Catalog`, `Capability`, `Router`, and the one place an engine is selected and asked | position 0 |

Selection lives here rather than with the services that do it. The rule
deciding which language owns a path was written twice while it lived in
the services, once in the read path and once in the write path; a read
and a write that disagree about who owns a file plan a change with one
engine and gate it with another. `Languages`, `Ask`, `AskEach` and
`AskAny` are that rule and the loop around it, and `Publish` is where a
tier is stamped.

Roles are separate interfaces so an engine declines a capability by not
having the method rather than by returning an error. Selection is by type
assertion, so every engine package asserts the set it claims at compile
time; without that, a signature drifting from its port removes a
capability silently instead of breaking the build.

Engines never construct their own provenance. They declare a fixed
fidelity and return per-answer caveats, and the services stamp the rest.
An engine therefore cannot overstate its evidence, and a service cannot
discard a limit only the engine knows about.

## techne-service

The paths that drive the ports, and the workspace they run over. It
imports `core` and nothing else in the repository.

| Package | Holds | Imports |
|---|---|---|
| `query` | the read path: one dispatcher per role, strongest engine first | `core/engine`, core position 0 |
| `change` | the write pipeline: validate, plan, seal, admit, project, gate, lock, apply | `core/engine`, `workspace/files`, core position 0 |
| `gate` | verification: an in-process verifier, or the language's declared argv | `core/engine`, `core/diag` |
| `workspace/index` | `Store` and `Source` ports, boot scan, staleness | `core/engine`, `core/sema`, `core/source` |
| `workspace/files` | reading and writing one rooted directory | `core/source` |
| `workspace/project` | which workspace a path belongs to | `core/source` |

`gate` sits below `change` because `change` runs it, not beside it.
`workspace/files` is the only package here that touches disk, and the
write path reaches it through a port declared in `change`, so a second
path around the policy check would have to be written on purpose.

`workspace/` keeps its group because without it a reader meets `files`
beside `query` and `change` and has to work out that one of the three is
not a service.

## techne-tool

The surface an agent calls. It imports `core` and nothing else in the
repository.

| Package | Holds | Imports |
|---|---|---|
| `tool` | the tool interface, the registry, the answer shape, the size budget, and the tools | `core/engine`, core position 0 |

A tool is given a service through an interface `tool` declares itself,
one per question asked, so this module names no service. A tool that
could name a service would be tested against one, and what a tool does
with an answer would stop being separable from how the answer was
assembled.

A tool declares its name, its summary and the schemas derived from its Go
types; carrying it over a wire is the presenter module's job.

## techne-presenter

| Package | Holds | Imports |
|---|---|---|
| `presenter` | `Transport`, and the loop that drives a tool call | `tool` |
| `presenter/mcp` | JSON-RPC over stdio | `presenter`, `tool` |

MCP is the only transport. A presenter carries no domain knowledge: it
translates between one transport and `tool.Tool.Execute`, and nothing
else. A presenter that knew about individual tools would be N×M pieces of
code for N tools and M transports, and the two would drift.

The module exists so that nothing below it carries the MCP SDK. Embedding
`query` and `change` as Go APIs then costs nothing extra, and a second
transport, if one is ever wanted, is a package here rather than a change
to the module holding the tools.

## techne-lang

| Package | Holds | Imports |
|---|---|---|
| `lang` | `Declaration`, `CommentStyle`, `Register` | `core/source`, `core/engine` |
| `lang/treesitter` | the grammar-driven engine (cgo) | `lang`, `core/engine`, position 0 |
| `lang/lsp` | the language-server engine | `lang`, `core/engine`, position 0 |
| `lang/command` | the subprocess engine, for formatters and verifiers | `lang`, `core/engine`, `core/diag` |
| `lang/conformance` | the suite every language module runs | `lang`, `core/engine` |

Belongs here: machinery more than one language can use. Does not:
anything only one language can use.

`lang/treesitter` is the only cgo package in the module. A binary that
registers only language-server-backed languages never reaches it and
still builds with `CGO_ENABLED=0`; the module's own `go build ./...` does
not, because that command compiles every package.

## A language module

Two shapes, depending on what the language needs.

**Declaration only.** A language served entirely by the shared engines is
a `Declaration` and its query files. No engine code is written for it.

```
techne-lang-python/
  doc.go                package python: Declaration and registration
  queries/tags.scm
  queries/imports.scm
```

**Declaration and an engine.** A language with its own type checker adds
an engine implementing the same ports at a higher fidelity. Both
register, the catalog orders them, and the dispatcher tries the next one
when the first cannot answer. That includes the case where the workspace
does not compile, which is when the parser is the only engine left that
can say anything.

```
techne-lang-go/
  doc.go                package golang: Declaration and registration
  queries/tags.scm
  queries/imports.scm
  gotypes/              the go/types engine: read ports at Resolved
  refactor/             the planners, one file per operation family
  module/               go.work and module loading, with a load cache
```

The root package cannot be called `go`, because `go` is a keyword. It is
`package golang`, imported as `go.dokimi.dev/techne/lang/go`.

Deleting the directory and its line in `go.work` removes the language.

## The root module

| Package | Holds | Imports |
|---|---|---|
| `internal/app` | the composition root: build the catalog, register every language, offer the tools, choose a presenter | `core`, `service`, `tool`, `presenter`, every language module |
| `internal/version` | build metadata stamped at link time | stdlib |
| `cmd/techne` | a shim that forwards an exit code | `internal/app` |

It holds no logic. Opening a directory, applying an edit and rendering
an answer all live in the modules whose job they are; what is left here
is a list of what this binary offers and the wiring that joins it.

This is the only module that names a language, and it names all of them.
Registration is an explicit call, never an `init` with a blank import:
the set of languages has to be a value the caller chooses, so that a test
can build a catalog holding one language and someone who wants a smaller
binary can write their own `main` over the same `Register` calls.

## File conventions inside a package

- Name a file after the unit or family it holds, never after a technical
  layer. No `types.go`, `interfaces.go`, `helpers.go`.
- A banner comment grouping declarations inside a file is a file boundary
  that was not drawn. Those groups are the files.
- `doc.go` carries the package comment: a `Package <name> <verb phrase>`
  first line, a `# Heading` per concern, doc links to the package's own
  symbols, and a `# Dependency position` section naming what it imports.
- Tests are black-box: the test package is `<pkg>_test`.
- One test file per production file, paired in both directions. A file
  holding only type declarations still gets its twin, covering the zero
  values and the contracts its docblocks state.
- One `Test<Unit>` function per production file. Everything below it is
  `t.Run`, and the path reads `Test<Unit>/<Method>/<case>`. Error cases
  are cases under their method, not a group of their own.
- `t.Parallel()` at every level.
- Every exported declaration carries a docblock stating its contract:
  what it returns, what the zero value means, what it does on failure.
  State a convention shared across many types once, under a `doc.go`
  heading, rather than repeating it on each.
