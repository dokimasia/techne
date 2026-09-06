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

// For builds the engines a workspace supports for one language, in the
// order a catalogue should meet them.
//
// A declaration with no server named registers a parser alone, and so
// does a workspace that is not on disk. Neither is a fault: the first is
// a language nobody has written a server declaration for, and the second
// is a tree that a process cannot open by name.
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

	served, declared, err := Serving(w, d, s)
	if err != nil {
		return nil, err
	}
	if declared {
		out = append(out, served)
	}
	// Last, so an engine a module brings of its own meets a catalogue
	// after the server: two engines claiming one tier are ordered by
	// where they were registered, and the server is the one that is
	// warm.
	return append(out, also...), nil
}

// Serving returns the server engine a workspace supports for one
// language, and whether it has one at all.
//
// It has none when the module declared no server, and none when the
// workspace is not on disk: a server is a process that opens files by
// name, and one pointed at a tree that was never written opens nothing
// and is answered about nothing.
//
// A server that is not installed still counts. Left out, a caller
// concludes the language cannot be served; declared, it reports through
// [lsp.Engine.Available] what to install, which is a different problem
// and a fixable one.
func Serving(w lang.Workspace, d lang.Declaration, s lsp.Server) (engine.Engine, bool, error) {
	if s.Name == "" || !w.OnDisk() {
		return nil, false, nil
	}
	served, err := lsp.New(w.Root, d, s)
	if err != nil {
		return nil, false, fmt.Errorf("engines: %q: %w", d.Language, err)
	}
	return served, true, nil
}

// Register adds a language to a registry and its engines to a catalogue.
//
// A language module's whole entry point. Nothing is registered when any
// part of it fails, because [lang.Registry.Register] checks the
// declaration before it touches the catalogue.
//
// A module with an engine of its own passes it last. Go has one: an
// in-process type checker, which answers what a server answers on a
// machine where no server is installed.
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
