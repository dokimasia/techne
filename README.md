<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: MIT
-->

# techne

[![CI](https://github.com/dokimasia/techne/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/dokimasia/techne/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/dokimasia/techne)](https://github.com/dokimasia/techne/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/go.dokimi.dev/techne.svg)](https://pkg.go.dev/go.dokimi.dev/techne)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

techne is a server of the Model Context Protocol (MCP) that gives an AI coding agent the code
navigation and the refactorings of an IDE in ten languages: C, C#, Go, Java, JavaScript, Python,
Ruby, Rust, Scala and TypeScript.

techne parses code with the tree-sitter grammar of each language, which is built into the binary.
It binds names through the language server of the language, and it checks each change before it
writes a file. Every answer states its evidence, so an agent can tell an empty list that proves
absence from an empty list that does not.

## Install

### macOS and Linux

Install techne with Homebrew:

```sh
brew install --cask dokimasia/tap/techne
```

The cask supports arm64 and amd64. It downloads the release archive for your machine and links
`techne` into the `bin` directory of Homebrew. The binary is not notarized, so on macOS the cask
removes the quarantine attribute. To update techne, run `brew upgrade --cask dokimasia/tap/techne`.

To install techne with `brew bundle`, add these lines to your `Brewfile`:

```ruby
tap "dokimasia/tap"
cask "dokimasia/tap/techne"
```

### Linux packages

Each release has `deb`, `rpm` and `apk` packages for amd64 and arm64. Download the package of
your distribution from the [latest release](https://github.com/dokimasia/techne/releases/latest),
check its digest against `checksums.txt`, and install it with the package manager of your
distribution. The package installs `techne` into `/usr/bin`.

### Windows

Download `techne_<version>_windows_amd64.tar.gz` or `techne_<version>_windows_arm64.tar.gz` from
the [latest release](https://github.com/dokimasia/techne/releases/latest). Check its digest
against `checksums.txt`, extract it with `tar -xf`, and put `techne.exe` on your `PATH`.

### Verify a download

The latest release has archives for Linux, macOS and Windows on amd64 and arm64, packages for
Linux, and `checksums.txt` with the SHA-256 digest of each file. cosign signs `checksums.txt`
without a key, and each archive and package has an SPDX SBOM. `gh attestation verify` checks the
build provenance of an archive or a package:

```sh
gh attestation verify techne_<version>_linux_amd64.tar.gz --repo dokimasia/techne
```

### Build from source

techne uses cgo, because the grammars are C. You need Go 1.27.3 or later and a C compiler. From a
checkout of this repository, run:

```sh
go build -o techne ./cmd/techne
```

Put the binary on your `PATH`. `go install go.dokimi.dev/techne/cmd/techne@latest` fails, because
the `go.mod` of techne has replace directives. `techne --version` prints the version of a binary.

## Install the language servers

techne starts the language server of a language from your `PATH`. `workspace`, `outline`,
`search`, `relations` and `document.symbol` need only the grammar. `resolve`, `verify`,
`rename.symbol`, `move.file` and `extract.function` need the server of the language. With the
server, `relations` returns resolved sites instead of the sites that spell the name.

Install the server of each language that you work in:

| Language | Server | Install |
|---|---|---|
| C | `clangd` | `brew install llvm`, the `clangd` package of your distribution, or the LLVM installer of [releases.llvm.org](https://releases.llvm.org) on Windows |
| C# | `csharp-ls` | `dotnet tool install --global csharp-ls` |
| Go | `gopls` | `go install golang.org/x/tools/gopls@latest` |
| Java | `jdtls` | `brew install jdtls`, or a [milestone build](https://download.eclipse.org/jdtls/milestones/) with its `bin` directory on your `PATH` |
| JavaScript and TypeScript | `typescript-language-server`, or `tsc` for TypeScript 7 | `npm install -g typescript-language-server`, and `npm install -g typescript` for TypeScript 7 |
| Python | `pyright-langserver` | `npm install -g pyright` |
| Ruby | `ruby-lsp` | `gem install ruby-lsp` |
| Rust | `rust-analyzer` | `rustup component add rust-analyzer` |
| Scala | `metals` | `cs install metals`, with [Coursier](https://get-coursier.io) |

On macOS and Linux, Homebrew installs every server but csharp-ls with one command:

```sh
brew install gopls pyright typescript-language-server rust-analyzer jdtls llvm ruby-lsp metals
```

These servers need a runtime, or a directory on your `PATH`:

- csharp-ls needs the .NET 10 SDK or later. `dotnet tool install` writes the program into
  `~/.dotnet/tools`, so put that directory on your `PATH`.
- `go install` writes gopls into `$(go env GOPATH)/bin` by default, so put that directory on your
  `PATH`.
- jdtls needs Java 21 or later, and its `bin/jdtls` script needs Python 3.9. The Homebrew formula
  installs both.
- metals needs a JDK. The Homebrew formula installs one.

Each server reads the build of its project:

| Language | The project needs |
|---|---|
| C | `compile_commands.json` |
| C# | a `.csproj` or a `.sln` that `dotnet restore` has restored |
| Go | `go.mod` or `go.work`, and the `go` command for the type checker that runs inside techne |
| Java | a Maven or a Gradle build |
| JavaScript and TypeScript | the `typescript` package in `node_modules`. From version 7, techne runs `tsc --lsp` |
| Python | an interpreter with the dependencies of the project |
| Ruby | a `Gemfile.lock` when the project has a `Gemfile` |
| Rust | `Cargo.toml` |
| Scala | a build that metals imports, such as `build.sbt` |

`capabilities` lists each engine that cannot run, with the reason.

## Connect an agent

### Claude Code

Add techne to Claude Code for every project:

```sh
claude mcp add --scope user techne -- techne
```

Claude Code starts techne in the directory of each session, and techne serves that directory.
Without `--scope`, `claude mcp add` adds techne to the current project alone. `--scope project`
writes it into the `.mcp.json` of the project for everyone who works on it.

Install the skill, so that the agent uses techne in place of grep, find, cat and sed for code:

```sh
mkdir -p ~/.claude/skills/techne
curl -fsSL -o ~/.claude/skills/techne/SKILL.md \
  https://raw.githubusercontent.com/dokimasia/techne/main/skills/techne/SKILL.md
```

Start Claude Code in a repository and ask about its code. Ask what calls a function, or ask the
agent to rename a type. The first call that needs a language server starts it. Most servers start
in a few seconds, and jdtls and metals take tens of seconds.

### Other clients

A client that reads a JSON configuration starts techne with the same command:

```json
{
  "mcpServers": {
    "techne": {
      "command": "techne"
    }
  }
}
```

When a client starts techne outside the project, add the path of the project to `args`. With
`"args": ["--trust", "~/Projects"]`, a call can also set its field `wd` to any other project under
`~/Projects`.

## Tools

| Tool | What it does |
|---|---|
| `workspace` | Lists the units of a directory, such as its packages, with their files, declarations and summaries |
| `outline` | Lists the declarations of a file or a directory |
| `search` | Finds declarations by name |
| `resolve` | Returns the declaration that the name at a position denotes |
| `relations` | Returns the relations of one kind of a declaration, such as its callers or its implementations |
| `verify` | Runs the checks of a language over a file or a directory |
| `capabilities` | Lists the engines of each language, with the tier and the cost of each role |
| `rename.symbol` | Renames a declaration and every reference to it |
| `move.file` | Moves a file and rewrites the references to it |
| `extract.function` | Moves a run of lines into a new function and calls it |
| `document.symbol` | Writes documentation onto a declaration |
| `apply.change` | Writes the change of a preview |

## Why techne

An agent without techne reads code with `grep` and changes it with find and replace. `grep`
matches spelling. It returns every `Close` in a repository and every comment that mentions one,
and it cannot report whether it found every caller. A find and replace also renames the old name
in comments and in strings.

techne asks the parser, the type checker or the language server of the language instead. To list
the types that implement the interface `Relator` of this repository, an agent calls `relations`
with these arguments:

```json
{"scope": "techne-core/engine/port.go", "name": "Relator", "relation": "implemented-by"}
```

techne returns each type with its line of code:

```text
implemented-by Relator — 9 sites

techne-core/engine/doc_test.go:16:6  in server
  type server struct{ complete }

techne-core/engine/port_test.go:57:6  in complete
  type complete struct {

techne-lang-go/checker/engine.go:27:6  in Engine
  type Engine struct {

techne-lang-go/constrained.go:38:6  in constrained
  type constrained struct {

techne-lang-mock/engine.go:21:6  in Engine
  type Engine struct {

techne-lang/lsp/engine.go:37:6  in Engine
  type Engine struct {

techne-lang/lsp/lsptest/outline.go:62:6  in parser
  type parser struct{ root string }

techne-lang/treesitter/engine.go:30:6  in Engine
  type Engine struct {

techne-service/query/service_test.go:42:6  in answering
  type answering struct {

resolved, partial coverage. the build constraints exclude 3 files from the build that the answer reads, such as techne-service/workspace/files/lock_other.go.
```

A Go type implements an interface without naming it, so a text search for `Relator` cannot list
these types. The last line states the evidence. `resolved` means that a type checker or a
language server bound the names. The coverage is partial because the build for Linux excludes 3
of the files that the answer reads.

## How it works

### Engines

techne serves each role of a language with an engine, and `capabilities` lists the engines:

- The tree-sitter engine parses with the grammar of the language. It serves `workspace`,
  `outline`, `search`, `relations` and `document.symbol` at the syntactic tier, where a name
  matches every declaration of that spelling.
- The language-server engine runs the server of the language over LSP 3.17. It serves `resolve`,
  `relations`, `verify` and the write tools at the resolved tier.
- For Go, a type checker runs inside techne. It loads the packages of the workspace through
  `golang.org/x/tools/go/packages` and the `go` command. It serves `resolve`, `relations`,
  `verify` and the write tools of Go, as gopls does.

A call goes to the engine with the strongest evidence that can run. When a server did not offer a
request at its start, its engine declines, and the next engine of the language serves the call.

### Language servers

techne starts the server of a language at the first call that needs it, and keeps it running until
techne exits or closes the workspace. Later calls reuse the index of the running server. A server
that does not reply to `initialize` within 10 seconds declines each call until it replies, and the
next engine of the language serves the call.

A server analyses the buffers that it receives, not the files on disk. Before each call, techne
sends the server each file that changed on disk since the server received it. It also sends the
creations, changes and deletions of the files that the watchers of the server match. So an answer
takes the edits of an editor or of another tool into account.

A call waits until the server has ended its progress jobs and has been idle for 300 milliseconds.
An answer from a server that has not settled is partial. A server whose process exits
is replaced at the next call.

### Evidence

The last line of every answer states its fidelity and its coverage, such as
`resolved, total coverage`.

- The fidelity is how the engine bound the names. `syntactic` means that a parser matched the
  text. `indexed` means that an index resolved the name without a type checker. `resolved` means
  that a type checker or a language server bound each name to its declaration.
- The coverage is how much of the scope the engine examined. A partial answer ends with a caveat
  about what the engine left out.
- An empty answer proves that there are none only with resolved fidelity and total coverage. The
  line then adds `an empty answer here means there are none`.

A language server that binds names through types and still indexes the workspace returns
`resolved, partial coverage`. Its empty answer does not prove absence, and techne refuses a rename
on its evidence. With `--structured`, the JSON of a result contains the evidence as `fidelity`,
`completeness` and `supportsNegativeClaim`.

### Writes

A write tool returns a preview unless the call sets `dry_run` to false. techne projects the change
over the files in memory and checks the projection before it writes anything. The preview contains
the rewritten lines, the engine that checked them and a handle. `apply.change` takes the handle and
writes every file of the change or none of them.

- `rename.symbol` and `move.file` need resolved fidelity and total coverage, because a claim to
  have found every reference is a claim that no other reference exists. Otherwise techne refuses
  the change and states the reason.
- The check refuses a change whose result has more errors than the code it replaces, and lists the
  errors.
- The grammar checks a change of C, because clangd reads the headers of a file from disk and not
  from the buffers that techne sends.
- `apply.change` refuses a handle when a file of the change changed after the preview.
- A write takes an advisory lock of the operating system for the workspace. Agents that each run
  their own techne over one repository do not interleave their writes.

## Trusted folders

techne serves one workspace per process. `--trust DIR` lets one process serve every project under
the folder DIR:

```sh
claude mcp add --scope user techne -- techne --trust ~/Projects
```

With a trusted folder, every tool takes the field `wd`. Its value is a directory under a trusted
folder, and a relative directory is relative to the workspace. A call with `wd` runs in the
workspace of that directory, and the paths of the call and of its answer are relative to it. A
call without `wd` runs in the workspace that techne serves. One session can then read a library
and the service that uses it, each in its own workspace.

- techne refuses a `wd` outside the trusted folders. Trust only folders whose builds you would
  run, because a language server runs the build of its project.
- techne keeps three such directories open with their language servers. A call in a fourth
  directory closes the directory that served a call least recently and has no call in flight. The
  previews of a closed directory are gone, and `apply.change` refuses their handles.
- `--trust` repeats for more than one folder. techne expands a leading `~`, because an editor
  starts techne without a shell.

## Command line

```text
techne [-h | --help] [--version] [--structured] [--trust DIR]... [workspace]
```

- `workspace` is the root of the workspace, and the working directory when you omit it.
- `--structured` sends each result as JSON beside its text, and declares the output schema of
  each tool.
- `--trust DIR` adds a trusted folder, and repeats for more than one folder.
- `-h` and `--help` write the usage to standard output and exit. `--version` writes the version
  and exits.

While techne serves a workspace, standard output contains the messages of the protocol alone.
Every diagnostic goes to standard error. techne exits with status 0 when the client closes
standard input and after SIGINT or SIGTERM. It exits with 1 for a workspace or a trusted folder
that it cannot serve, and with 2 for an unknown flag, a `--trust` that lacks a folder or a second
workspace.

By default a result contains its answer as text written for a model, and techne declares no
output schema. Claude Code passes the JSON of a result to the model when the result contains
JSON, and the text is about half its size. Pass `--structured` when your client validates or
processes the JSON.

## Dependencies

| Module | Use |
|---|---|
| [github.com/modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) | The server of the Model Context Protocol |
| [github.com/tree-sitter/go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter) | The parser, with a `tree-sitter-<language>` module for the grammar of each language |
| [go.lsp.dev/protocol](https://github.com/go-language-server/protocol) | The types of LSP 3.17, with `go.lsp.dev/jsonrpc2` for the connection to each server |
| [golang.org/x/tools](https://pkg.go.dev/golang.org/x/tools) | `go/packages`, through which the Go type checker loads the packages of a workspace |
| [github.com/bmatcuk/doublestar](https://github.com/bmatcuk/doublestar) | The glob patterns of the file watchers of a server |

The grammar of Go is the fork [dokimasia/tree-sitter-go](https://github.com/dokimasia/tree-sitter-go),
which parses the type parameters of a method.

## The Claude Code skill

`skills/techne/SKILL.md` is a skill for Claude Code. It covers:

- the tool for each question about code
- the evidence that ends each answer, and what an empty answer proves
- the preview and the apply of a change

From a checkout of this repository, link the skill instead of a copy, so that it changes with
the checkout:

```sh
ln -s "$PWD/skills/techne" ~/.claude/skills/techne
```

A project can also include the skill in its own `.claude/skills/` directory.

## Security

techne starts the language server of a language at the first call about it. A server runs the
build of its project. rust-analyzer runs `cargo check`, and jdtls imports a Gradle or a Maven
build. Serve only a workspace whose build you would run.

Report a vulnerability by email to security@dokimi.dev and not in an issue.
[SECURITY.md](SECURITY.md) describes what a report contains and which bugs to report privately.

## Documentation

| Directory | Contents |
|---|---|
| [docs/architecture](docs/architecture) | How the system is put together |
| [docs/rfc](docs/rfc) | The proposals and the argument behind them |
| [docs/adr](docs/adr) | The decisions and what they cost |
| [docs/roadmap](docs/roadmap) | The order of the work |

## Development

The repository uses [ergon](https://github.com/dokimasia/ergon) for its checks and its releases.
[CONTRIBUTING.md](CONTRIBUTING.md) describes the setup and how to propose a change. The project
follows the [Contributor Covenant 2.1](CODE_OF_CONDUCT.md).

```sh
make check        # the gate: lint, tests, race detector and vulnerability scan of every module
make test         # the tests of every module
make corpus       # drive the binary over large repositories, hours on a first run
```

`make help` lists every target. To drive the tools without a language server, set
`TECHNE_MOCK=1`. techne then adds a mock language that serves every tool but `extract.function`
at the resolved tier.

## License

Dokimasia B.V. licenses techne under the MIT license. See [LICENSE](LICENSE).
