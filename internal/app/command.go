// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app

import (
	"fmt"
	"strings"
)

// Usage is the text that the command writes to standard error for a command line that
// [Parse] refuses.
const Usage = `usage: techne [--version] [workspace]

techne serves the tools of a workspace to a client of the Model Context Protocol over
standard input and output. workspace is the root of the workspace, and the working
directory when it is omitted.

  --version  write the version to standard output and exit
`

// versionFlag is the flag of [Command.Version].
const versionFlag = "--version"

// Command is what a command line asks of techne.
type Command struct {
	// Root is the root of the workspace as the command line gives it, and the empty string
	// for the working directory.
	Root string

	// Version asks for the version alone.
	Version bool
}

// Parse returns the command of args, the arguments that follow the name of the program. It
// returns an error for a flag other than --version and for a second workspace.
func Parse(args []string) (Command, error) {
	var out Command
	rooted := false
	for _, arg := range args {
		switch {
		case arg == versionFlag:
			out.Version = true
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
