<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: MIT
-->

# Contributing to techne

## Setup

You need Go 1.27.2 or later, a C compiler, ergon and pre-commit. The grammars are C, and techne
builds them with cgo. `go.work` and every `go.mod` pin that Go version, and CI reads it from
`go.work`.

Install ergon with Homebrew, or with Go from source:

```sh
brew install --cask dokimasia/tap/ergon
go install go.dokimi.dev/ergon/cmd/ergon@latest
```

The targets of the Makefile run their tools through `ergon tool run`. On its first run, ergon
installs each tool at the version that `.ergon.yaml` pins. Install the git hooks once:

```sh
pre-commit install
```

The hooks run `make lint` and `make test` before each commit, `make check` before each push, and
commitlint on each commit message.

## Before you open a PR

```sh
make check
```

`make check` runs the gate of Go in every module of `go.work`. The gate runs golangci-lint with
its format check and ergon-go-vet, the tests, the tests under the race detector, and govulncheck.
CI runs the same gate on Linux, macOS and Windows. CI also checks the commit messages, the
changesets, the license headers, the Markdown files and the managed files.

While you work, run the narrower targets:

```sh
make fmt            # the formatters of .golangci.yml
make lint-go        # golangci-lint, its format check and ergon-go-vet
make test           # go test, per module
make help           # every target
ergon license fix   # the SPDX header of every file
```

The Makefile runs golangci-lint once per module, from the directory of the module. Every module
reads the one `.golangci.yml` at the repository root.

## Managed files

ergon writes the Makefile, `.golangci.yml`, the workflows and every other file whose first line
starts with `Managed by ergon init`. Do not edit these files. Put a setting of techne into the
file of the same path under `.ergon/local/`, run `ergon init sync`, and commit both files. The
Baseline job of CI runs `ergon init check`, which fails when a managed file differs from the file
that ergon writes.

## Changesets

A pull request that changes a module adds a changeset to `.changeset/`. The changeset contains
each module that the change releases with its bump, and a summary. The summary becomes the entry
in the changelog of each module:

```sh
ergon release add --bump go.dokimi.dev/techne/lang=minor -m "Add the rename of a module."
```

A change that releases nothing, such as a change to the tests or the documentation alone, adds a
changeset without modules with `ergon release add --empty`. The Changeset job of CI runs `ergon
release status`, which fails a pull request that changes a module without a changeset for it.

## Modules

Each component is its own Go module, and `go.work` lists them:

| Module | What it contains |
|---|---|
| `techne-core` | The vocabulary and the ports that every engine implements |
| `techne-lang` | The shared language layer: the tree-sitter engine, the language-server engine and the conformance suite |
| `techne-lang-<language>` | One module per language, and `techne-lang-mock`, a language without a grammar or a server |
| `techne-service` | The read and write services that the tools call |
| `techne-tool` | The tools that an agent calls |
| `techne-presenter` | The server of the Model Context Protocol |
| The root module | The command `techne`, its composition in `internal/app`, and the corpus harness in `test/corpus` |

[The package layout](docs/architecture/01-package-layout.md) states which module each package
belongs to and what each module may import.

## Commits

Write Conventional Commits. The commit-msg hook and the Commits job of CI run commitlint with the
rules of `.commitlint.yaml`. commitlint rejects a type outside this list: `feat`, `fix`, `docs`,
`refactor`, `test`, `ci`, `chore`, `perf`, `build` and `revert`.

Name the scope after the module without its `techne-` or `techne-lang-` prefix, such as `tool`,
`lsp` or `go`. Leave the scope off for a change of the whole repository and for documentation
alone. Keep the subject to 72 bytes and each body line to 100 bytes. State what changed and why.
The diff shows how.

## Tests

Put test files beside what they test.

Each language module runs the conformance suite of `techne-lang/conformance` over a fixture of
its language. Add a case to the suite, and every language runs it.

`make corpus` drives the binary over the repositories of `test/corpus/corpus.json` and checks
each rename, move and extraction with the build of the repository. The first run takes hours.
Set `ONLY` to the names or the languages of the repositories to run fewer of them. To drive the
tools without a language server, set `TECHNE_MOCK=1`, as the README states.

## Documentation

Write a Go docblock for the person who is about to call the package. State what it is and what
it does, in the present tense. Do not mention repository files, because godoc renders where those
paths mean nothing. Do not write status, roadmap or history into a docblock.

[docs/README.md](docs/README.md) lists the architecture documents, the RFCs, the decision records
and the roadmap. A change to what a tool takes or returns needs an RFC before any code.

## Vocabulary

Write plain English in code comments and documents alike. Keep a term of art that a reader can
look up, such as seam, provenance, manifest or drift, and the ordinary word for a thing. Drop a
word that is an image in place of a plain verb. Functions return, and booleans report whether.

Do not write these words: `rung` and `ladder` (write check, step, level or scale), `answers` as a
verb for returns, `fires`, `lands`, `rides`, `travels`, `seat`, `floor`, `bill`, `ritual`,
`story`, `journey`, `stranger`, `world`, `inhabited`, `first-class`, `pays`, `priced`, `earns`,
`heals` and `blind to`.

## Security

Do not open an issue for a vulnerability. Read the [security policy](SECURITY.md).
