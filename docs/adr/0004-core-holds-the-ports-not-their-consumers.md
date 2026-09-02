---
adr: 0004
title: core holds the ports, not their consumers
status: Accepted
date: 2026-09-02
supersedes: none
superseded-by: none
rfc: RFC-0001
---

# ADR-0004: core holds the ports, not their consumers

## Status

Accepted

## Context

`core` was laid out to hold everything that does not know a language:
the vocabulary, the ports an engine implements, the read and write
services, the workspace, and the surface an agent calls. That is a
coherent sentence and it produced a module where 64% of the code
consumed the other 36%.

The measurement: 1037 lines of vocabulary and ports, against 1247 lines
of tool surface and 580 lines of services and filesystem. The tool
surface alone was 44% of the module and implements no port.

Sharing a module let the two sides grow into each other, and they did.
The tool surface came to take a `*query.Service` rather than an
interface, so a tool could not be built without the read path. The write
path came to hold its own copy of the rule deciding which language owns
a path, duplicating the read path's, so a read and a write could
disagree about who serves a file and a change could be planned by one
engine and gated by another. Neither was a decision anybody made; both
were convenient at the time and nothing objected.

What made them repairable was that no boundary had to be invented to
find them. Both were visible as duplication inside one module. What
makes them recur is that nothing stops the next one.

## Decision

We will keep in `core` only what an engine implements and the vocabulary
those ports speak, and move everything that consumes a port out: the
read and write paths and the workspace to `techne-service`, and the
surface an agent calls to `techne-tool`. depguard enforces it, so core
importing a service fails the build rather than a review.

A tool is given a service through an interface the tool declares itself,
one per question asked. `techne-tool` therefore does not depend on
`techne-service`, and neither depends on the other. Only the root module
joins them.

The consumers moved rather than the contract. `core` keeps its name and
its import path, so no language module's imports changed at all: the ten
of them import `sema`, `engine`, `source`, `trust`, `edit` and `diag`,
which is what they imported before.

## Alternatives Considered

### One module for both, called service

`query`, `change`, `workspace` and `tool` together, leaving `core` the
same size as under this decision for half the ceremony.

It lost because it re-lumps the two things whose entanglement caused the
problem. The tool surface holding a service is the defect this was
written to stop recurring, and putting them back in one module makes it
available again.

### Keep one module and enforce layers with depguard on package paths

The packages stay in `techne-core`, and depguard forbids the vocabulary
importing the services and the services importing the tool surface.

It lost on what the module promises rather than on what it enforces.
depguard would have caught both defects either way. But a module named
`core` that holds 1827 lines which are not core teaches every reader
that core means "the language-agnostic half", and the next thing that is
language-agnostic and not a port lands there for the same reason these
did.

### Move the contract down instead

Extract the vocabulary and the ports into a module below `core`, leaving
the services where they are. RFC-0001 considered this and rejected it on
the grounds that the coupling it removes is version coupling, of which
there is none inside one repository with a workspace.

That reasoning still holds, and it cuts the same seam this decision
cuts. It loses on direction. Moving the contract down renames what every
module already imports; moving the consumers out leaves `core` holding
what it always held.

### Move the tool surface into presenter

`techne-presenter` is already the agent-facing module, so the surface
could join the transport and save a module.

It lost because RFC-0001 keeps schema derivation out of the transport on
purpose: deriving a tool's schema from its Go types is what makes a
second transport cost a package rather than a second schema. Putting the
two in one module puts the MCP SDK within the surface's reach, and the
argument for keeping them apart is the same one that made presenter a
module in the first place.

## Consequences

Adding a tool that needs a new read role is now an edit to three
modules: `core` for the port, `service` for the path that drives it, and
`tool` for what an agent sees. That is the price of the boundary and it
is paid per capability rather than per change.

Seven `go.mod` files instead of five, each with its own tidy, its own
pinned versions and its own row in the coverage and CI matrices.

`core` no longer depends on `jsonschema-go`, which left with the surface
that derives schemas. Nothing below the tool surface pulls it in.

A test that wants to drive a tool over a real service now spans two
modules, so it belongs in the root module rather than in either. The
tool module's own tests use the interfaces it declares, which is what
those interfaces are for and makes them unit tests rather than
integration tests wearing the name.

## References

| What | Where |
|---|---|
| depguard, the linter enforcing the boundary | https://github.com/OpenPeeDeeP/depguard |
| Go modules and the unit in which dependencies are declared | https://go.dev/ref/mod |
