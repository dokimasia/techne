// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
)

// CapabilitiesInput narrows the report.
//
// Language keeps one language's entries. Empty reports every language
// served, which is what a caller in a mixed repository wants.
type CapabilitiesInput struct {
	Language string `json:"language,omitempty" jsonschema:"one language, or omit for all"`
}

// CapabilitiesOutput is what the catalogue can answer.
type CapabilitiesOutput struct {
	Items []Capability `json:"items"`
}

// Capability is one language and role an engine serves.
//
// It carries no coverage claim. What an engine will cover depends on the
// scope it is asked about and on how far its index has got, so it is a
// property of an answer rather than of an engine.
type Capability struct {
	Language string `json:"language"`
	Role     string `json:"role"`
	Engine   string `json:"engine,omitempty"`
	Fidelity string `json:"fidelity"`
	Cost     string `json:"cost"`

	// Available reports whether the engine can run now. A missing
	// server is a different problem from a missing capability, so an
	// engine that cannot run is reported rather than omitted.
	Available   bool   `json:"available"`
	Unavailable string `json:"unavailable,omitempty"`
}

// Render writes the report for a reader rather than a parser.
//
// One line per engine rather than per role, because an engine serving
// four roles is one fact about the system and four lines of it read as
// four.
func (o CapabilitiesOutput) Render() string {
	type held struct {
		language, engine, fidelity, cost, unavailable string
		roles                                         []string
	}
	var order []string
	by := map[string]*held{}
	for _, c := range o.Items {
		key := c.Language + "\x00" + c.Engine
		if _, seen := by[key]; !seen {
			by[key] = &held{
				language: c.Language, engine: c.Engine, fidelity: c.Fidelity,
				cost: c.Cost, unavailable: c.Unavailable,
			}
			order = append(order, key)
		}
		by[key].roles = append(by[key].roles, c.Role)
	}

	var b strings.Builder
	if len(order) == 0 {
		return "nothing is served\n"
	}
	for _, key := range order {
		one := by[key]
		fmt.Fprintf(&b, "%-12s %-22s %-10s %-8s %s\n",
			one.language, one.engine, one.fidelity, one.cost, strings.Join(one.roles, " "))
		if one.unavailable != "" {
			// What would make it available, so a caller can act rather
			// than only route around.
			fmt.Fprintf(&b, "%-12s %s\n", "", one.unavailable)
		}
	}
	return b.String()
}

// Capabilities builds the tool that reports what the system can answer.
func Capabilities(c *engine.Catalog) (Tool, error) {
	return New("capabilities", capabilitiesDescription,
		func(ctx context.Context, in CapabilitiesInput) (CapabilitiesOutput, error) {
			wanted := source.Language(in.Language)

			out := CapabilitiesOutput{Items: []Capability{}}
			for _, capability := range c.Capabilities(ctx) {
				if wanted != "" && capability.Language != wanted {
					continue
				}
				out.Items = append(out.Items, Capability{
					Language:    string(capability.Language),
					Role:        capability.Role.String(),
					Engine:      capability.Engine,
					Fidelity:    capability.Fidelity.String(),
					Cost:        capability.Cost.String(),
					Available:   capability.Available,
					Unavailable: capability.Unavailable,
				})
			}
			return out, nil
		})
}

const capabilitiesDescription = "PREFER OVER guessing from the tool list what this server can do. " +
	"Reports, per language and role, which engine would answer, how strong its evidence is, " +
	"what it costs, and whether it can run at all. An operation missing here is one no engine " +
	"serves, which is different from one whose language server is not installed."
