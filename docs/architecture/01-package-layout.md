# Package layout

This document lists the packages of the 17 modules of techne, what each package contains, and
what it imports. The depguard rules of `.golangci.yml` enforce the imports between modules.
Each package states its imports in its `doc.go` under `# Dependency position`. The doc tests of
core, the presenter and the root module check those imports with `go list`.

## Placement

- **Vocabulary is split by meaning.** `source`, `sema`, `trust`, `diag` and `edit` declare the
  types that the other packages name. A new fidelity tier changes `trust` alone, and a new
  operation changes `edit` alone.
- **Code that two or more languages use is in `lang`.** The code of one language is in the
  module of that language. The Go type checker serves Go alone and is in the Go module.

## Modules

| Module | Packages | Imports of techne |
|---|---|---|
| `techne-core` | `core`, `source`, `sema`, `trust`, `diag`, `edit`, `engine`, `internal/wire` | none |
| `techne-lang` | `lang`, `lang/treesitter`, `lang/lsp`, `lang/lsp/lsptest`, `lang/engines`, `lang/conformance` | core |
| a language module | one package, and `lang/go/checker` in `techne-lang-go` | core and the shared packages of `lang` |
| `techne-lang-mock` | `lang/mock` | core and `lang` |
| `techne-service` | `service`, `service/query`, `service/change`, `service/workspace/files` | core |
| `techne-tool` | `tool` | core |
| `techne-presenter` | `presenter` | `tool` |
| the root module | `internal/app`, `internal/version`, `cmd/techne`, `test/corpus` | every module |

The root packages `core` and `service` declare nothing. Their package comments list the
packages of their module.

Only the root module imports a language module, a service or the presenter. The tool module
declares an interface for each service that it calls and does not import `techne-service`.

## techne-core

Core declares the ports that an engine implements and the vocabulary of the ports. It does not
call a port. The services are in `techne-service`, and the tools are in `techne-tool`.

| Package | Contains | Imports |
|---|---|---|
| `source` | where code is: `Language`, `Path`, `Position`, `Span` | nothing |
| `sema` | what code declares: `Symbol`, `ID`, `Kind`, `Relation`, `RelationKind`, `Visibility`, `Unit`, `Annotation` | `source`, `internal/wire` |
| `trust` | the evidence of an answer: `Fidelity`, `Completeness`, `Status`, `Caveat`, `Provenance` | `source`, `internal/wire` |
| `diag` | the problems that a verifier reports: `Diagnostic`, `Severity` | `source`, `internal/wire` |
| `edit` | how code changes: `Operation`, `Spec`, `Target`, `Plan`, `Change`, `Policy`, `Request`, `Outcome` | `source`, `sema`, `trust`, `diag` |
| `engine` | the ports of an engine, `Role`, `Cost`, `Catalog`, `Result`, `Answer`, and the selection of the engines of a request | `source`, `sema`, `trust`, `edit`, `internal/wire` |
| `internal/wire` | the name tables and the JSON methods of the enums of core | the standard library |

The vocabulary packages contain types and pure functions. `edit.Policy.Admit` is one of them:
it decides whether a plan may be applied.

Each role of `engine` has a port interface of its own, such as `Outliner` or `Planner`. The
catalogue selects the engines of a role by type assertion. An engine that implements a port can
still decline its role by declaring the fidelity `trust.None` for it.

`Catalog.For` orders the engines of a language and a role by fidelity, then by cost. `Ask`
tries them in that order. The read path calls `AskEach`, which asks every language of a
request. The write path calls `AskAny`, which returns the first answer that is not skipped.

An engine declares a fixed fidelity and cost per role. It returns a `Result` of items, coverage
and caveats. `Publish` turns the result into an `Answer` with the tier that the engine declared.
An engine cannot overstate its evidence.

## techne-lang

| Package | Contains | Imports |
|---|---|---|
| `lang` | `Declaration`, `Registry`, `Workspace`, the walk of the files of a scope, the ignore rules, `Indentation` and `Lowered` | core |
| `lang/treesitter` | the syntactic engine, which serves every language with a tree-sitter grammar | `lang`, core, the tree-sitter binding |
| `lang/lsp` | the engine of a language server | `lang`, core, `go.lsp.dev` |
| `lang/lsp/lsptest` | a scripted language server for the tests of `lang/lsp` | `lang/lsp`, `lang`, core |
| `lang/engines` | the engines of one language, from its declaration, its grammar and its server | `lang/treesitter`, `lang/lsp`, `lang`, core |
| `lang/conformance` | the suite that every language module runs | `lang/treesitter`, `lang/lsp`, `lang`, core, `go.dokimi.dev/assert` |

