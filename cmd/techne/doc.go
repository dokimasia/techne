// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

// Command techne serves the tools of a workspace to a client of the Model Context Protocol over
// standard input and output.
//
//	techne [-h | --help] [--version] [--structured] [--trust DIR]... [workspace]
//
// workspace is the root of the workspace, and the working directory when it is omitted. -h and
// --help write the usage to standard output and exit. --version writes the version to standard
// output and exits. --structured sends each result as JSON beside its text.
//
// # Trusted folders
//
// --trust DIR, or --trust=DIR, adds DIR to the trusted folders, and the flag repeats for more
// than one. With a trusted folder, every tool takes the field wd: a directory under a trusted
// folder, absolute or relative to the workspace root. A call that sets wd runs in the workspace
// of that directory, and the paths of the call and of its answer are relative to it. A leading ~
// in a folder or a directory is the home directory of the user, because an editor starts techne
// without a shell to expand it.
//
// # Streams
//
// While techne serves a workspace, standard output contains the messages of the protocol
// alone. Every diagnostic, such as the stack of a panic in a tool, goes to standard error.
//
// # Exit status
//
//   - 0 after a help flag or --version, after the client closes standard input, and after
//     SIGINT or SIGTERM
//   - 1 for a workspace that techne cannot serve, such as a root that does not exist, and for a
//     trusted folder that does not exist or is not a directory
//   - 2 for a flag other than -h, --help, --version, --structured and --trust, for a --trust
//     without a folder, and for a second workspace
//
// # Environment
//
// TECHNE_MOCK registers mock languages beside the ten languages. A mock language serves every
// role at the tiers of its entry:
//
//	TECHNE_MOCK=1                       one language, named mock
//	TECHNE_MOCK=alpha,beta              two languages, of the extensions .alpha and .beta
//	TECHNE_MOCK=alpha@syntactic         a language at the syntactic tier
//	TECHNE_MOCK=alpha@resolved/partial  a language at the resolved tier with partial answers
//
// A tier or a completeness outside the vocabularies of trust ends the command with status 1.
package main
