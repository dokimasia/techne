// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package change applies an operation to the workspace, or reports what
// applying it would do.
//
// # One pipeline, whatever the operation
//
// [Service.Apply] validates the request against the operation's spec,
// asks a language for a plan, pins the content that plan was computed
// against, puts it to the policy, projects it, gates the projection and
// only then writes. Every operation takes the same steps, so what counts
// as safe cannot drift between a rename and a comment.
//
// # The gate runs before the write
//
// RFC-0004 draws the gate after the apply, with a rollback behind it,
// because a build gate needs files on disk. A gate that reads content
// rather than the workspace does not, and running it first means a
// refused change never touches a file. The rollback stays for the
// narrower case it is still needed in: a write that fails partway
// through a multi-file change.
//
// # A gate is worth what it refuses
//
// What gates a change is whatever serves [go.dokimi.dev/techne/core/engine.Checker]
// for the language, and today that is a parser. It refuses a change that
// stopped the file being the language it was, which is how writing a
// comment fails: the text ends the comment early and the rest becomes
// code. It does not refuse a change that still parses and no longer
// compiles, and it is more forgiving than a compiler even about syntax.
// A change reports which gate ran, so nobody reads "it parses" as "it
// builds".
//
// # A dry run is gated, not previewed
//
// A dry run is the same call without the write. The projection it gates
// is byte-for-byte what an apply would put on disk, so a dry run whose
// gate passed is a promise about the apply rather than a diff to read.
//
// # What is pinned, and what that is worth
//
// [Service.Apply] seals the content itself rather than trusting a
// planner to, because a planner that forgets produces a plan that
// applies cleanly to a file it never saw. It seals after planning, so a
// file rewritten between the planner's read and the seal is pinned as it
// is now and not as the planner saw it. The gate is what catches that:
// offsets computed against other bytes describe other code, which
// stops parsing. Locks are held from the seal to the write, so nothing
// in this process can open that window.
//
// # Dependency position
//
// Imports core/diag, core/edit, core/engine, core/source and
// core/trust. [Router] and [Files] are ports: the registry that knows
// which language claims a path lives in another module, and core names
// no language and opens no file.
package change
