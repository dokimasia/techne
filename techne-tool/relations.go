// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"context"
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// RelationsInput is what an agent sends to the relations tool.
type RelationsInput struct {
	Scope     string `json:"scope"                        jsonschema:"file or directory, workspace-relative"`
	Name      string `json:"name"                         jsonschema:"the declaration to ask about"`
	Relation  string `json:"relation"                     jsonschema:"calls|called-by|implements|implemented-by|references|referenced-by|imports|imported-by|embeds|embedded-by"`
	Kind      string `json:"kind,omitempty"               jsonschema:"narrows an ambiguous name to one kind"`
	Language  string `json:"language,omitempty"           jsonschema:"language to assume"`
	Limit     int    `json:"limit,omitempty"              jsonschema:"cap the edges returned"`
	Preferred string `json:"preferred_fidelity,omitempty" jsonschema:"syntactic|indexed|resolved"`
}

// RelationsOutput is what the relations tool returns.
//
// The declaration asked about is stated once, in Of, rather than on each
// of its callers. The direction is stated once too: every edge in an
// answer runs the way the caller asked, whichever way an engine happened
// to store it.
type RelationsOutput struct {
	Scope      Scope       `json:"scope"`
	Of         string      `json:"of"`
	Relation   string      `json:"relation"`
	Items      []Connected `json:"items"`
	Provenance Provenance  `json:"provenance"`
	Error      *Failure    `json:"error,omitempty"`
}

// Connected is one declaration at the far end of an edge.
//
// Path and Line are where the edge was written rather than where the
// declaration is. A caller asking who calls this navigates to the call;
// the declaration is named, and finding it is what outline is for.
type Connected struct {
	Name string    `json:"name"`
	Kind sema.Kind `json:"kind"`
	// In is the declaration this one sits inside, empty at the top level.
	In   string `json:"in,omitempty"`
	Path string `json:"path"`
	Line int    `json:"line"`
	// Via is the source line the edge was written on, so a caller reads
	// the call rather than spending a turn per caller fetching it.
	Via string `json:"via,omitempty"`
}

// Failed reports whether a caller should read this as a failure.
func (o RelationsOutput) Failed() bool { return o.Error != nil }

// Render writes the edges for a reader rather than a parser.
func (o RelationsOutput) Render() string {
	var b strings.Builder
	if o.Error != nil {
		fmt.Fprintf(&b, "%s %s — %s\n%s\n", o.Relation, o.Of, o.Error.Code, o.Error.Reason)
		return b.String()
	}

	fmt.Fprintf(&b, "%s %s — %s\n", o.Relation, o.Of, plural(len(o.Items), "site", "sites"))
	for _, one := range o.Items {
		fmt.Fprintf(&b, "\n%s:%d", one.Path, one.Line)
		// The far end is named unless it is the file the site is in,
		// which an import edge makes it: "a.go:7 in a.go" states the
		// same fact twice and reads as a mistake.
		if one.Name != "" && one.Name != one.Path {
			fmt.Fprintf(&b, "  in %s", qualify(one.In, one.Name))
		}
		b.WriteString("\n")
		if one.Via != "" {
			fmt.Fprintf(&b, "  %s\n", strings.TrimSpace(one.Via))
		}
	}

	b.WriteString("\n")
	b.WriteString(evidence(o.Provenance))
	return b.String()
}

// qualify writes a declaration as its language would, so a reader
// recognises it.
func qualify(in, name string) string {
	if in == "" {
		return name
	}
	return in + "." + name
}

// Relations builds the tool that reports how a declaration connects.
func Relations(reads Outliner, relates Relator) (Tool, error) {
	return New("relations", relationsDescription,
		func(ctx context.Context, in RelationsInput) (RelationsOutput, error) {
			scope, err := relative(in.Scope)
			if err != nil {
				return RelationsOutput{}, err
			}
			kind, known := relationOf(in.Relation)
			if !known {
				return relationsRefused(scope, in, fmt.Sprintf(
					"no relation is called %q; the directions are %s",
					in.Relation, strings.Join(relationNames(), ", "))), nil
			}

			req := engine.Request{
				Scope:     scope,
				Language:  source.Language(in.Language),
				Preferred: fidelity(in.Preferred),
			}
			of, failure := addressed(ctx, reads, req, scope, in.Name, kindOf(in.Kind))
			if failure != nil {
				out := relationsRefused(scope, in, failure.Reason)
				out.Error.Code = failure.Code
				return out, nil
			}

			answered, err := relates.Relate(ctx, req, of.ID, kind)
			if err != nil {
				return RelationsOutput{}, err
			}

			out := RelationsOutput{
				Scope:      about(scope, in.Language, engine.Answer[sema.Symbol]{}),
				Of:         in.Name,
				Relation:   kind.String(),
				Items:      connected(answered.Items, in.Limit),
				Provenance: provenance(answered.Provenance),
			}
			out.Scope.Language = string(of.Language)
			if !answered.Status.Answered() {
				out.Error = &Failure{
					Code:   answered.Status.String(),
					Reason: reasonFrom(out.Provenance.Caveats),
				}
			}
			return out, nil
		})
}

// connected turns the edges an engine found into what a caller reads,
// and stops at the cap the caller set.
func connected(found []sema.Relation, limit int) []Connected {
	out := []Connected{}
	for _, edge := range found {
		if limit > 0 && len(out) == limit {
			break
		}
		out = append(out, Connected{
			Name: edge.To.Name,
			Kind: edge.To.Kind,
			In:   parentName(edge.To),
			Path: string(edge.At.Path),
			Line: edge.At.Start.Line + 1,
			Via:  edge.Via,
		})
	}
	return out
}

// parentName reads the container out of a declaration's parent identity.
//
// An identity is language, unit, name and kind joined, so the name is in
// there and the alternative is a lookup per edge for something the
// engine already stated.
func parentName(s sema.Symbol) string {
	if s.Parent == "" {
		return ""
	}
	held := string(s.Parent)
	from := strings.LastIndex(held, "#")
	if from < 0 {
		return ""
	}
	name := held[from+1:]
	if to := strings.LastIndex(name, ":"); to >= 0 {
		name = name[:to]
	}
	return name
}

// relationOf reads the direction a caller asked for.
func relationOf(name string) (sema.RelationKind, bool) {
	for _, kind := range sema.RelationKinds() {
		if kind.String() == name {
			return kind, true
		}
	}
	return sema.RelationUnknown, false
}

// relationNames lists the directions, for a caller that named none of
// them.
func relationNames() []string {
	out := make([]string, 0, len(sema.RelationKinds()))
	for _, kind := range sema.RelationKinds() {
		out = append(out, kind.String())
	}
	return out
}

// relationsRefused is the answer when the request never reached an
// engine.
func relationsRefused(scope source.Path, in RelationsInput, why string) RelationsOutput {
	return RelationsOutput{
		Scope:    about(scope, in.Language, engine.Answer[sema.Symbol]{}),
		Of:       in.Name,
		Relation: in.Relation,
		Items:    []Connected{},
		Error:    &Failure{Code: trust.Refused.String(), Reason: why},
	}
}

const relationsDescription = "PREFER OVER grep for finding what calls, implements or " +
	"references a declaration. Returns each edge with the line it was written on, so a " +
	"caller reads the call rather than fetching it, and states whether an empty answer " +
	"means there are none or only that none were found."
