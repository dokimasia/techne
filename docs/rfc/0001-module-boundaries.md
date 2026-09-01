---
rfc: 0001
title: Module boundaries
author: Roy Klopper
status: Draft
created: 2026-09-01
updated: 2026-09-01
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

# RFC-0001: Module boundaries

## Summary

Split techne into five Go modules in one repository: `core` holds
everything that does not know a language, `lang` holds machinery more
than one language uses, `lang/<x>` holds one language each, `presenter`
holds the MCP transport, and the root module holds the binary.
Dependencies run one way: `lang` and `presenter` import `core`, a
language module imports `lang` and `core`, `core` imports none of them,
and only the root module imports a language module or a presenter.
Adding or removing a language is then a directory plus two lines, and
`core` compiles without a C toolchain.

## Motivation

Grammar bindings are cgo. A tree-sitter binding needs a C compiler to
build at all, and each grammar on top of it is a package of generated C.
In a single module that cost cannot be contained:
`go build ./...` compiles every package in the module, so one grammar
anywhere means the whole module needs a C toolchain, `CGO_ENABLED=0`
stops working, and cross-compiling to a platform with no C
cross-toolchain stops working. Every language techne serves at the
syntactic tier adds one of these.

The second cost is deletion. Support for a language is a declaration,
some queries, sometimes an engine, and a line of dispatch wherever the
system chooses what to run. Spread across one module, removing a language
means finding those references and deciding which of them are shared.
This layout is built so that deleting one directory and one line from
`go.work` removes a language completely, with nothing else in the tree
mentioning it.

The third is the dependency ledger. Go support needs
`golang.org/x/tools`; a language served by a language server needs an LSP
client; each grammar is its own module. Put them in one module and every
consumer's `go.sum` lists all of them, whether or not that consumer ever
asks about that language.

**Why this belongs at the module layer.** All
three costs are about a *dependency set*, and the module is the unit in
which Go declares dependencies. Splitting into packages inside one module
changes none of it: `go.sum` still lists every grammar, `go build ./...`
still compiles every package, and `go get` still resolves the union.
Package boundaries control which package may import which. They do not
control what `go get` downloads or what the linker includes.

## Detailed design

### The module graph

| Directory | Module path | Holds | cgo |
|---|---|---|---|
| `techne-core/` | `go.dokimi.dev/techne/core` | Vocabulary, engine ports, catalog, read and write services, index, tool interface | no |
| `techne-lang/` | `go.dokimi.dev/techne/lang` | Language declaration, engines more than one language uses, conformance suite | in `lang/treesitter` only |
| `techne-lang-go/` | `go.dokimi.dev/techne/lang/go` | Go: queries, a `go/types` engine, planners | via `lang/treesitter` |
| `techne-lang-<x>/` | `go.dokimi.dev/techne/lang/<x>` | One language each | via `lang/treesitter` |
| `techne-presenter/` | `go.dokimi.dev/techne/presenter` | The MCP transport and the loop that drives a tool call | no |
| `.` | `go.dokimi.dev/techne` | `cmd/techne`, composition root, version stamp | inherited |

```mermaid
flowchart BT
    core["core<br/>vocabulary, ports, services"]
    lang["lang<br/>declaration, shared engines"]
    langgo["lang/go"]
    langpy["lang/python"]
    pres["presenter<br/>mcp"]
    root["techne<br/>cmd + composition root"]

    lang -->|"ports, vocabulary"| core
    pres -->|"tool interface"| core
    langgo -->|"Declaration, treesitter, lsp"| lang
    langpy -->|"Declaration, treesitter"| lang
    langgo -.->|"sema, edit, trust"| core
    langpy -.->|"sema, edit, trust"| core
    root -->|"Catalog"| core
    root -->|"Serve"| pres
    root -->|"Register"| langgo
    root -->|"Register"| langpy
```

### The four rules

1. `core` imports nothing else in this repository.
2. `lang` imports `core` and nothing else in this repository.
3. `presenter` imports `core` and nothing else in this repository.
4. `lang/<x>` imports `core` and `lang`, never another `lang/<x>`.
5. Only the root module imports a `lang/<x>` or `presenter`.

Rule 1 keeps `core` free of cgo and of every language ecosystem. Rule 3
keeps the MCP SDK out of `core`, so embedding the services does not pull
a transport in with them. Rule 4 makes a language deletable: nothing but
the root module names it. Rule 5 keeps the graph acyclic, and it is the
reason the binary lives in its own module rather than in `core`.

Enforce them with depguard, from the root `.golangci.yml`. Each module
gets a strict allow-list keyed to its directory, and golangci-lint finds
this config when it runs with a module directory as its working
directory:

