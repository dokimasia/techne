// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app

import (
	"fmt"
	"strings"

	"go.dokimi.dev/techne/presenter"
)

// Usage is the usage of techne. The command writes it to standard output for a help flag, and
// to standard error after a command line that [Parse] refuses.
const Usage = `usage: techne [-h | --help] [--version] [--structured] [workspace]

techne serves the tools of a workspace to a client of the Model Context Protocol over
standard input and output. workspace is the root of the workspace, and the working
directory when it is omitted.

  -h, --help    write this text to standard output and exit
  --version     write the version to standard output and exit
  --structured  send each result as JSON beside its text, and declare the output schema of
                each tool, for a client that reads the JSON
`

const (
	// versionFlag is the flag of [Command.Version].
	versionFlag = "--version"
	// helpFlag and helpLetter are the flags of [Command.Help].
	helpFlag, helpLetter = "--help", "-h"
	// structuredFlag is the flag that sets [Command.Output] to [presenter.Structured].
	structuredFlag = "--structured"
)

// Command is what a command line asks of techne.
type Command struct {
	// Root is the root of the workspace as the command line gives it, and the empty string
	// for the working directory.
	Root string

	// Output is what the result of a call contains: [presenter.Text] by default.
	Output presenter.Output

	// Version asks for the version alone.
	Version bool

	// Help asks for the usage alone.
	Help bool
}

// Parse returns the command of args, the arguments that follow the name of the program. It
// reads the arguments in order, and a help flag returns the command of the usage without
// reading the arguments after it. Parse returns an error for a flag other than the help flags,
// --version and --structured, and for a second workspace.
func Parse(args []string) (Command, error) {
	var out Command
	rooted := false
	for _, arg := range args {
		switch {
		case arg == helpFlag || arg == helpLetter:
			return Command{Help: true}, nil
		case arg == versionFlag:
			out.Version = true
		case arg == structuredFlag:
			out.Output = presenter.Structured
		case strings.HasPrefix(arg, "-"):
			return Command{}, fmt.Errorf("app: the command does not take the flag %q", arg)
		case rooted:
			return Command{}, fmt.Errorf("app: the command takes one workspace, and %q is a second", arg)
		default:
			out.Root, rooted = arg, true
		}
	}
	return out, nil
}