The grammars are C, and techne builds with cgo. Each language module imports its own grammar.
`lang/treesitter` imports only the binding, `github.com/tree-sitter/go-tree-sitter`.

## A language module

Each of the ten language modules has one package. The package declares its language and
registers its engines:

```text
techne-lang-python/
  doc.go                the package comment
  language.go           Language, Declaration, Grammar, Server and Register
  conventions.go        the rules of the language for visibility and test files
  queries/extends.scm   the tags query that the module compiles
  queries/upstream.scm  the tags query of the grammar, to compare with its next release
```

`Register` builds the tree-sitter engine and the engine of the language server with
`lang/engines`. It adds both to the catalogue. The Go module adds a type checker:

```text
techne-lang-go/
  doc.go, language.go, conventions.go, queries/
  checker/              resolve, relate, verify and check for Go, from go/packages in this process
```

The import path of the Go module is `go.dokimi.dev/techne/lang/go`. Its package is `golang`,
because `go` is a keyword.

No language module imports another. Removing a language takes its directory and the four edits
in the root module that the package comment of `internal/app` lists.

## techne-lang-mock

`lang/mock` declares a line-oriented language without a grammar or a server. Its engine reads
the files itself. It serves outline, search, resolve, relate, plan, check and verify at the tier
that its registration sets. Tests and the root module use it to drive the tools at tiers that no
language module claims for every role.

`mock.Registering(name)` returns the registration of a mock language that claims the extension
of that name. `mock.At` sets its tier, and `mock.Covering` sets its completeness. The root
module registers the mock languages that the variable `TECHNE_MOCK` specifies, and none when it
is empty.

## techne-service

The services call the ports of the engines. The module imports only core from techne.

| Package | Contains | Imports |
|---|---|---|
| `service/query` | the read path: a method per read role, which asks the engines of each language of the request | core |
| `service/change` | the write path: it checks a request against its spec, plans it, seals what the plan reads, admits the plan, gates the projection and writes it | core |
| `service/workspace/files` | the directory of a workspace, which the engines read and the write path changes, and the lock of its writes | `core/source`, and `golang.org/x/sys/windows` on Windows |

`service/change` declares the port `Files`, and the root module passes it a `files.Root`. The
write path changes the disk only through that port.

## techne-tool

| Package | Contains | Imports |
|---|---|---|
| `tool` | the `Tool` interface, the `Registry`, the answers, the token budget and the eleven tools | core, `github.com/google/jsonschema-go` |

Each tool declares its name and its description. jsonschema-go derives the schemas of its input
and its output from Go types.

## techne-presenter

| Package | Contains | Imports |
|---|---|---|
| `presenter` | the MCP server of the tools of a registry, over standard input and output | `tool`, `github.com/modelcontextprotocol/go-sdk` |

The presenter serves every tool of its registry and sends each payload as the tool encoded it.
`test/corpus` uses the client of the same SDK.

## The root module

| Package | Contains | Imports |
|---|---|---|
| `internal/app` | the command line, and the composition of the languages, the mock languages, the services, the tools and the presenter | every module |
| `internal/version` | the version of the build, which the release stamps with the linker | nothing |
| `cmd/techne` | the command | `internal/app`, `internal/version` |
| `test/corpus` | the harness that drives the binary over large repositories. Its test `TestCorpus` builds only with the tag `corpus` | `core/source`, `tool`, the MCP SDK |

`internal/app` is the only production package that imports a language module. `Build`
registers each language by an explicit call. The set of languages is a value of `internal/app`,
not of the imports of the binary.

## File conventions

- Name a file after the unit or the family that it declares. Never name it after a technical
  layer, such as `types.go` or `helpers.go`.
- Split a file where a banner comment would group declarations.
- Put the package comment in `doc.go`:
  - The first line reads `Package <name> <verb phrase>`.
  - Each concern has a `# Heading`, and the text links to the symbols of the package.
  - The last section, `# Dependency position`, lists what the package imports.
- Test black-box: the test package is `<pkg>_test`.
- Pair each production file with a test file of its name, and each test file with a production
  file. These files are the exceptions:
  - `lock_test.go` of `service/workspace/files` tests the lock variant that the platform
    builds, so the variants have no test file of their own.
  - `corpus_test.go` is the harness that `make corpus` runs, and has no production file.
- Give each test file one `Test<Unit>` function. Its subtests read `Test<Unit>/<Method>/<case>`.
- Call `t.Parallel()` at every level of a test.
- Document every exported declaration with its contract. The revive rule `exported` checks that
  each has a comment.
