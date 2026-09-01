# Documentation

Where each kind of document lives, and which one to write.

| Directory | Holds | Write one when |
|---|---|---|
| `architecture/` | How the system is put together, kept current | Someone needs to place a file or trace a dependency |
| `roadmap/` | What we build in what order, and what has to be true first | Work needs sequencing rather than justifying |
| `rfc/` | Proposals and the argument behind them | A change alters a contract, or several approaches are worth comparing |
| `adr/` | Decisions already made, and what they cost | One decision is settled and no credible alternative is still open |

Milestone numbers live in `roadmap/` and nowhere else. An RFC or an ADR
that named one would be a second schedule, kept by someone who does not
own it.

An RFC is written to be argued with and may be rejected. An ADR records
what was decided. Do not rewrite it afterwards: to change a decision,
write a new ADR and mark the old one superseded.

Documents in `architecture/` are the exception. They describe the system
as it currently is, so edit them whenever it changes.