```yaml
linters:
  settings:
    depguard:
      rules:
        core:
          files: ["**/techne-core/**"]
          list-mode: strict
          allow: [$gostd, go.dokimi.dev/techne/core]
        lang:
          files: ["**/techne-lang/**"]
          list-mode: strict
          allow:
            - $gostd
            - go.dokimi.dev/techne/core
            - go.dokimi.dev/techne/lang$
            - go.dokimi.dev/techne/lang/treesitter
            - go.dokimi.dev/techne/lang/lsp
            - go.dokimi.dev/techne/lang/command
            - go.dokimi.dev/techne/lang/conformance
        presenter:
          files: ["**/techne-presenter/**"]
          list-mode: strict
          allow:
            - $gostd
            - go.dokimi.dev/techne/core
            - go.dokimi.dev/techne/presenter
        language-module:
          files: ["**/techne-lang-*/**"]
          list-mode: strict
          allow:
            - $gostd
            - go.dokimi.dev/techne/core
            - go.dokimi.dev/techne/lang
```

Two details do the work. `list-mode: strict` denies whatever the list
does not name, so a language module added later is already covered. A
deny-list would have to name each language module, and the rule would
quietly stop covering whichever one somebody forgot to add.

depguard also matches by prefix, so allowing `go.dokimi.dev/techne/lang`
would allow `go.dokimi.dev/techne/lang/go` with it. The `$` suffix asks
for an exact match instead, which is what lets `lang` import its own
engine packages while rule 2 still stops it importing a language module.

None of this can be left to `go.mod`. Inside the workspace `techne-lang`
can import `go.dokimi.dev/techne/lang/go` and build clean with no
`require` line for it, because `go.work` puts every workspace module in
the build list. Only `GOWORK=off` reports it, and that is not how the
tree gets built during development.

### What core holds

```
techne-core/
  doc.go
  source/            Language, Path, Position, Span         where code lives
  sema/              Symbol, ID, Kind, Relation, Unit, Ref   what it means
  trust/             Fidelity, Completeness, Status, Caveat, Provenance
  edit/              Operation, Family, Target, Spec, Plan   how it changes
  diag/              Diagnostic, Severity, Fix               what is wrong with it
  engine/            Role, Cost, the port interfaces, Catalog, Capability
  query/             the read path, strongest engine first
  change/            the write pipeline
  gate/              verification
  workspace/
    index/           Store and Source ports, boot scan, staleness
    files/           filesystem operations, produced as edit.Plan
    project/         which workspace a path belongs to
  tool/              the tool interface: name, summary, derived schemas
```

The five vocabulary packages sit at the top level rather than under a
group of their own, because the module name already says what they are.
`workspace/` keeps its group: without it a reader meets `files` beside
`query` and `change` and has to work out that one of the three is not a
service.

### What presenter holds

```
techne-presenter/
  doc.go             package presenter: Transport, and the drive loop
  mcp/               JSON-RPC over stdio
```

MCP is the only transport. The module root still holds what a transport
needs: enumerate the registry, decode a call, run `tool.Tool.Execute`,
encode the result. `mcp/` holds the wire format and no domain knowledge.
A presenter that knew about individual tools would be N×M pieces of code
for N tools and M transports, and the two would drift.

This is a module rather than a package in `core` because a presenter
needs the MCP SDK, and `core` is the module people embed to get `query`
and `change` as Go APIs. Keeping the transport out means embedding the
services does not pull the SDK in with it.

Splitting the drive loop from `mcp/` while only one transport exists is a
guess that a second one arrives. If none does, the two collapse into one
package, and the dependency argument alone still justifies the module.

### What lang holds

```
techne-lang/
  doc.go             package lang: Declaration, CommentStyle, Register
  treesitter/        the grammar-driven engine                    (cgo)
  lsp/               the language-server engine
  command/           the subprocess engine, for formatters and verifiers
  conformance/       the suite every language module runs
```

A package belongs here when more than one language can use it. A
`go/types` engine is engine-shaped and still belongs in the Go module,
because no other language can use it, and filing it here would put Go's
implementation where a reader asking what techne does about Go would not
look.

`lsp` is built alongside `treesitter` rather than deferred, so the
`Indexed` tier has an implementation as soon as the ports do. A language
whose ecosystem ships a server reaches that tier without anyone writing a
Go engine for it. Three protocol details corrupt silently when they are
got wrong: positions are UTF-16 code units rather than byte offsets,
edits arrive in two shapes of which the older carries no file operations,
and a server answers only about documents it has been told to open.
Solving them in one package solves them for every language.

Only `lang/treesitter` is cgo. Go compiles just the packages a binary
reaches, so a binary registering only language-server-backed languages
never reaches that package and still builds with `CGO_ENABLED=0`. The
module's own `go build ./...` does need a C toolchain, because that
command compiles every package in the module; see Drawbacks.

