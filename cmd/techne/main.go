// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Command techne serves code intelligence over MCP.
//
// It reads the workspace given as its first argument, or the working
// directory when given none, and speaks the protocol over stdin and
// stdout. Nothing but protocol traffic is written to stdout: a stray
// print there corrupts the stream, so every diagnostic goes to stderr.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.dokimi.dev/techne/internal/app"
	"go.dokimi.dev/techne/internal/version"
)

func main() {
	os.Exit(run())
}

// run returns the exit code, so main holds the one os.Exit and every
// deferred cleanup still runs.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := rootFrom(os.Args)

	// A cancelled context is how a client disconnects or an operator
	// interrupts, neither of which is a failure.
	if err := app.Run(ctx, root, version.Full()); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "techne:", err)
		return 1
	}
	return 0
}

// rootFrom reads the workspace root from the command line. Empty means
// the working directory, which is what an agent launching the server
// with no arguments gets.
func rootFrom(args []string) string {
	if len(args) > 1 {
		return args[1]
	}
	return ""
}
