// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"fmt"
	"strings"
)

// maxNameLength is what the protocol asks of a tool name. A client that
// rejects a name drops the tool, and the server then looks as though it
// never offered it.
const maxNameLength = 128

// Registry holds the tools a server offers.
//
// A composition root builds one and then serves requests; nothing adds a
// tool afterwards, so it is safe for concurrent reads.
type Registry struct {
	tools []Tool
	named map[string]bool
}

// NewRegistry returns a registry holding no tools.
func NewRegistry() *Registry {
	return &Registry{named: map[string]bool{}}
}

// Add registers a tool.
//
// It refuses a name already taken, because a name is what an agent
// routes on and two tools sharing one make the choice undefined. It also
// refuses a name outside what the protocol allows, so an unusable tool
// is caught here rather than dropped by a client.
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

// notAllowed reports whether a rune may not appear in a tool name.
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

// Tools returns every registered tool, in registration order.
//
// The order is stable so a client can cache the list; one that reordered
// itself would invalidate that cache for no reason.
func (r *Registry) Tools() []Tool {
	out := make([]Tool, len(r.tools))
	copy(out, r.tools)
	return out
}

// validName reports why a name cannot be offered, or nil.
//
// The protocol allows ASCII letters, digits, underscore, hyphen and dot,
// between 1 and 128 characters. The dot is what family.subject relies
// on.
func validName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("tool: a tool must be named")
	case len(name) > maxNameLength:
		return fmt.Errorf("tool: %q is %d characters, over the %d the protocol allows",
			name, len(name), maxNameLength)
	}
	if i := strings.IndexFunc(name, notAllowed); i >= 0 {
		return fmt.Errorf("tool: %q holds %q, which the protocol does not allow in a name",
			name, name[i:i+1])
	}
	return nil
}
