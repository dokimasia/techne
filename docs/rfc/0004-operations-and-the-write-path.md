---
rfc: 0004
title: Operations and the write path
author: Roy Klopper
status: Accepted
created: 2026-09-01
updated: 2026-09-02
discussion: none
supersedes: none
superseded-by: none
produces-adr: none
---

# RFC-0004: Operations and the write path

## Summary

Which operations exist, what each one declares about the evidence it
needs, and the pipeline every change passes through. An operation that
rewrites references is admitted only on evidence that licenses a negative
claim, because "I found every reference" is a claim that there are no
others.

## Motivation

RFC-0001 drew the write pipeline and named none of its types. Nothing can
be built against a pipeline whose stages have no names, and nothing can
be refused consistently against operations that declare nothing about
themselves.

The pipeline is also where a mistake is expensive in a way a read is not.
A wrong read wastes a turn. A rename that updates nine of ten references
leaves code that compiles, passes the gate, and fails at run time in
whatever path the tenth reference was on.

That failure has a precise cause worth designing against. Finding every
reference is not a positive claim that can be checked by looking at what
was found; it is a claim that nothing else exists. The read contract
already says what licenses that: a type checker bound the name, and the
engine covered the whole scope. The write path is where that stops being
a label on an answer and starts deciding whether a file is written.

## Detailed design

### The catalogue

Twelve operations across seven families, named `family.subject`.

| Operation | Target | Rewrites references | Weakest fidelity that can be correct |
|---|---|---|---|
| `rename.symbol` | symbol | yes | resolved |
| `rename.file` | file | yes | resolved |
| `move.file` | file | yes | resolved |
| `move.symbol` | symbol | yes | resolved |
| `extract.function` | span | no | resolved |
| `extract.variable` | span | no | resolved |
| `extract.interface` | symbol | no | resolved |
| `inline.variable` | symbol | yes | resolved |
| `inline.constant` | symbol | yes | resolved |
| `change.signature` | symbol | yes | resolved |
| `implement.interface` | symbol | no | resolved |
| `document.symbol` | symbol or span | no | syntactic |

`document.symbol` writes a comment above a declaration and touches
nothing else, which is why it is the one operation a parser can serve
correctly. It is in the table to show that the minimum is a property of
the operation rather than a global setting. Every other operation either
rewrites references or needs a type to be correct.

It is also the one operation that takes a span as readily as a symbol. A
name and a kind do not pick out one declaration, because a unit
declaring two methods called `Get` satisfies one identity twice, and the
prose to write is the caller's rather than something derived from the
target. A caller that has already resolved which declaration it means
says so by position.

The family exists so a caller can ask what extractions there are, and so
a language module can advertise or refuse a family as a unit.

### Types

```go
// Package edit describes how code changes.
package edit

// Operation is one thing a caller can ask for, named family.subject.
type Operation string

const (
	RenameSymbol       Operation = "rename.symbol"
	RenameFile         Operation = "rename.file"
	MoveFile           Operation = "move.file"
	MoveSymbol         Operation = "move.symbol"
	ExtractFunction    Operation = "extract.function"
	ExtractVariable    Operation = "extract.variable"
	ExtractInterface   Operation = "extract.interface"
	InlineVariable     Operation = "inline.variable"
	InlineConstant     Operation = "inline.constant"
	ChangeSignature    Operation = "change.signature"
	ImplementInterface Operation = "implement.interface"
	DocumentSymbol     Operation = "document.symbol"
)

// Family is the part before the dot.
func (o Operation) Family() Family

// TargetKind is what an operation is pointed at.
type TargetKind uint8

const (
	TargetSymbol TargetKind = iota
	TargetFile
	TargetSpan
)

// Target says what to operate on. Exactly one field matching Kind is set.
type Target struct {
	Kind   TargetKind
	Symbol sema.ID
	Path   source.Path
	Span   source.Span
}

// ArgKey names a parameter. Every string crossing this boundary is a
// constant with one definition point, because a mistyped key produces an
// operation that silently does nothing.
type ArgKey string

const (
	ArgNewName     ArgKey = "new_name"
	ArgDestination ArgKey = "destination"
	ArgSignature   ArgKey = "signature"
	ArgReceiver    ArgKey = "receiver"
	ArgDoc         ArgKey = "doc"
)

type Args map[ArgKey]string

// Spec is what an operation declares about itself. The service validates
// a request against it before any planner is chosen, so a malformed
// request produces one consistent refusal whatever language would have
// served it.
type Spec struct {
	Operation Operation
	Accepts   []TargetKind
	Required  []ArgKey
	Optional  []ArgKey

	// MinFidelity is the weakest binding this operation can be correct
	// on.
	MinFidelity trust.Fidelity

	// RewritesReferences says this operation changes code that refers to
	// the target. Admission then also requires completeness, because
	// finding every reference is a claim that no others exist.
	RewritesReferences bool
}
```

### A plan is inert

