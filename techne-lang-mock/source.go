// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"strings"
	"unicode"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Line is one line of the language, read.
//
// A file is its lines and nothing else: there is no expression, no type
// and no block. What is here is what the ports need to be exercised —
// something to name, something to nest it in, something to refer to it,
// and somewhere to write documentation.
type Line struct {
	// Kind is what the line declares, or [sema.KindUnknown] for a line
	// that refers rather than declares.
	Kind sema.Kind
	Name string
	// Uses is the declaration a use line names, empty on a declaration.
	Uses string
	// Doc is the documentation written above, already stripped.
	Doc string
	// Depth is how far the line is indented, in levels of two spaces.
	Depth int
	// Span covers the line, without its terminator.
	Span source.Span
	// At covers the name alone. Rewriting the line would rewrite what
	// is around it as well, which is the difference between a rename and
	// a substitution.
	At source.Span
}

// declares reports whether a word opens a declaration, and what of.
//
// The set is small on purpose. A language with every kind would be a
// language to maintain rather than one to drive the tools with, and the
// kinds here are the ones that nest, that are referred to, and that a
// caller filters on.
var declares = map[string]sema.Kind{
	"type":   sema.KindType,
	"func":   sema.KindFunction,
	"method": sema.KindMethod,
	"field":  sema.KindField,
	"const":  sema.KindConstant,
	"var":    sema.KindVariable,
}

// Kinds returns the words this language declares with, so a caller can
// write a file without reading the parser.
func Kinds() []string {
	return []string{"const", "field", "func", "method", "type", "var"}
}

const (
	// documents opens a documentation line, which attaches to the
	// declaration below it.
	documents = ";;"
	// refers opens a line naming a declaration rather than making one.
	refers = "use"
	// nests is how much indentation one level is.
	nests = 2
)

// Parse reads a file.
//
// Every line is one of four things: blank, documentation, a declaration
// or a use. Anything else is a fault, which is what makes a gate over
// this language mean something.
func Parse(p source.Path, content []byte) ([]Line, []source.Span) {
	var (
		out    []Line
		broken []source.Span
		doc    []string
		at     int
	)

	for number, text := range strings.Split(string(content), "\n") {
		line := source.Span{
			Path:  p,
			Start: source.Position{Offset: at, Line: number},
			End:   source.Position{Offset: at + len(text), Line: number, Column: len(text)},
		}
		at += len(text) + 1

		body := strings.TrimLeft(text, " ")

		switch {
		case body == "":
			doc = nil
		case strings.HasPrefix(body, documents):
			doc = append(doc, strings.TrimSpace(strings.TrimPrefix(body, documents)))
		default:
			one, ok := read(body, len(text)-len(body), line)
			if !ok {
				broken = append(broken, line)
				doc = nil
				continue
			}
			one.Doc = strings.Join(doc, "\n")
			doc = nil
			out = append(out, one)
		}
	}
	return out, broken
}

// read turns one non-blank, non-documentation line into what it says.
//
// One word, one space, one name. Anything else is a fault rather than
// something to be lenient about: the name's position is worked out from
// the two lengths, and a line that is lenient about spacing is one where
// a rename writes over the wrong bytes.
func read(body string, indent int, line source.Span) (Line, bool) {
	word, name, split := strings.Cut(body, " ")
	if !split || name == "" || strings.ContainsAny(name, " \t") {
		return Line{}, false
	}

	from := indent + len(word) + 1
	held := Line{
		Depth: indent / nests,
		Span:  line,
		Name:  name,
		At: source.Span{
			Path: line.Path,
			Start: source.Position{
				Offset: line.Start.Offset + from,
				Line:   line.Start.Line,
				Column: from,
			},
			End: source.Position{
				Offset: line.Start.Offset + from + len(name),
				Line:   line.Start.Line,
				Column: from + len(name),
			},
		},
	}

	if word == refers {
		held.Uses, held.Name = name, ""
		return held, true
	}
	kind, known := declares[word]
	if !known {
		return Line{}, false
	}
	held.Kind = kind
	return held, true
}

// Visibility reads a name the way Go does, because a language needs some
// rule and this one is the shortest to state.
func Visibility(name string) sema.Visibility {
	if name == "" {
		return sema.VisibilityUnknown
	}
	if unicode.IsUpper([]rune(name)[0]) {
		return sema.Exported
	}
	return sema.Unexported
}
