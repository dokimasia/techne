// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package mock

import (
	"maps"
	"slices"
	"strings"
	"unicode"

	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
)

// Line is one declaration or one use of a mock file, as [Parse] reads it.
type Line struct {
	// Kind is the kind of a declaration, and [sema.KindUnknown] for a use.
	Kind sema.Kind
	// Name is the name that a declaration declares, and empty for a use.
	Name string
	// Uses is the name that a use names, and empty for a declaration.
	Uses string
	// Doc is the documentation of a declaration: the lines above it that start with ;;, without
	// the marker.
	Doc string
	// Depth is the indentation of the line, in levels of two spaces.
	Depth int
	// Span covers the line from its first character after the indentation to its end, without
	// the line terminator.
	Span source.Span
	// At covers the name alone, which a rename rewrites.
	At source.Span
}

// declares maps each word that opens a declaration to the kind of the declaration.
var declares = map[string]sema.Kind{
	"type":   sema.KindType,
	"func":   sema.KindFunction,
	"method": sema.KindMethod,
	"field":  sema.KindField,
	"const":  sema.KindConstant,
	"var":    sema.KindVariable,
}

// Kinds returns the words that open a declaration, sorted.
func Kinds() []string { return slices.Sorted(maps.Keys(declares)) }

const (
	// documents opens a line of documentation, which belongs to the declaration below it.
	documents = ";;"
	// refers opens a use.
	refers = "use"
	// nests is the number of spaces of one level of indentation.
	nests = 2
)

// Parse returns the declarations and the uses of the file at p, in the order of its lines, and
// the span of each line that the language does not have.
//
// A line is blank, documentation that starts with ;;, a declaration, or a use. A declaration
// is a word of [Kinds], one space and a name. A use is the word use, one space and a name. The
// documentation of a declaration is the run of documentation lines right above it. A carriage
// return before a line feed ends the line with it. The span of a line starts after its
// indentation.
func Parse(p source.Path, content []byte) ([]Line, []source.Span) {
	var (
		out    []Line
		broken []source.Span
		doc    []string
		at     int
	)
	for number, raw := range strings.Split(string(content), "\n") {
		text := strings.TrimSuffix(raw, "\r")
		body := strings.TrimLeft(text, " ")
		indent := len(text) - len(body)
		line := source.Span{
			Path:  p,
			Start: source.Position{Offset: at + indent, Line: number, Column: indent},
			End:   source.Position{Offset: at + len(text), Line: number, Column: len(text)},
		}
		at += len(raw) + 1

		switch {
		case body == "":
			doc = nil
		case strings.HasPrefix(body, documents):
			doc = append(doc, strings.TrimSpace(strings.TrimPrefix(body, documents)))
		default:
			one, ok := read(body, line)
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

// read returns the declaration or the use that body writes on line, whose span starts at
// body, and reports whether body is one. A single space separates the word and the name, so
// the span of the name follows from the length of the word.
func read(body string, line source.Span) (Line, bool) {
	word, name, split := strings.Cut(body, " ")
	if !split || name == "" || strings.ContainsAny(name, " \t") {
		return Line{}, false
	}

	from := len(word) + 1
	out := Line{
		Depth: line.Start.Column / nests,
		Span:  line,
		Name:  name,
		At: source.Span{
			Path: line.Path,
			Start: source.Position{
				Offset: line.Start.Offset + from,
				Line:   line.Start.Line,
				Column: line.Start.Column + from,
			},
			End: source.Position{
				Offset: line.Start.Offset + from + len(name),
				Line:   line.Start.Line,
				Column: line.Start.Column + from + len(name),
			},
		},
	}

	if word == refers {
		out.Uses, out.Name = name, ""
		return out, true
	}
	kind, known := declares[word]
	if !known {
		return Line{}, false
	}
	out.Kind = kind
	return out, true
}

// Visibility returns [sema.Exported] for a name that starts with an upper-case letter,
// [sema.Unexported] for another name, and [sema.VisibilityUnknown] for the empty name.
func Visibility(name string) sema.Visibility {
	if name == "" {
		return sema.VisibilityUnknown
	}
	if unicode.IsUpper([]rune(name)[0]) {
		return sema.Exported
	}
	return sema.Unexported
}
