# RFCs

Proposals, and the argument that produced them. An RFC is written to be
argued with. Numbers are never reused and never renumbered, including for
proposals that are withdrawn.

| RFC | Title | Status |
|---|---|---|
| [0001](0001-module-boundaries.md) | Module boundaries | Accepted |
| [0002](0002-the-read-contract.md) | The read contract | Accepted |
| [0003](0003-the-tool-surface.md) | The tool surface | Accepted |
| [0004](0004-operations-and-the-write-path.md) | Operations and the write path | Accepted |
| [0005](0005-turn-cost.md) | Turn cost | Accepted |
| [0006](0006-what-a-tool-returns.md) | What a tool returns | Draft |

RFC-0001 fixes which module each thing lives in; ADR-0004 records why
the services and the tool surface are not among the things in `core`. The next three fix what
those things are: 0002 the types and ports every engine speaks, 0003 what
an agent sees, 0004 what changes a file. RFC-0005 cuts across all three
and treats the number of round trips an agent spends as a cost the design
has to pay down. RFC-0006 settles the field-by-field shape of every input
and answer, which 0003 named but left to whoever wrote each handler.