### The language declaration

The one type that crosses from `lang` into every language module:

```go
// Package lang declares what a language is, independently of which
// engine answers questions about it.
package lang

// Declaration states the facts about a language that hold whichever
// engine serves it. A language module constructs exactly one.
type Declaration struct {
	// Language identifies the language on every request and every answer.
	Language source.Language

	// Extensions are the file suffixes that select this language,
	// each including its leading dot.
	Extensions []string

	// Manifests are the filenames marking a project root for this
	// language, such as "go.mod" or "pyproject.toml".
	Manifests []string

	// Comment describes how documentation comments are written. The
	// document operations need it and no grammar states it.
	Comment CommentStyle

	// IsTest reports whether a path holds tests rather than shipped code.
	IsTest func(path string) bool

	// Namespace maps a file path to the name an import would use for
	// it: a/b/c.py becomes a.b.c.
	Namespace func(path string) string

	// Exported reports whether a declared name is visible outside the
	// unit that declares it.
	Exported func(name string) bool
}

// CommentStyle describes how a documentation comment attaches to a
// declaration.
type CommentStyle struct {
	// Line prefixes each line of a line-comment block, including any
	// trailing space: "// " for Go, "# " for Python.
	Line string

	// Above reports whether the comment sits above the declaration
	// rather than inside it.
	Above bool
}
```

These fields belong to the language rather than to a grammar, because
they hold whichever engine answers. A language module served only by a
language server declares where its test files are without constructing a
parser it never uses.

### Registration

```go
// Register adds a language and its engines to the catalog. It reports
// an error when the declaration is incomplete or when the language is
// already registered.
func Register(c *engine.Catalog, d Declaration, engines ...engine.Engine) error
```

The root module's composition root calls this once per language. Nothing
registers from `init`. Registering from `init` through blank imports
makes the language set a consequence of which packages happen to be
imported: it cannot be varied at run time, and a test cannot build a
catalog holding only Go. Explicit registration makes the language set a
value the caller chooses.

The shipped binary registers every language module in this repository.
Someone who wants fewer writes their own composition root and calls
`Register` for the subset, which costs a `main` of about twenty lines and
is the thing explicit registration exists to allow. No build tag selects
languages, because a build tag would put the set back into the source
rather than into the caller's hands.

### The go.mod files

```
techne-core/go.mod       module go.dokimi.dev/techne/core
                         go 1.27.0
                         (no techne requires)

techne-lang/go.mod       module go.dokimi.dev/techne/lang
                         require go.dokimi.dev/techne/core

techne-lang-go/go.mod    module go.dokimi.dev/techne/lang/go
                         require go.dokimi.dev/techne/core
                         require go.dokimi.dev/techne/lang

techne-presenter/go.mod  module go.dokimi.dev/techne/presenter
                         require go.dokimi.dev/techne/core

go.mod                   module go.dokimi.dev/techne
                         require go.dokimi.dev/techne/core
                         require go.dokimi.dev/techne/presenter
                         require go.dokimi.dev/techne/lang/go
                         require go.dokimi.dev/techne/lang/python
```

`go.work` lists every directory, so local development resolves across the
modules without `replace` directives and without consulting the network.

The Go module `go.dokimi.dev/techne/lang/go` cannot declare
`package go`, because `go` is a keyword. Its root package is
`package golang`, imported as `go.dokimi.dev/techne/lang/go` and referred
to as `golang`. Import path and package name are allowed to differ, and
`goimports` resolves it.

### Directory names and module paths

The directory names do not match the module path suffixes: the module
`go.dokimi.dev/techne/lang/go` lives in `techne-lang-go/`. The go command
derives a nested module's subdirectory from its path suffix, so
resolution of these paths depends on the vanity host mapping each module
path to a repository root rather than on the layout of this tree.
`go.work` makes local development independent of that mapping; `go get`
and `go install` are not.

Releasing is out of scope here. `ergon release` operates per module over
the set named in `.ergon.yaml`, and nothing is published yet.

### Adding a language

Three edits, in one commit:

1. Create `techne-lang-<x>/` with a `go.mod` declaring
   `go.dokimi.dev/techne/lang/<x>`, a `Declaration`, and whatever queries
   or engine it needs.
2. Add `./techne-lang-<x>` to `go.work`.
3. Call `lang.Register` for it in the root module's composition root.

Removing one is the same three in reverse, and nothing else in the tree
names the language.

## Alternatives considered

### A. One module

Everything in `go.dokimi.dev/techne`, languages as packages under a
`lang/` directory.

