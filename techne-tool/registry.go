// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"fmt"
	"strings"
)

// maxNameLength is the longest tool name that the Model Context Protocol allows.
const maxNameLength = 128

// Registry is the set of tools that a server offers. A composition root adds the tools before
// the server serves a request, so the registry is safe for concurrent reads after that.
type Registry struct {
	tools []Tool
	named map[string]bool
}

// NewRegistry returns a registry without tools.
func NewRegistry() *Registry {
	return &Registry{named: map[string]bool{}}
}

// Add adds t to the registry. It returns an error for a name that another tool of the
// registry has, and for a name that [validName] refuses.
func (r *Registry) Add(t Tool) error {
	name := t.Name()
	if err := validName(name); err != nil {
		return err
	}
	if r.named[name] {
		return fmt.Errorf("tool: %q is already registered", name)
	}
	r.named[name] = true
	r.tools = append(r.tools, t)
	return nil
}

// Tools returns every tool of the registry, in the order in which they were added.
func (r *Registry) Tools() []Tool {
	out := make([]Tool, len(r.tools))
	copy(out, r.tools)
	return out
}

// validName returns an error for a name that the Model Context Protocol does not allow: a
// name that is empty, longer than [maxNameLength] characters, or contains a character other
// than an ASCII letter, a digit, an underscore, a hyphen or a dot.
func validName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("tool: a tool must be named")
	case len(name) > maxNameLength:
		return fmt.Errorf("tool: %q is %d characters, over the %d the protocol allows",
			name, len(name), maxNameLength)
	}
	if i := strings.IndexFunc(name, notAllowed); i >= 0 {
		return fmt.Errorf("tool: %q contains %q, which the protocol does not allow in a name",
			name, name[i:i+1])
	}
	return nil
}

// notAllowed reports whether r is a character that a tool name cannot contain.
func notAllowed(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	case r == '_', r == '-', r == '.':
		return false
	default:
		return true
	}
}