```go
// Plan is what a planner produces. It holds no filesystem handle, so it
// can be inspected, diffed and thrown away without touching the
// workspace. A dry run is the real call without the commit.
type Plan struct {
	Operation     Operation
	Changes       []Change
	Preconditions []Precondition
	Provenance    trust.Provenance
}

type ChangeKind uint8

const (
	ChangeEdit ChangeKind = iota
	ChangeCreate
	ChangeDelete
	ChangeMove
)

type Change struct {
	Kind    ChangeKind
	Path    source.Path
	To      source.Path // ChangeMove only
	Edits   []TextEdit  // ChangeEdit only, sorted by Span.Start, non-overlapping
	Content []byte      // ChangeCreate only
}

// Request is one change a caller asked for. It is vocabulary rather than
// the write path's own type, so a tool can name a change without
// depending on the thing that applies one.
type Request struct {
	Operation Operation
	Scope     source.Path
	Language  source.Language
	Target    Target
	Args      Args
	DryRun    bool
}

type TextEdit struct {
	Span source.Span
	New  string
}

// Precondition pins the content a plan was computed against. The service
// seals these, not the planner.
type Precondition struct {
	Path   source.Path
	Digest [32]byte // SHA-256 of the file as the planner read it
}
```

The service seals preconditions rather than trusting a planner to do it,
because a planner that forgets produces a plan that applies cleanly to a
file it never saw. Byte ranges computed against different content
describe something else, and the result usually still compiles, so the
gate does not catch it.

### Admission

```go
// Admit reports why a plan may not be applied, or nil.
func (p Policy) Admit(spec Spec, plan Plan) error
```

Four checks, in order:

1. The plan's fidelity is at least `spec.MinFidelity`.
2. If `spec.RewritesReferences`, then
   `trust.SupportsNegativeClaim(plan.Provenance.Fidelity,
   plan.Provenance.Completeness)` is true.
3. No two edits in the plan overlap, and every edit list is sorted.
4. Every path the plan touches has a precondition.

Check 2 is what makes splitting fidelity from completeness worth a second
field. A
language server that binds through types but is still building its index
reports `resolved` and `partial`. It can serve `outline` and `relations`
with a caveat. It cannot rename, because the set of references it found
is not the set that exists, and nothing in the fidelity alone would have
said so.

### The pipeline

RFC-0001 draws the stages. Three rules govern them and are settled here.

**The gate runs before the write, where the gate can read content.** A
gate that judges bytes rather than the workspace needs nothing on disk,
so it runs against the projection and a refused change never touches a
file. That is a separate port from the one that runs a build:

```go
// Checker reports what is wrong with content the workspace does not
// hold. A dry run gates a projection, and a projection exists only in
// memory, so a gate that can only read the workspace cannot serve one.
type Checker interface {
	Check(ctx context.Context, files map[source.Path][]byte) (Result[diag.Diagnostic], error)
}
```

A build gate still needs files on disk and still runs after the write,
with the rollback behind it. Both report which of them answered, because
"it parses" and "it builds" are different promises and a caller told only
that a gate passed cannot tell which one it was given.

**A gate judges what a change replaces as well as what it produces.** A
file that did not parse before is not made worse by a comment written
into it, and refusing on inherited faults would make the code that most
wants fixing the code nothing may touch.

**Locks are held from the seal to the write.** Per-path, in-process,
acquired in sorted path order so two changes touching the same files in
different orders cannot deadlock, plus one advisory workspace lock for a
second process. Releasing between pinning the content and writing it back
would let another change land in between, and the byte ranges would then
describe a file nobody computed them against.

**Formatting touches only what the plan named.** The format step runs
over the paths in `plan.Changes` and no others. If any other file
changed, the gate fails. A formatter configured over the whole workspace
would otherwise fold unrelated reformatting into a rename, and the diff
would be unreviewable.

A language whose formatter has to touch a sibling, to normalise imports
across a package, names those paths in the plan. They then get a
precondition and a lock like every other touched path, rather than being
an exception to the rule.

**The gate records which verifier answered.** An in-process `Verifier` is
preferred; a language's declared argv is the fallback. The result names
which ran, because "the build passed" means different things when it came
from a type checker in this process and from a subprocess that may not
have seen the same files.

### A dry run is gated, not just previewed

`dry_run` runs the whole pipeline against an overlay: the plan's changes
are projected over the real files in memory and the gate runs against
that projection, without a byte being written.

A preview that only prints a diff answers "what would change". This
answers "would it still build", which is the question the caller has, and
it means a dry run that reports the gate passing is a promise that
applying for real compiles. An agent can then batch a dry run and an
apply without reading the diff in between.

### A failed gate carries its fixes

When the gate fails, each diagnostic that has a known fix carries it as a
ready `Plan`. A lint, fix and re-verify cycle costs two round trips
rather than five, because the caller never has to work the edit out from
the message.

The limit is the same as anywhere else this is done: carry a fix when
there is one obvious fix. A diagnostic with three plausible ones carries
none, because three payloads to save one round trip that may not be taken
is a bad trade.

