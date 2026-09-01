// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"

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