**Why not:** a grammar package is cgo, and `go build ./...` compiles
every package in a module, so one grammar makes the whole module require
a C toolchain and lose both `CGO_ENABLED=0` and cross-compilation. Every
consumer's `go.sum` lists every grammar and every language ecosystem's
tooling. This stays workable only while no language has a parser.

### B. Three modules, with the services in the root module

`core` for the vocabulary and ports, `lang` for shared engines,
`lang/<x>` per language, and `query`, `change` and `gate` in the root
module alongside the binary.

**Why not:** it puts the read path and the write pipeline in the same
module as `cmd/`, so nothing can depend on them without depending on the
command-line tool, and the root module stops being the thing that only
names which languages ship. The module count is the same as this
proposal; the difference is which module holds the services, and this
placement makes them unembeddable.

### C. A separate contract module for the vocabulary and ports

A sixth module holding the vocabulary and the ports, with `core` and
every language module depending on it, so a port change does not force a
version bump on the service layer.

**Why not:** the coupling it removes is version coupling, and inside one
repository with `go.work` there is none to remove: every module builds
against the working tree. The build-time argument does not
apply either, because Go compiles per package: a language module
importing `core/engine` never compiles `core/query`. The extraction stays
available and is mechanical, since the packages that would move already
import nothing above them. It becomes worth doing when language modules
move to their own repositories, or when someone outside this repository
writes one.

### D. A separate module for the tree-sitter engine

Pull `lang/treesitter` out so the rest of `lang` needs no C toolchain.

**Why not:** the benefit is already there. Go compiles only the packages
a binary reaches, so a binary that never imports `lang/treesitter`
already builds without cgo. What splitting buys is that `lang`'s own
`go build ./...` and `go test ./...` stop needing a C compiler. That
helps contributors and CI, not users. One module for one package is not
worth it yet. It becomes worth it if a language module ships with no
grammar at all and its maintainers cannot build the tree.

### E. One repository per module

Each module in its own git repository, so every module path is a
repository root.

**Why not:** it is what a public plugin ecosystem would eventually need,
and it makes every module path resolve without a vanity mapping. It also
means a port change is N pull requests across N repositories in a
required order, with a release of each between them, at a point when the
ports are not settled. One repository keeps that change to one commit.

## Drawbacks

- Five `go.mod` files, and one more for every language added. Each
  needs `go mod tidy` and each pins its own dependency versions. The
  pre-merge gate already iterates the directories `go.work` lists, so
  this is a cost in review attention rather than in tooling.
- Adding a language touches three places rather than one: the new
  directory, `go.work`, and the composition root. Two of those are one
  line each.
- A port signature change is a cross-module edit: `core`, `lang`, and
  every language module, in one commit. Inside one repository that
  compiles as a unit; the versioning discipline is manual, because
  nothing stops a released `lang/go` from pinning a `core` whose ports
  have moved.
- `lang`'s own `go build ./...` and `go test ./...` need a C toolchain
  even though only one of its four packages is cgo, because those
  commands cover every package in the module. A contributor changing
  `lang/lsp` still needs a C compiler for `lang/treesitter`.
- The shipped binary registers every language module, so it is cgo, it
  links every grammar, and its size grows with each language added.
  Getting a smaller one means writing a `main`, which is a source change
  rather than a build flag.
- Splitting presenters out puts the transports one module away from the
  tool interface they consume, so a change to `tool.Tool` is a two-module
  edit even though only one thing changed.
- Directory names and module paths differ, so a reader cannot derive one
  from the other and `go get` depends on the vanity host being correct.
- A language module's path sits under `lang`'s, so a prefix test cannot
  separate the two. The depguard rules need the `$` exact-match suffix
  for that reason, and so will anything else that tests the boundary by
  import path.
- A strict allow-list names every third-party dependency a module may
  have, so adding one is an edit to `.golangci.yml` as well as to
  `go.mod`. That makes each new dependency a decision somebody signs off,
  and it makes a legitimate addition take two steps instead of one.
- Coverage thresholds, commit scopes and the CI matrix all grow a row per
  module, and each new language adds one to each.

## Unresolved and future work

This proposal fixes which module each thing lives in and nothing about
what it contains. The vocabulary types, the port set and engine selection
are settled in RFC-0002. The agent-facing tools, their schemas and the
size budget are settled in RFC-0003. The operation catalogue and the
write pipeline are settled in RFC-0004.

Extraction of a contract module, and the move to one repository per
module, are both noted in Alternatives as things that become worth doing
under conditions that do not hold yet. Neither is proposed.

## References

| What | Where |
|---|---|
| Module subdirectories and version tag prefixes | https://go.dev/ref/mod#vcs-dir |
| Vanity import paths and the go-import meta tag | https://go.dev/ref/mod#vcs-find |
| cgo and cross-compilation constraints | https://pkg.go.dev/cmd/cgo |