### Batches share one gate

Half the catalogue is naturally plural: moving eight symbols, documenting
every declaration in a file. Applying those one at a time leaves the
workspace broken between members and costs N verifications.

```go
// Batch merges plans and admits on the weakest member.
func (s *Service) Batch(ctx context.Context, plans []Plan) (Result, error)
```

Plans are merged and checked for overlap before anything is written, and
the batch is admitted on its weakest member, so a resolved change cannot
carry a syntactic one past the policy.

Every plan in a batch is for one language. The gate is a language's own
verifier, so a mixed batch would need a gate per language and a rollback
rule for the case where one passes and another fails. A caller changing
two languages sends two batches.

### What a caller gets back

```go
// Outcome is what the write path did, or would have done. It is named
// for what it is rather than called a result, because an engine returns
// what it found and a tool returns what it encoded, and those are three
// different things.
type Outcome struct {
	Operation   Operation
	Status      trust.Status
	Applied     bool
	Changed     []source.Path     // empty for a dry run
	Changes     []Change          // what was done, or would be
	Rewrites    []Rewrite         // the same, read back as text
	Diagnostics []diag.Diagnostic // when the gate objected
	Provenance  trust.Provenance
	Reason      string // when Status is Refused or Unsupported
}

// Rewrite is one range a change replaces, with the text on both sides.
// A plan says which bytes move; a caller reviewing a change reads what
// goes and what arrives. The write path fills these in because it has
// read the file and the planner has not.
type Rewrite struct {
	Path source.Path
	Line int // counting from one, as an editor reports it
	Was  string
	Now  string
}
```

`Applied` is false and `Changed` is empty on every failure path. There is
no partial success: either the gate passed and the files are as the plan
described, or every file is byte-identical to what it was.

## Alternatives considered

### A. One fidelity minimum for every write

Require `resolved` for anything that writes, and drop `MinFidelity` from
the spec.

**Why not:** it makes `document.symbol` impossible in nine of ten
languages for no reason. Writing a comment above a declaration needs the
declaration's position and nothing else, and refusing it would push the
agent back to editing the file by hand, which is the failure mode the
tool exists to remove.

### B. Trust the fidelity tier alone for reference rewrites

Admit a rename on `resolved` without checking completeness.

**Why not:** it readmits exactly the failure the read contract was
reshaped to prevent. A server mid-index reports `resolved` truthfully and
has seen half the workspace, and the rename that follows updates half the
references.

### C. Let each planner decide whether it is safe

Move admission into the language module, where the planner knows most
about its own limits.

**Why not:** the rule then differs per language and cannot be reviewed in
one place. It also puts the judgement in the component with the strongest
reason to be optimistic about itself, which is the same argument that
keeps engines from constructing their own provenance.

### D. Apply, then verify, then keep or revert with no locking

Skip the lock and rely on the gate to catch interference.

**Why not:** the gate cannot tell which of two interleaved changes broke
the build, and a rollback computed from one change's snapshot discards
the other's work. Locks are cheaper than the failure they prevent.

## Drawbacks

- Twelve operations are declared and one is planned. Eleven entries in
  the catalogue are refusals for every language until someone writes a
  planner, and a caller reading the catalogue may reasonably expect more
  than it can do.
- `RewritesReferences` plus `MinFidelity` means two fields decide
  admission, and an operation added with the wrong pair fails open rather
  than closed. It wants a test per operation rather than a review.
- Requiring completeness for reference rewrites means a language server
  that never reports completeness can never rename, even where it is in
  fact complete. Engines that cannot say will need to learn to.
- Holding locks across the gate serialises changes to overlapping files
  for the whole duration of a build, which for a large workspace is
  seconds rather than milliseconds.
- Preconditions are whole-file digests, so an unrelated edit anywhere in
  a touched file invalidates a plan that would still have applied
  correctly.

## Unresolved and future work

Two questions wait for more of the write path to exist. Whether
`extract.variable` can be correct on a parser's evidence in a language
with type inference, and whether the second-process lock is a file in the
workspace or something the caller supplies.

What a dry run reports for a language whose toolchain has no in-memory
projection is settled: it reports that nothing judged the change. The
status is degraded and a caveat says so, because a language nothing can
gate is one where refusing every change would be the only alternative.

Applying the edits a language server proposes through a command is not
proposed. Those edits arrive outside the pipeline and would be the one
path that bypasses policy, so capturing them as a proposal is where this
stops.

Undo beyond the rollback of a single call is not proposed. A caller that
wants to reverse a change that already passed the gate uses version
control.

Planners for the eleven operations without one are not proposed here.
Each needs its own design per language.

## References

| What | Where |
|---|---|
| SHA-256 as the precondition digest | https://pkg.go.dev/crypto/sha256 |
| Atomic file replacement semantics on POSIX | https://pubs.opengroup.org/onlinepubs/9699919799/functions/rename.html |
