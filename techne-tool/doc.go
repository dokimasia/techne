// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package tool defines the tools that techne offers an agent: what each tool takes, what it
// returns, and how an answer fits the context of a model.
//
// A transport calls a [Tool] through [Tool.Execute] and sends its [Result]. A tool takes the
// services that it calls as the interfaces of this package, such as [Outliner] and [Writer],
// and a composition root adds the tools to a [Registry].
//
// # Tools
//
//   - Read: [Outline], [Search], [Resolve], [Relations], [Verify] and [Capabilities].
//   - Write: [Document], [Rename], [Move] and [Extract], which preview a change unless the
//     caller sets dry_run to false, and [Apply], which writes a preview by its handle.
//
// The description of each tool starts with PREFER OVER and names the built-in operation that
// the tool replaces. [New] derives the schemas of a tool from the types of its handler.
//
// # Answers
//
// The outline, search and resolve tools return an [Answer], and the relations and verify tools
// an output of the same parts: the [Scope], the items, the [Provenance], and a [Failure] when
// the request was not served. A result contains the output twice: as JSON for a program, and
// as the text of its [Renderer] for a model.
//
// # Words
//
// The fields kind, detail, include, relation and preferred_fidelity take a word of a closed
// vocabulary. The schema of each field lists the words of the vocabulary of core, and a tool
// refuses any other word with a Failure that lists them.
//
// # Addressing
//
// A tool that takes the name of a declaration finds the declaration in the outline of its
// scope. A name matches a declaration of that name or of that qualified name: the names of
// its containers and its own name, joined by dots as
// [go.dokimi.dev/techne/core/sema.Qualify] joins them. A kind narrows the match. Two or more
// matches are one declaration when they are imports of one name, a type and its own
// constructors, or declarations of one ID. The tool refuses any other set of matches, and the
// reason lists the kind and the site of each.
//
// # Budget
//
// [Detail] selects the fields of each declaration, and [Fit] thins the answer to the token
// budget of the caller: the documentation first, then the source text, then the bindings,
// the members and the unexported declarations, and last the declarations at the end. A
// truncation caveat counts what Fit removed. The relations tool keeps the first relations whose
// render fits, and every cap adds a truncation caveat.
//
// # Dependency position
//
// Imports the standard library, core/diag, core/edit, core/engine, core/sema, core/source,
// core/trust and github.com/google/jsonschema-go.
package tool
