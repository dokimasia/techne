// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"cmp"
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
)

// Search returns the declarations in a scope that match a query, best match
// first.
//
// A name matches in rank order: the exact text, the text with case folded,
// a prefix, then a substring. A text with a dot matches the qualified name,
// as Instant.Time for the method Time of Instant, and any other text the
// name. Within one rank a shorter name comes first,
// and the ID breaks the remaining ties, so identical requests return
// identical answers. An empty text matches every name. Without q.Private,
// Search leaves out the declarations that Outline reports as
// sema.Unexported. It returns the bindings that q.Include selects, and no
// other binding. Search reads the metadata of matching declarations only.
//
// A text with white space is a description, which no name matches. A
// declaration matches it when its documentation or its name contains every
// word of [described], case folded, and the declarations with the most
// occurrences of the words come first. Search then reads the metadata of
// every declaration that the other filters keep. A description that matches
// nothing and that is written as a declaration, as `func NewMainKubelet` is,
// is searched again as the name that it declares, by the rule of
// [Engine.declaredName].
//
// q.Limit cuts the list after the filters. When it does, a caveat states how
// many of the matches the result contains.
func (e *Engine) Search(ctx context.Context, req engine.Request, q engine.Query) (engine.Result[sema.Symbol], error) {
	files, err := e.walk(req)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}
	words := described(q.Text)
	wanted := func(d named) bool {
		return (q.Kind == sema.KindUnknown || d.kind == q.Kind) &&
			(q.Private || d.visibility != sema.Unexported) &&
			q.Include.Keeps(d.kind, d.local) &&
			(len(words) > 0 || rank(compared(d.name, d.qualified, q.Text), q.Text) != noMatch)
	}
	found, unparsed, err := parse(ctx, e, files.Read, wanted, declaredIn)
	if err != nil {
		return engine.Result[sema.Symbol]{}, err
	}

	type ranked struct {
		symbol sema.Symbol
		rank   int
	}
	var matches []ranked
	for _, symbol := range slices.Concat(found...) {
		score := rank(compared(symbol.Name, symbol.ID.Name(), q.Text), q.Text)
		if len(words) > 0 {
			if score = mentions(symbol, words); score == noMention {
				continue
			}
		}
		matches = append(matches, ranked{symbol: symbol, rank: score})
	}
	if name := e.declaredName(q.Text); len(words) > 0 && len(matches) == 0 && name != "" {
		q.Text = name
		return e.Search(ctx, req, q)
	}
	slices.SortStableFunc(matches, func(a, b ranked) int {
		return cmp.Or(
			cmp.Compare(a.rank, b.rank),
			cmp.Compare(len(a.symbol.Name), len(b.symbol.Name)),
			cmp.Compare(a.symbol.ID, b.symbol.ID),
		)
	})

	total := len(matches)
	if q.Limit > 0 && total > q.Limit {
		matches = matches[:q.Limit]
	}
	items := make([]sema.Symbol, len(matches))
	for i, m := range matches {
		items[i] = m.symbol
	}
	out := result(items, files, unparsed, matchedText)
	if len(items) < total {
		out.Caveats = append(out.Caveats, trust.Caveat{
			Code: trust.CaveatTruncated,
			Note: fmt.Sprintf("%d of %d matches returned", len(items), total),
		})
	}
	return out, nil
}

// The ranks of a name match, best first.
const (
	literal = iota
	folded
	prefix
	substring
	noMatch
)

// shortestWord is the length of the shortest word of a description that
// [described] keeps. Shorter words, such as a, an and to, occur in almost
// every doc comment.
const shortestWord = 3

// described returns the words of text, lower-cased, of at least
// [shortestWord] bytes, when text is a description: it contains white space,
// which no name contains. It returns nil for a name, and for a description
// without a word of that length.
func described(text string) []string {
	if !strings.ContainsFunc(strings.TrimSpace(text), unicode.IsSpace) {
		return nil
	}
	var out []string
	for word := range strings.FieldsSeq(strings.ToLower(text)) {
		if len(word) >= shortestWord && !slices.Contains(out, word) {
			out = append(out, word)
		}
	}
	return out
}

// declaredName returns the name that text declares when text is written as a
// declaration, and the empty string for any other text. text is written as a
// declaration when it contains a keyword of the grammar outside brackets, as
// `func NewMainKubelet` contains func, or an identifier that an opening
// parenthesis follows without white space, as `add(int a, int b)` contains
// add. The name is the first such identifier, or else the first identifier
// that is no keyword, both outside brackets: Get in
// `func (s *store) Get(id string)`, and Store in `type Store struct`. An
// identifier that a parenthesis follows is the name even when the grammar has
// it as a keyword, as TypeScript has the get of `get(): number`.
func (e *Engine) declaredName(text string) string {
	var first, called, previous string
	keyworded, depth := false, 0
	for token, spaced := range tokens(text) {
		switch {
		case token == "(" && depth == 0 && !spaced && called == "":
			called = previous
			depth++
		case strings.Contains(opening, token):
			depth++
		case strings.Contains(closing, token):
			depth = max(depth-1, 0)
		case depth > 0:
		case e.keywords[token]:
			keyworded = true
		case isIdentifier(token) && first == "":
			first = token
		}
		previous = ""
		if depth == 0 && isIdentifier(token) {
			previous = token
		}
	}
	switch {
	case called != "":
		return called
	case keyworded:
		return first
	}
	return ""
}

// opening and closing are the brackets of a parameter list, an index and a
// list of type parameters. [Engine.declaredName] skips what is between them.
const (
	opening = "([<"
	closing = ")]>"
)

// tokens returns the identifiers and the other characters of text, other than
// white space, in order, each with whether white space precedes it.
func tokens(text string) iter.Seq2[string, bool] {
	return func(yield func(string, bool) bool) {
		spaced := false
		for rest := text; rest != ""; {
			r, size := utf8.DecodeRuneInString(rest)
			if unicode.IsSpace(r) {
				spaced, rest = true, rest[size:]
				continue
			}
			if inIdentifier(r) {
				size = len(rest)
				if end := strings.IndexFunc(rest, func(c rune) bool { return !inIdentifier(c) }); end >= 0 {
					size = end
				}
			}
			if !yield(rest[:size], spaced) {
				return
			}
			spaced, rest = false, rest[size:]
		}
	}
}

// noMention is the score of [mentions] for a declaration that does not
// contain every word.
const noMention = 0

// mentions returns the negative count of the occurrences of words in the
// lower-cased documentation and name of s, so more occurrences rank first, and
// [noMention] when a word does not occur.
func mentions(s sema.Symbol, words []string) int {
	text := strings.ToLower(s.Doc + " " + s.Name)
	total := 0
	for _, word := range words {
		n := strings.Count(text, word)
		if n == 0 {
			return noMention
		}
		total += n
	}
	return -total
}

// compared returns the name that wanted is matched against: the qualified
// name, as Instant.Time, when wanted is qualified with a dot, and name
// otherwise.
func compared(name, qualified, wanted string) string {
	if strings.Contains(wanted, ".") {
		return qualified
	}
	return name
}

// rank returns how name matches wanted, and literal for every name when
// wanted is empty.
func rank(name, wanted string) int {
	if wanted == "" {
		return literal
	}
	lowered, lowWanted := strings.ToLower(name), strings.ToLower(wanted)
	switch {
	case name == wanted:
		return literal
	case lowered == lowWanted:
		return folded
	case strings.HasPrefix(lowered, lowWanted):
		return prefix
	case strings.Contains(lowered, lowWanted):
		return substring
	default:
		return noMatch
	}
}
