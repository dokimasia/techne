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

Positions run 0 to 3. A package may import anything at a lower position
and nothing at a higher one. Packages at the same position may import
each other only where the table says so.

## techne-core

### Position 0: vocabulary

| Package | Holds | Imports |
|---|---|---|
| `source` | Where code lives: `Language`, `Path`, `Position`, `Span` | stdlib |
| `sema` | What it means: `Symbol`, `ID`, `Kind`, `Relation`, `RelationKind`, `Unit`, `Ref` | `source` |
| `trust` | How sure we are: `Fidelity`, `Status`, `Caveat`, `Provenance` | `source` |
| `edit` | How it changes: `Operation`, `Family`, `Target`, `Spec`, `Plan`, `Change` | `source`, `sema` |
| `diag` | What is wrong with it: `Diagnostic`, `Severity`, `Fix` | `source` |

Belongs here: a type more than one package needs to name. Does not:
anything that *does* something. These packages depend on the standard
library and on each other. If a type here needs a service, it is not
vocabulary.

### Position 1: the plug-in contract

| Package | Holds | Imports |
|---|---|---|
| `engine` | `Role`, `Cost`, `Availability`, the port interfaces, `Catalog`, `Capability` | position 0 |

Roles are separate interfaces so an engine declines a capability by not
having the method rather than by returning an error. Selection is by type
assertion, so every engine package asserts the set it claims at compile
time; without that, a signature drifting from its port removes a
capability silently instead of breaking the build.

Engines never construct their own provenance. They declare a fixed
fidelity and return per-answer caveats, and the services stamp the rest.
An engine therefore cannot overstate its evidence, and a service cannot
discard a limit only the engine knows about.

### Position 2: services and the environment

| Package | Holds | Imports |
|---|---|---|
| `query` | the read path: one dispatcher per role, strongest engine first | `engine`, position 0 |
| `change` | the write pipeline: validate, plan, seal, admit, lock, apply, format, gate | `engine`, `gate`, `workspace/files`, position 0 |
| `gate` | verification: an in-process verifier, or the language's declared argv | `engine`, `diag` |
| `workspace/index` | `Store` and `Source` ports, boot scan, staleness | `engine`, `sema`, `source` |
| `workspace/files` | filesystem operations, produced as `edit.Plan` so they cross the one write path | `edit`, `source` |
| `workspace/project` | which workspace a path belongs to | `source` |

`gate` sits below `change` because `change` runs it, not beside it.
`workspace/files` produces plans rather than writing, so there is one
place that touches disk and no second path around the policy check.

`workspace/` keeps its group because without it a reader meets `files`
beside `query` and `change` and has to work out that one of the three is
not a service.

### Position 3: the tool interface

| Package | Holds | Imports |
|---|---|---|
| `tool` | the tool interface, the registry, and the tool set over the services | positions 0 to 2 |

This is where `core` stops. A tool declares its name, its summary and the
schemas derived from its Go types; carrying it over a wire is the
presenter module's job.

## techne-presenter

| Package | Holds | Imports |
|---|---|---|
| `presenter` | `Transport`, and the loop that drives a tool call | `core/tool` |
| `presenter/mcp` | JSON-RPC over stdio | `presenter`, `core/tool` |

MCP is the only transport. A presenter carries no domain knowledge: it
translates between one transport and `tool.Tool.Execute`, and nothing
else. A presenter that knew about individual tools would be N×M pieces of
code for N tools and M transports, and the two would drift.

The module exists so that `core` does not carry the MCP SDK. Embedding
`query` and `change` as Go APIs then costs nothing extra, and a second
transport, if one is ever wanted, is a package here rather than a change
to the module holding the services.

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
| `internal/app` | the composition root: build the catalog, register every language, choose a presenter | `core/tool`, `presenter/...`, every language module |
| `internal/version` | build metadata stamped at link time | stdlib |
| `cmd/techne` | a shim that forwards an exit code | `internal/app` |

This is the only module that names a language, and it names all of them.
Registration is an explicit call, never an `init` with a blank import:
the set of languages has to be a value the caller chooses, so that a test
can build a catalog holding one language and someone who wants a smaller
binary can write their own `main` over the same `Register` calls.

## File conventions inside a package

- One file per family of related declarations, with a banner comment at
  the top naming the family. The banner is the file boundary: when a
  second family appears in a file, it moves to its own.
- `doc.go` holds the package comment and the `# Dependency position`
  heading. It declares no types.
- Tests are black-box by default: `foo.go` is tested by `foo_test.go` in
  package `<pkg>_test`. A white-box test file exists only where the test
  needs an unexported symbol, and says why at the top.
- Subtests are named `Test<Unit>/<Method>/<case>`, so a failure names the
  case without reading the test body.
- Every exported declaration carries a doc comment stating its contract:
  what it returns, what it does on a zero value, and what it does on
  cancellation.
