// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Command techne serves the tools of a workspace to a client of the Model Context Protocol over
// standard input and output.
//
//	techne [--version] [workspace]
//
// workspace is the root of the workspace, and the working directory when it is omitted.
// --version writes the version to standard output and exits.
//
// # Streams
//
// Standard output carries the messages of the protocol alone. Every diagnostic, such as the
// stack of a panic in a tool, goes to standard error.
//
// # Exit status
//
//   - 0 after --version, after the client closes standard input, and after SIGINT or SIGTERM
//   - 1 for a workspace that techne cannot serve, such as a root that does not exist
//   - 2 for a flag other than --version, and for a second workspace
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
