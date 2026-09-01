---
adr: 0002
title: tree-sitter grammars through the cgo bindings
status: Accepted
date: 2026-09-01
supersedes: none
superseded-by: none
rfc: RFC-0001
---

# ADR-0002: tree-sitter grammars through the cgo bindings

## Status

Accepted

## Context

The syntactic tier answers questions about a file by parsing it, with no
compiler and no language server installed. That needs a grammar per
language, and tree-sitter is where those grammars come from.

There are two ways to get a tree-sitter grammar into a Go process. The
cgo bindings link the C parser and each grammar into the binary. The
other route compiles tree-sitter and its grammars to wasm and runs them
in a Go wasm runtime, which keeps the whole thing pure Go.

The two things pulling against each other are the build and the parse.
cgo means every platform the binary ships to needs a C cross-toolchain at
release time, and every contributor needs a C compiler to run the tests.
wasm means one static binary that cross-compiles anywhere, at the cost of
an interpretation layer between the engine and the grammar and a binding
far fewer people have used.

The module split already limits how far this reaches. The vocabulary, the
services and the presenters live in a module that never imports a
grammar, and Go compiles only the packages a binary reaches. So this
decision touches the language modules and the shared parser engine, and
nothing else.

## Decision

We will use the cgo tree-sitter bindings, because the grammars exist as
importable Go modules today and the wasm route would have this project
building, versioning and shipping a wasm asset per language before it can
parse anything.

## Alternatives Considered

### wasm grammars in a Go wasm runtime

Compile tree-sitter and each grammar to wasm, load them with a pure-Go
runtime such as wazero, and ship one static binary that cross-compiles to
any target with no C toolchain anywhere.

It lost on what has to be built first. The cgo ecosystem publishes
ready-to-import Go modules for the grammars this project needs; the wasm
route requires this project to produce, version and distribute a `.wasm`
per language, and to keep them in step with grammar upstreams. Far fewer
people have used the host binding, so its failure modes are less well
documented than the cgo one's. Parsing is expected to be slower, and
nobody here has measured by how much.

This is the alternative most likely to replace the decision later.
`lang/treesitter` sits behind the engine ports, so replacing it changes
one package.

### A hand-written parser per language

Skip tree-sitter and write a parser for each language directly.

It lost on the amount of work and on error recovery. The syntactic tier's
job includes answering about a file that does not currently parse, which
is when a stronger engine has already given up, and recovering usefully
from a syntax error is the part a hand-written parser gets wrong first.

### Each language's own toolchain for the syntactic tier

Shell out to the compiler or formatter each ecosystem already ships and
read its output.

It lost because it requires that toolchain to be installed, which is the
condition the syntactic tier exists to avoid. It also costs a subprocess
per file per query, where the parser costs a function call.

## Consequences

**Positive:**

- Grammars are ordinary Go modules, so adding a language at the syntactic
  tier is a dependency and a query file rather than a build pipeline.
- Parsing runs as native code with no interpretation layer between the
  engine and the grammar.
- The binding is the one most tree-sitter users in Go run, so other
  people have already documented how it behaves under load and how it
  fails.

**Negative:**

- Releasing the binary needs a C cross-toolchain for every target
  platform, so the release job needs a cross-compilation matrix rather
  than `GOOS=... go build`.
- A contributor without a C compiler cannot build or test the `lang`
  module or any language module, even to change a package that has
  nothing to do with parsing.
- Every language module that ships a grammar is cgo, so the binary is cgo
  whenever it registers one. The CGO-free build the module split makes
  possible only survives for a language set served entirely by language
  servers.
- Each grammar links its own generated C parser, so binary size grows
  with every language registered.

**Neutral:**

- This decision reaches `lang/treesitter` and the language modules, and
  nothing else. No package in `core` imports a grammar, and the
  dependency check in the pre-merge gate keeps it that way.

## References

| What | Where |
|---|---|
| cgo constraints and cross-compilation | https://pkg.go.dev/cmd/cgo |
| tree-sitter parsing and error recovery | https://tree-sitter.github.io/tree-sitter/ |
| wazero, the pure-Go wasm runtime the alternative would use | https://wazero.io |
