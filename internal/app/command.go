// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package app

import (
	"fmt"
	"strings"

	"go.dokimi.dev/techne/presenter"
)

// Usage is the usage of techne. The command writes it to standard output for a help flag, and
// to standard error after a command line that [Parse] refuses.
const Usage = `usage: techne [-h | --help] [--version] [--structured] [--trust DIR]... [workspace]

techne serves the tools of a workspace to a client of the Model Context Protocol over
standard input and output. workspace is the root of the workspace, and the working
directory when it is omitted.

  -h, --help    write this text to standard output and exit
  --version     write the version to standard output and exit
  --structured  send each result as JSON beside its text, and declare the output schema of
                each tool, for a client that reads the JSON
  --trust DIR   let a call name a directory under DIR in its wd field, and run the call in
                the workspace of that directory; repeat it for more than one folder
`

const (
	// versionFlag is the flag of [Command.Version].
	versionFlag = "--version"
	// helpFlag and helpLetter are the flags of [Command.Help].
	helpFlag, helpLetter = "--help", "-h"
	// structuredFlag is the flag that sets [Command.Output] to [presenter.Structured].
	structuredFlag = "--structured"
	// trustFlag is the flag of each folder of [Command.Trusted]. It takes the folder as the
	// next argument, or after an equals sign.
	trustFlag = "--trust"
)

// Command is what a command line asks of techne.
type Command struct {
	// Root is the root of the workspace as the command line gives it, and the empty string
	// for the working directory.
	Root string

	// Trusted are the folders under which a call can set the directory to run in, as the
	// command line gives them.
	Trusted []string

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
// --version, --structured and --trust, for a --trust without a folder, and for a second
// workspace. A folder that starts with a hyphen follows an equals sign.
func Parse(args []string) (Command, error) {
	var out Command
	rooted, trusting := false, false
	for _, arg := range args {
		folder, joined := strings.CutPrefix(arg, trustFlag+"=")
		switch {
		case arg == helpFlag || arg == helpLetter:
			return Command{Help: true}, nil
		case trusting && !strings.HasPrefix(arg, "-"):
			out.Trusted, trusting = append(out.Trusted, arg), false
		case trusting, joined && folder == "":
			return Command{}, fmt.Errorf("app: %s takes a folder", trustFlag)
		case joined:
			out.Trusted = append(out.Trusted, folder)
		case arg == trustFlag:
			trusting = true
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
	if trusting {
		return Command{}, fmt.Errorf("app: %s takes a folder", trustFlag)
	}
	return out, nil
}
