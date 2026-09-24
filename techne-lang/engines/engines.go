// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package engines

import (
	"fmt"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/treesitter"
)

// For returns the engines of one language over a workspace, in the order in which a catalogue
// meets them: the tree-sitter engine, the server engine when [Serving] returns one, and then
// the engines of also. A catalogue keeps that order for two engines of equal fidelity and
// cost, so a server engine comes before an engine of the module.
//
// It returns the error of [treesitter.New] and of [Serving].
func For(
	w lang.Workspace,
	d lang.Declaration,
	g treesitter.Grammar,
	s lsp.Server,
	also ...engine.Engine,
) ([]engine.Engine, error) {
	parser, err := treesitter.New(w.FS, d, g)
	if err != nil {
		return nil, err
	}
	out := []engine.Engine{parser}

	served, declared, err := Serving(w, d, s, parser)
	if err != nil {
		return nil, err
	}
	if declared {
		out = append(out, served)
	}
	return append(out, also...), nil
}

// Serving returns the server engine of one language over a workspace, and reports whether the
// workspace has one. It has none when s names no server, and none when the workspace is not on
// disk, because a server opens files by name. A server that is not on PATH still has an
// engine, and its [lsp.Engine.Available] names the program to install.
//
// declarations is the outline engine of the language. The server engine reads the
// declarations of a file through it, so the server does not open the file.
//
// It returns the error of [lsp.New] for a server declaration that is not valid.
func Serving(
	w lang.Workspace,
	d lang.Declaration,
	s lsp.Server,
	declarations engine.Outliner,
) (engine.Engine, bool, error) {
	if s.Name == "" || !w.OnDisk() {
		return nil, false, nil
	}
	served, err := lsp.New(w.Root, d, s, declarations)
	if err != nil {
		return nil, false, fmt.Errorf("engines: %q: %w", d.Language, err)
	}
	return served, true, nil
}

// Register adds the language that d declares to r and its engines to c, from [For]. A module
// passes an engine of its own in also, as the Go module passes its type checker.
//
// It registers nothing when an engine fails to build, and [lang.Registry.Register] checks the
// declaration before it changes the catalogue.
func Register(
	w lang.Workspace,
	r *lang.Registry,
	c *engine.Catalog,
	d lang.Declaration,
	g treesitter.Grammar,
	s lsp.Server,
	also ...engine.Engine,
) error {
	built, err := For(w, d, g, s, also...)
	if err != nil {
		return err
	}
	return r.Register(c, d, built...)
}
