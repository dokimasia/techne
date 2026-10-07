# techne

techne is a server of the Model Context Protocol that gives an AI agent the code navigation and
the refactorings of an IDE in ten languages: C, C#, Go, Java, JavaScript, Python, Ruby, Rust,
Scala and TypeScript. One techne process serves a workspace over standard input and output. With
`--trust`, it also serves each directory under a trusted folder that a call sets in its `wd`
field.

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

A write tool previews its change unless the call sets `dry_run` to false. The preview contains
the rewritten lines, the check that judged the result, and a handle that `apply.change` takes.
techne writes a change in full or not at all.

Every answer states its evidence. The last line of the text states the fidelity and the
completeness, such as `resolved, total coverage`. An empty answer adds `an empty answer here
means there are none` when it proves that there are none. With `--structured`, the JSON of a
result contains the same evidence:

- `fidelity` is how the engine bound the names: `syntactic`, `indexed` or `resolved`.
- `completeness` is how much of the scope the engine examined.
- `supportsNegativeClaim` is true when the engine resolved the names and examined the whole
  scope. An empty answer proves that there are none only when it is true.

## Language servers

The grammar of each language serves `workspace`, `outline`, `search`, `relations` and
`document.symbol` without anything installed. The other tools need the language server of the language on
`PATH`:

| Language | Program |
|---|---|
| C | `clangd` |
| C# | `csharp-ls` |
| Go | `gopls`, and the `go` command for the type checker that runs inside techne |
| Java | `jdtls` |
| JavaScript and TypeScript | `typescript-language-server` with the `typescript` package in the workspace, or `tsc` for a workspace on TypeScript 7 |
| Python | `pyright-langserver` |
| Ruby | `ruby-lsp` |
| Rust | `rust-analyzer` |
| Scala | `metals` |

A server reads the project files of its language, such as `go.mod`, `Cargo.toml`, `build.sbt`
or `compile_commands.json`. `capabilities` lists each engine that cannot run, with the reason.

## Build

techne uses cgo, because the grammars are C. You need Go 1.27 and a C compiler. From a checkout
of this repository, run:

```sh
go build -o techne ./cmd/techne
```

Put the binary on your `PATH`.

## Run

```text
techne [-h | --help] [--version] [--structured] [--trust DIR]... [workspace]
```

techne serves the workspace at the path that you pass, or the working directory. While it
serves, it writes only protocol messages to standard output, and its errors to standard error.
`-h` and `--help` print the usage and exit. `--version` prints the version and exits.

`--trust DIR` adds a trusted folder, such as `~/Projects`, and repeats for more than one folder.
Every tool then takes the field `wd`, a directory under a trusted folder, absolute or relative
to the workspace. A call with `wd` runs in the workspace of that directory, and the paths of the
call and of its answer are relative to it. A call without `wd` runs in the workspace that techne
serves. techne keeps three such directories open with their language servers. A call in a
fourth directory closes the directory that served a call least recently and has no call in
flight. The previews of a closed directory are gone. `apply.change` refuses their handles.

By default a result contains its answer as text written for a model, and techne declares no
output schema. Claude Code passes the JSON of a result to the model when the result contains
JSON, and the text is about half its size. With `--structured`, a result also contains the
answer as JSON, and techne declares the output schema of each tool. Pass it when your client
validates or processes the JSON.

A write takes an advisory lock of the operating system for the workspace, and the write of
another techne process waits until the lock is free. Agents that each run their own techne
over one repository do not interleave their writes.

## Connect a client

In Claude Code, run:

```sh
claude mcp add techne -- techne /path/to/repository
```

A client that reads a JSON configuration starts techne with the same command and argument:

```json
{
  "mcpServers": {
    "techne": {
      "command": "techne",
      "args": ["/path/to/repository"]
    }
  }
}
```

An editor that starts techne in the project it opens can pass the trusted folder alone, as
`"args": ["--trust", "~/Projects"]`. A call then sets `wd` to any other project under that
folder. techne expands the `~`, because an editor passes it without a shell.

## The Claude Code skill

`skills/techne/SKILL.md` is a skill for Claude Code. It covers:

- the tool for each question about code
- the evidence that ends each answer, and what an empty answer proves
- the preview and the apply of a change

To install it for every project, link it into your skills directory from a checkout of this
repository:

```sh
ln -s "$PWD/skills/techne" ~/.claude/skills/techne
```

A project can also include the skill in its own `.claude/skills/` directory.

## Development

The repository uses [ergon](https://go.thesmos.sh/ergon) for the build, test, lint and release
lifecycle.

```sh
make bootstrap    # install dev tools
make install      # go mod download
make check        # full pre-merge gate (mod verify + lint + test + checks)
make build        # compile every module's source
make corpus       # drive the binary over large repositories, hours on a first run
```

`make help` lists every target. To drive the tools without a language server, set
`TECHNE_MOCK=1`. techne then adds a mock language that serves every tool but
`extract.function` at the resolved tier. `docs/README.md` lists the architecture documents, the
RFCs, the decision records and the roadmap.

## License

MIT. See [LICENSE](LICENSE).
