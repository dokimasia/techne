# Contributing to techne

## Setup

You need Go 1.27.0 or later and a C compiler. The grammars are C, and techne builds them with
cgo. Every `go.mod` and `go.work` pins that Go version, and CI reads it from there.

```sh
make bootstrap    # installs gofumpt, gci, golangci-lint, govulncheck, go-license
make install      # downloads and verifies dependencies
pre-commit install --hook-type pre-commit --hook-type commit-msg
```

## Before you open a PR

```sh
make check
```

That runs `ergon check`, which verifies the modules, lints them, runs the tests and checks the
coverage of each module. CI runs the same command, and so does the pre-commit hook.

While you work, run the narrower targets:

```sh
make fmt          # SPDX headers, the Go formatters and markdownlint
make lint-go      # golangci-lint only
make test         # go test, per module
make build        # compile every module
make help         # every target
```

ergon runs golangci-lint once per module, from each module directory. Every module reads the one
`.golangci.yml` at the repository root.

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

Write Conventional Commits. The commit-msg hook runs `ergon check commit-msg`, which rejects a
type outside this list: `feat`, `fix`, `docs`, `refactor`, `test`, `ci`, `chore`, `perf`, `build`
and `revert`.

Name the scope after the module without its `techne-` or `techne-lang-` prefix, such as `tool`,
`lsp` or `go`. Leave the scope off for a change of the whole repository and for documentation
alone. Keep the subject to 72 bytes and each body line to 100 bytes. State what changed and why.
The diff shows how.

## Tests and coverage

Put test files beside what they test. `make test` runs each test three times, so a test that
passes in one order only fails here before it fails in CI.

Each module has a floor of line coverage under `checks.coverage.packages` in `.ergon.yaml`, and
`make check` fails a module below its floor.

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
