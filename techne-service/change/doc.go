// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package change applies an operation to the workspace, or reports what applying it would
// do.
//
// # One pipeline for every operation
//
// [Service.Apply] and [Service.Commit] take every operation through the same steps:
//
//  1. Apply checks the request against the spec of the operation, and asks the languages of
//     the scope for a plan. A skipped answer states that the scope contains no file of its
//     language, so the next language is asked.
//  2. The paths of the plan are locked in this process. Apply reads and seals the content
//     that the plan depends on. Commit reads the files again and refuses a plan whose files
//     changed since the preview.
//  3. A plan that creates a file or moves a file onto a path where a file is gets refused,
//     and so does a plan that [edit.Policy.Admit] refuses.
//  4. The projection is the workspace as the plan leaves it, and a gate checks it.
//  5. A dry run keeps the plan and its request under a handle for Commit. Every other call
//     writes the projection.
//
// # The gate
//
// The engine of the strongest tier that checks the language judges the projection: a type
// checker or a language server judges whether the result compiles, and a parser judges
// whether it parses. The evidence of that engine is the gate of the outcome. The gate
// refuses a change whose projection has more errors than the content it replaces:
//
//   - A gate at [trust.Resolved] counts every error of the files that it checks.
//   - A parser counts every error of a file that parsed before the change.
//   - For a file with errors before the change, a parser counts the errors on the lines
//     that the change edits. An edit elsewhere in such a file can change how the grammar
//     recovers from a fault that the file contained.
//   - The gate of a plan that moves a file leaves out each error inside an edit of another
//     file. Such an edit rewrites an import of the moved file, which a server resolves on
//     disk before the write. A [trust.CaveatPartialCheck] caveat of the gate lists them.
//
// A change is written with the status [trust.Degraded] when no engine checks its language.
//
// # Writes
//
// A write takes the lock of [Files], which one writer of the workspace takes at a time in
// every techne process. Right before it changes a file, the write reads the file again and
// compares its digest with the precondition of the plan. A file that changed refuses the
// change. A file whose projected content equals its content is not written. A write that
// stops puts back the files that it changed, and its refusal or error states whether that
// succeeded.
//
// # Dependency position
//
// Imports the standard library, core/diag, core/edit, core/engine, core/source and
// core/trust. [Router] and [Files] are ports: the language registry and the directory of the
// workspace are in other modules.
package change
