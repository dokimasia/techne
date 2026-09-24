// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

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
	os.Exit(run(os.Args[1:]))
}

// run runs the command line args and returns the exit status. main exits with it after the
// deferred calls of run have returned.
func run(args []string) int {
	command, err := app.Parse(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "techne: %v\n%s", err, app.Usage)
		return 2
	}
	if command.Version {
		fmt.Println(version.Full())
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// A signal ends the session, and the error of Run for it is ctx.Err().
	if err := app.Run(ctx, command.Root, version.Full()); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "techne:", err)
		return 1
	}
	return 0
}
