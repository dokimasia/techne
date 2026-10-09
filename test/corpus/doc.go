// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

// Package corpus drives the techne binary over large public repositories,
// one per language, and checks each answer against the files of the
// repository and against the build of its language.
//
// # Running
//
// make corpus runs TestCorpus. The build tag corpus keeps it out of the
// tests of the module and out of make check, because its first run clones
// the repositories and installs their dependencies over the network. These
// variables steer a run:
//
//   - TECHNE_CORPUS is the directory of the clones, their tools, the logs and
//     the report. It defaults to techne/corpus under the user cache
//     directory, outside this repository.
//   - TECHNE_CORPUS_ONLY limits a run to the repositories whose name or
//     language it lists, separated by commas.
//   - TECHNE_CORPUS_MANIFEST is the path of a manifest that a run reads in
//     place of corpus.json, such as one of other repositories.
//
// # Repositories
//
// corpus.json pins each repository to a tag and a commit, and names the
// commands that install its dependencies and build it. A run clones a
// repository once and marks the clone. It resets only a marked clone. A
// repository that the manifest names by path is read and never changed.
//
// # Checks
//
// A run builds cmd/techne and starts it over each repository. It waits until
// the language server settles, which is when a resolve returns total
// coverage, and then drives every tool over a sample of files:
//
//   - The declarations of an outline match the bytes of their file.
//   - A search returns an exact match first.
//   - Resolve at a use returns the declaration that the use names. The probe
//     passes over a use in a file that techne does not read: a file that an
//     ignore rule of the repository covers, such as a source that the build
//     generates under target/, and a file whose outline techne refuses.
//   - The site of each relation shows the name on its line.
//   - Rename, move, extract and document each apply to the clone, and the
//     build of the language accepts the result. The run then resets the
//     clone, and removes each source of the language that the build of the
//     change generated where an ignore rule of the repository covers it. A
//     change that techne declines is reported, and does not fail
//     the run. Neither does a change that techne refuses after its check of
//     the files on disk, once each file of the change is as it was.
//   - techne spends at most the budget of the manifest on every warm call of
//     outline, search, resolve and relations. The time of a call counts
//     without the time that techne waited for the language server, which
//     the result states in its _meta under tool.WaitedMeta.
//
// A probe addresses a declaration as an agent does: by its name and kind,
// and by the name qualified by its container when techne refuses the name
// as ambiguous. A refusal of both forms fails the run. The sample of a
// repository leaves out vendored copies of other projects and build output,
// and keeps the tests and the build code, where agents also work.
//
// The subtests of one repository run in sequence, because a run measures
// the time of each call. The repositories run one at a time, because the
// language server of a large repository takes gigabytes of memory.
//
// # Report
//
// A run writes report.md to TECHNE_CORPUS after each repository. The report
// lists the latency of each tool per repository, the time until the
// language server settled, and the outcome of each change. The output of
// every command goes to a log per repository under TECHNE_CORPUS/logs.
//
// # Dependency position
//
// Imports the standard library, core/source, tool for the types of the
// answers, and the MCP client of github.com/modelcontextprotocol/go-sdk. Its
// tests import lang and the ten language modules for the name and the
// extensions of each language.
package corpus
