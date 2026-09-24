// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// Parser returns the outline engine of [Language] over the workspace at root. It reads a file
// as a parser of the language reads it, and reports these declarations:
//
//   - a type for each line that starts with type, which is a struct when the line contains
//     struct
//   - a function for each func at the start of a line or after "; ", which is a method
//     qualified by the type of its receiver when it has one
//   - a variable for each line whose text starts with var
//
// A declaration whose line ends with an opening brace spans the lines through the next line
// that is a closing brace. Any other declaration spans its text on its line. The parent of a
// declaration is the smallest declaration whose span contains it.
func Parser(root string) engine.Outliner { return parser{root: root} }

// parser is the outline engine that [Parser] returns.
type parser struct{ root string }

// Outline returns the declarations of the file that the scope of req names. A scope that names
// no file with the [Extension] suffix returns a skipped result.
func (p parser) Outline(_ context.Context, req engine.Request) (engine.Result[sema.Symbol], error) {
	if path.Ext(string(req.Scope)) != Extension {
		return engine.Result[sema.Symbol]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}
	content, err := os.ReadFile(filepath.Join(p.root, filepath.FromSlash(string(req.Scope))))
	if err != nil {
		return engine.Result[sema.Symbol]{}, fmt.Errorf("lsptest: read %s: %w", req.Scope, err)
	}
	return engine.Result[sema.Symbol]{
		Items:        declarations(req.Scope, string(content)),
		Completeness: trust.ScopeTotal,
	}, nil
}

// found is one declaration that [declarations] reads, before its identity is known.
type found struct {
	kind     sema.Kind
	name     string
	receiver string
	span     source.Span
}

// declarations returns the declarations of text, the content of the file at p, in the order of
// the text.
func declarations(p source.Path, text string) []sema.Symbol {
	lines := strings.Split(text, "\n")
	starts := make([]int, len(lines))
	for n := 1; n < len(lines); n++ {
		starts[n] = starts[n-1] + len(lines[n-1]) + 1
	}
	at := func(n, column int) source.Position {
		return source.Position{Offset: starts[n] + column, Line: n, Column: column}
	}
	// spanned returns the span from column from to column to of line n, through the next line
	// that is a closing brace when the text ends with an opening brace.
	spanned := func(n, from, to int) source.Span {
		end := at(n, to)
		if strings.HasSuffix(strings.TrimSpace(lines[n][from:to]), "{") {
			for close := n + 1; close < len(lines); close++ {
				if lines[close] == "}" {
					end = at(close, 1)
					break
				}
			}
		}
		return source.Span{Path: p, Start: at(n, from), End: end}
	}

	var read []found
	for n, line := range lines {
		if rest, typed := strings.CutPrefix(line, "type "); typed {
			kind := sema.KindType
			if strings.Contains(line, " struct") {
				kind = sema.KindStruct
			}
			read = append(read, found{kind: kind, name: word(rest), span: spanned(n, 0, len(line))})
		}
		for from := 0; from < len(line); {
			start := strings.Index(line[from:], "func ")
			if start < 0 {
				break
			}
			start += from
			end := strings.Index(line[start:], "; func ")
			if end < 0 {
				end = len(line)
			} else {
				end += start
			}
			if start == 0 || strings.HasSuffix(line[:start], "; ") {
				read = append(read, function(line[start:end], spanned(n, start, end)))
			}
			from = end + 1
		}
		if text := strings.TrimLeft(line, "\t "); strings.HasPrefix(text, "var ") {
			from := len(line) - len(text)
			read = append(read, found{
				kind: sema.KindVariable, name: word(text[len("var "):]), span: spanned(n, from, len(line)),
			})
		}
	}
	return identified(p, read)
}

// function returns the function or the method that text declares, from func to the end of the
// declaration.
func function(text string, span source.Span) found {
	rest := strings.TrimPrefix(text, "func ")
	out := found{kind: sema.KindFunction, span: span}
	if strings.HasPrefix(rest, "(") {
		closing := strings.Index(rest, ")")
		fields := strings.Fields(rest[1:closing])
		out.kind, out.receiver = sema.KindMethod, strings.TrimLeft(fields[len(fields)-1], "*")
		rest = strings.TrimSpace(rest[closing+1:])
	}
	out.name = word(rest)
	return out
}

// identified returns the declarations of read with their identities and their parents. A
// method is qualified by its receiver, and any other declaration by its container.
func identified(p source.Path, read []found) []sema.Symbol {
	out := make([]sema.Symbol, len(read))
	for i, one := range read {
		out[i] = sema.Symbol{Name: one.name, Kind: one.kind, Language: Language, Span: one.span}
	}
	containers := sema.Containers(out)
	unit := source.Path(path.Dir(string(p)))
	qualified := make([]string, len(read))
	for i := range read {
		// A container precedes what it contains in the text, so its qualified name is known.
		container := read[i].receiver
		if up := containers[i]; up >= 0 && container == "" {
			container = qualified[up]
			out[i].Parent = out[up].ID
		}
		qualified[i] = sema.Qualify(container, read[i].name)
		out[i].ID = sema.NewID(Language, unit, qualified[i], read[i].kind)
		out[i].Visibility = Declaration().Visibility(read[i].name)
	}
	return out
}

// word returns the identifier at the start of text: the ASCII letters, digits and underscores
// before the first other character.
func word(text string) string {
	end := strings.IndexFunc(text, func(r rune) bool { return !identifying(r) })
	if end < 0 {
		return text
	}
	return text[:end]
}

// identifying reports whether r is an ASCII letter, a digit or an underscore.
func identifying(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}
