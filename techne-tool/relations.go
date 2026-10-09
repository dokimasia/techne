// Copyright Dokimasia B.V. 2026
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

// DefaultRelations is the number of relations that the relations tool returns to a caller
// that names no limit.
const DefaultRelations = 50

// RelationsInput is the input of the relations tool.
type RelationsInput struct {
	Scope     string       `json:"scope"                        jsonschema:"file or directory of the declaration, relative to the root"`
	Name      string       `json:"name"                         jsonschema:"the declaration, qualified as Type.Method when ambiguous"                alias:"symbol"`
	Relation  RelationWord `json:"relation"                     jsonschema:"the direction of the relations"`
	Kind      KindWord     `json:"kind,omitempty"               jsonschema:"its kind, such as function, method or struct, when the name has several"`
	Language  string       `json:"language,omitempty"           jsonschema:"a language to ask instead of the scope's"`
	Preferred FidelityWord `json:"preferred_fidelity,omitempty" jsonschema:"weakest evidence wanted; the answer states its own fidelity"`
	Line      int          `json:"line,omitempty"               jsonschema:"a line of it, counted from one, when the name has several declarations"`
	Limit     int          `json:"limit,omitempty"              jsonschema:"most relations to return, 50 by default"`
	MaxTokens int          `json:"max_tokens,omitempty"         jsonschema:"answer ceiling in tokens, 6000 by default"`
}

// RelationsOutput is the output of the relations tool. It states the declaration and the
// direction once, and every relation runs in that direction.
type RelationsOutput struct {
	Error      *Failure    `json:"error,omitempty"`
	Scope      Scope       `json:"scope"`
	Of         string      `json:"of"`
	Relation   string      `json:"relation"`
	Items      []Connected `json:"items"`
	Provenance Provenance  `json:"provenance"`
}

// Connected is one relation: the declaration at its far end, and its site. Path and Line are
// the site of the relation, such as a call, and not the declaration.
type Connected struct {
	Name string `json:"name"`
	// In is the qualified name of the declaration that contains the far end, or empty at the
	// top level of a file.
	In   string `json:"in,omitempty"`
	Path string `json:"path"`
	// Via is the source line of the site.
	Via  string `json:"via,omitempty"`
	Line int    `json:"line"`
	// Column is the column of the site in bytes, counted from one, which tells apart two sites
	// on one line, as the opening and the closing tag of a JSX element are.
	Column int       `json:"column"`
	Kind   sema.Kind `json:"kind"`
}

// Failed reports whether the output has an Error.
func (o RelationsOutput) Failed() bool { return o.Error != nil }

// Render returns the relations as text: a heading, the site of each relation with the
// declaration at its far end and the line of the site, and the evidence.
func (o RelationsOutput) Render() string {
	var b strings.Builder
	if o.Error != nil {
		fmt.Fprintf(&b, "%s %s — %s\n%s\n", o.Relation, o.Of, o.Error.Code, o.Error.Reason)
		return b.String()
	}
	b.WriteString(o.heading(len(o.Items)))
	for _, one := range o.Items {
		b.WriteString(one.render())
	}
	b.WriteString("\n")
	b.WriteString(evidence(o.Provenance, len(o.Items) == 0))
	return b.String()
}

// heading returns the first line of the render of o with n relations.
func (o RelationsOutput) heading(n int) string {
	return fmt.Sprintf("%s %s — %s\n", o.Relation, o.Of, plural(n, "site", "sites"))
}

// render returns the lines of the render of c. The far end is left out when it is the file of
// the site, as the far end of an import is.
func (c Connected) render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s:%d:%d", c.Path, c.Line, c.Column)
	if c.Name != "" && c.Name != c.Path {
		fmt.Fprintf(&b, "  in %s", sema.Qualify(c.In, c.Name))
	}
	b.WriteString("\n")
	if c.Via != "" {
		fmt.Fprintf(&b, "  %s\n", strings.TrimSpace(c.Via))
	}
	return b.String()
}

