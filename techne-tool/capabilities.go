// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/source"
)

// CapabilitiesInput is the input of the capabilities tool. Language keeps the capabilities
// of one language, and the empty language keeps every language.
type CapabilitiesInput struct {
	Language string `json:"language,omitempty" jsonschema:"one language, or every language when omitted"`
}

// CapabilitiesOutput is the output of the capabilities tool.
type CapabilitiesOutput struct {
	Items []Capability `json:"items"`
}

// Capability is one role that one engine serves for one language. It has no completeness,
// because the completeness of an answer depends on its scope and on the state of the index.
type Capability struct {
	Language string `json:"language"`
	Role     string `json:"role"`
	Engine   string `json:"engine,omitempty"`
	Fidelity string `json:"fidelity"`
	Cost     string `json:"cost"`

	// Available reports whether the engine can run now. The output keeps an engine that
	// cannot run, with the reason in Unavailable.
	Available   bool   `json:"available"`
	Unavailable string `json:"unavailable,omitempty"`
}

// Render returns the capabilities as text: one line per language and engine, with the roles
// of the engine, and the reason under an engine that cannot run.
func (o CapabilitiesOutput) Render() string {
	type row struct {
		language, engine, fidelity, cost, unavailable string
		roles                                         []string
	}
	var order []string
	rows := map[string]*row{}
	for _, c := range o.Items {
		key := c.Language + "\x00" + c.Engine
		if _, seen := rows[key]; !seen {
			rows[key] = &row{
				language: c.Language, engine: c.Engine, fidelity: c.Fidelity,
				cost: c.Cost, unavailable: c.Unavailable,
			}
			order = append(order, key)
		}
		rows[key].roles = append(rows[key].roles, c.Role)
	}

	if len(order) == 0 {
		return "nothing is served\n"
	}
	var b strings.Builder
	for _, key := range order {
		one := rows[key]
		fmt.Fprintf(&b, "%-12s %-22s %-10s %-8s %s\n",
			one.language, one.engine, one.fidelity, one.cost, strings.Join(one.roles, " "))
		if one.unavailable != "" {
			fmt.Fprintf(&b, "%-12s %s\n", "", one.unavailable)
		}
	}
	return b.String()
}

// Capabilities returns the tool that reports the roles that the engines of each language
// serve, in the order of the catalogue.
func Capabilities(catalogue Catalogue) (Tool, error) {
	return New("capabilities", capabilitiesDescription,
		func(ctx context.Context, in CapabilitiesInput) (CapabilitiesOutput, error) {
			wanted := source.Language(in.Language)

			out := CapabilitiesOutput{Items: []Capability{}}
			for _, capability := range catalogue.Capabilities(ctx) {
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
	"It reports, per language and role, the engine that serves it, the strength of its " +
	"evidence, its cost, and whether it can run. An operation missing here is one that no " +
	"engine serves, which differs from one whose language server is not installed."