// Relations returns the tool that reports the relations of one declaration in one direction.
// It finds the declaration by the rule of [addressed], and asks the language of the
// declaration with its span, the limit of the input and [DefaultRelations] for none. It asks
// with the test files, because a test that imports, calls or references a declaration uses it.
// It fits the output to the budget with the run of relations that [longest] finds.
func Relations(reads Outliner, relates Relator) (Tool, error) {
	return New("relations", relationsDescription,
		func(ctx context.Context, in RelationsInput) (RelationsOutput, error) {
			scope, failure := relative(in.Scope)
			if failure != nil {
				return relationsRefused("", in, failure), nil
			}
			kind, byRelation := relationOf(in.Relation)
			declaredKind, byKind := kindOf(in.Kind)
			preferred, byFidelity := fidelityOf(in.Preferred)
			if failure = first(byRelation, byKind, byFidelity); failure != nil {
				return relationsRefused(scope, in, failure), nil
			}

			limit := in.Limit
			if limit <= 0 {
				limit = DefaultRelations
			}
			req := engine.Request{
				Scope:     scope,
				Language:  source.Language(in.Language),
				Preferred: preferred,
				Limit:     limit,
				Tests:     true,
			}
			of, failure := addressed(ctx, reads, req, scope, in.Name, declaredKind, in.Line)
			switch {
			case failure != nil && kind == sema.ImportedBy:
				// An import names a module or a package, which declares no name of its own in
				// most languages, so the engine matches the name as written.
				of = sema.Symbol{ID: sema.NewID(req.Language, scope, in.Name, declaredKind), Name: in.Name}
			case failure != nil:
				return relationsRefused(scope, in, failure), nil
			}

			req.Language, req.Declared = of.Language, of.Span
			answered, err := relates.Relate(ctx, req, of.ID, kind)
			if err != nil {
				return RelationsOutput{}, err
			}

			out := RelationsOutput{
				Scope:      about(scope, string(of.Language), engine.Answer[sema.Symbol]{}),
				Of:         in.Name,
				Relation:   kind.String(),
				Items:      connected(answered.Items, limit),
				Provenance: provenance(answered.Provenance),
			}
			if len(out.Items) < len(answered.Items) {
				out.Provenance.Caveats = append(out.Provenance.Caveats, Caveat{
					Code: string(trust.CaveatTruncated),
					Note: fmt.Sprintf("%d of %d relations returned", len(out.Items), len(answered.Items)),
				})
			}
			if !answered.Status.Answered() {
				out.Error = &Failure{
					Code:   answered.Status.String(),
					Reason: reasonFrom(out.Provenance.Caveats),
				}
			}
			return out.fitted(Budget{MaxTokens: in.MaxTokens}), nil
		})
}

// connected returns the first limit relations of found, each as a caller reads it.
func connected(found []sema.Relation, limit int) []Connected {
	out := []Connected{}
	for _, edge := range found {
		if len(out) == limit {
			break
		}
		out = append(out, Connected{
			Name:   edge.To.Name,
			Kind:   edge.To.Kind,
			In:     container(edge.To),
			Path:   string(edge.At.Path),
			Line:   edge.At.Start.Line + 1,
			Column: edge.At.Start.Column + 1,
			Via:    edge.Via,
		})
	}
	return out
}

// container returns the qualified name of the declaration that contains s: the name of the
// parent of s, or for a method without a parent the qualifier of its qualified name, as the
// receiver qualifies a Go method.
func container(s sema.Symbol) string {
	switch {
	case s.Parent != "":
		return s.Parent.Name()
	case s.Kind == sema.KindMethod:
		if qualifier, cut := strings.CutSuffix(s.ID.Name(), "."+s.Name); cut {
			return qualifier
		}
	}
	return ""
}

// fitted returns o with the longest run of its first relations whose render fits b, at least
// one, and a truncation caveat when it leaves any out.
func (o RelationsOutput) fitted(b Budget) RelationsOutput {
	if o.Error != nil || len(o.Items) == 0 {
		return o
	}
	ceiling := b.MaxTokens
	if ceiling <= 0 {
		ceiling = DefaultMaxTokens
	}
	limit := (ceiling+1)*bytesPerToken - 1
	sums := make([]int, len(o.Items)+1)
	for i, one := range o.Items {
		sums[i+1] = sums[i] + len(one.render())
	}
	tail := len("\n") + len(evidence(o.Provenance, false))
	fits := func(k int) bool { return len(o.heading(k))+sums[k]+tail <= limit }
	if fits(len(o.Items)) {
		return o
	}
	k := longest(len(o.Items), fits)
	out := o
	out.Items = o.Items[:k]
	out.Provenance.Caveats = append(append([]Caveat(nil), o.Provenance.Caveats...), Caveat{
		Code: string(trust.CaveatTruncated),
		Note: fmt.Sprintf("%d of %d relations returned within the token budget", k, len(o.Items)),
	})
	return out
}

// relationsRefused returns the output of a request that the tool refuses before an engine
// reads it.
func relationsRefused(scope source.Path, in RelationsInput, f *Failure) RelationsOutput {
	return RelationsOutput{
		Scope:      about(scope, in.Language, engine.Answer[sema.Symbol]{}),
		Of:         in.Name,
		Relation:   string(in.Relation),
		Items:      []Connected{},
		Provenance: unserved(),
		Error:      f,
	}
}

const relationsDescription = "PREFER OVER grep for finding what calls, implements or " +
	"references a declaration. It returns each site with its line of code, and states whether " +
	"an empty answer proves that there are none."
