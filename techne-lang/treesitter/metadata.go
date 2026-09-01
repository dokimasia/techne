// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"sort"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// modifiers reads the keywords qualifying a declaration.
//
// It looks at the declaring node's own children and, where the grammar
// wraps the declaration in something else, at that wrapper's: Python
// puts a decorated function inside a decorated_definition, and the
// keywords sit on either depending on the form.
func modifiers(node *ts.Node, content []byte) []string {
	var out []string
	seen := map[string]bool{}

	add := func(words ...string) {
		for _, word := range words {
			word = strings.TrimSpace(word)
			if word == "" || seen[word] {
				continue
			}
			seen[word] = true
			out = append(out, word)
		}
	}

	scan := func(n *ts.Node) {
		if n == nil {
			return
		}
		for i := range n.ChildCount() {
			child := n.Child(i)
			if child == nil {
				continue
			}
			switch {
			case modifierNodes[NodeKind(child.Kind())]:
				// Java nests annotations inside the modifiers node, and
				// an annotation carries its arguments. Reading the node's
				// text would take `classes = {...})` for a modifier, so
				// the keywords are taken from the children instead.
				add(keywords(child, content)...)
			case !child.IsNamed() && modifierWords[child.Kind()]:
				add(child.Kind())
			}
		}
	}

	scan(node)
	if parent := node.Parent(); parent != nil && wrapperNodes[NodeKind(parent.Kind())] {
		scan(parent)
	}
	return out
}

// keywords reads the modifier words out of a node grouping them,
// passing over any annotation nested inside.
//
// A node with no keyword children is its own keyword: Rust writes a
// visibility_modifier as `pub` or `pub(crate)`, which has no anonymous
// token this would recognise.
func keywords(node *ts.Node, content []byte) []string {
	var out []string
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child == nil || annotationNodes[NodeKind(child.Kind())] {
			continue
		}
		// Scala nests one node per keyword inside its modifiers node.
		if modifierNodes[NodeKind(child.Kind())] {
			out = append(out, keywords(child, content)...)
			continue
		}
		if !child.IsNamed() && modifierWords[child.Kind()] {
			out = append(out, child.Kind())
		}
	}
	if len(out) == 0 {
		text := strings.TrimSpace(node.Utf8Text(content))
		if text != "" && !strings.HasPrefix(text, annotationPrefix) {
			out = append(out, text)
		}
	}
	return out
}

// annotations reads the metadata written onto a declaration.
//
// Three shapes reach it. A grammar may hang the annotation off the
// declaration itself, as TypeScript does; put it in a wrapper node with
// the declaration, as Python's decorated_definition does; or leave it as
// a preceding sibling, as Rust's attribute_item is. Java nests them
// inside the modifiers node.
func annotations(node *ts.Node, content []byte, p source.Path) []sema.Annotation {
	var out []sema.Annotation
	seen := map[uint]bool{}

	add := func(n *ts.Node) {
		text := n.Utf8Text(content)
		out = append(out, sema.Annotation{
			Name: annotationName(text),
			Text: text,
			Span: spanOf(p, *n),
		})
	}

	collect := func(n *ts.Node) {
		if n == nil || seen[n.StartByte()] {
			return
		}
		seen[n.StartByte()] = true
		// A grammar may group several annotations in one node: C# writes
		// [Serializable, Obsolete] as one attribute_list holding two
		// attributes. Each is one annotation, or a caller asking about
		// the second would be told the declaration does not carry it. A
		// node wrapping a single one is kept whole, so the punctuation a
		// tool reproducing it needs stays in the text.
		if grouped := nested(n); len(grouped) > 1 {
			for _, one := range grouped {
				add(one)
			}
			return
		}
		add(n)
	}

	var fromChildren func(n *ts.Node, depth int)
	fromChildren = func(n *ts.Node, depth int) {
		if n == nil || depth > 2 {
			return
		}
		for i := range n.ChildCount() {
			child := n.Child(i)
			if child == nil {
				continue
			}
			if annotationNodes[NodeKind(child.Kind())] {
				collect(child)
				continue
			}
			// Java hides annotations one level down, inside modifiers.
			if modifierNodes[NodeKind(child.Kind())] {
				fromChildren(child, depth+1)
			}
		}
	}

	fromChildren(node, 0)
	if parent := node.Parent(); parent != nil && wrapperNodes[NodeKind(parent.Kind())] {
		fromChildren(parent, 0)
	}

	// Rust writes an attribute as a sibling above the item.
	for sibling := node.PrevNamedSibling(); sibling != nil; sibling = sibling.PrevNamedSibling() {
		if !annotationNodes[NodeKind(sibling.Kind())] {
			break
		}
		collect(sibling)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Span.Start.Offset < out[j].Span.Start.Offset
	})
	return out
}

// nested returns the annotations a node groups, which is empty for a
// node that is one annotation itself.
func nested(node *ts.Node) []*ts.Node {
	var out []*ts.Node
	for i := range node.NamedChildCount() {
		if child := node.NamedChild(i); child != nil && NodeKind(child.Kind()) == NodeAttribute {
			out = append(out, child)
		}
	}
	return out
}

// annotationName strips the punctuation and arguments from an
// annotation, leaving what a caller matches on.
func annotationName(text string) string {
	name := strings.TrimSpace(text)
	name = strings.TrimPrefix(name, rustInnerAttributeOpen)
	name = strings.TrimPrefix(name, rustAttributeOpen)
	// C# writes [Foo] and C writes [[foo]], so the brackets are trimmed
	// however many there are.
	name = strings.TrimLeft(name, attributeOpen)
	name = strings.TrimRight(name, attributeClose)
	name = strings.TrimPrefix(name, annotationPrefix)
	name = strings.TrimSpace(name)

	if at := strings.IndexAny(name, annotationBreak); at >= 0 {
		name = name[:at]
	}
	// A qualified annotation is named by its last segment, which is what
	// it is written and matched as.
	if at := strings.LastIndexAny(name, annotationQualifier); at >= 0 && at+1 < len(name) {
		name = name[at+1:]
	}
	return name
}

// tags reads a Go struct tag, which is metadata written as a string
// literal after the field rather than as a node of its own.
//
// Each key in the tag becomes one annotation, so a caller asks whether a
// field carries a json tag the same way it asks whether a class carries
// an Injectable decorator.
func tags(node *ts.Node, content []byte, p source.Path) []sema.Annotation {
	tag := node.ChildByFieldName(string(FieldNameTag))
	if tag == nil {
		return nil
	}

	var out []sema.Annotation
	for _, one := range pairs(unquote(tag.Utf8Text(content))) {
		out = append(out, sema.Annotation{
			Name: one.key,
			Text: one.text,
			Span: spanOf(p, *tag),
		})
	}
	return out
}

// pair is one key and the whole key:"value" it was written as.
type pair struct{ key, text string }

// pairs splits a struct tag into its key and value pairs.
//
// The grammar is the one reflect.StructTag documents: optionally
// space-separated key:"value" pairs, where a key holds no space, quote
// or colon, and a value is a quoted Go string. Splitting on whitespace
// would cut a value containing a space in half, and trimming quotes off
// both ends would take the last value's closing quote with them.
func pairs(tag string) []pair {
	var out []pair
	for len(tag) > 0 {
		tag = strings.TrimLeft(tag, " ")
		colon := strings.Index(tag, tagSeparator)
		if colon <= 0 || colon+1 >= len(tag) || tag[colon+1] != tagQuote {
			return out
		}

		// The value is a Go string literal, so a quote inside it is
		// escaped and does not end it.
		end := colon + 2
		for end < len(tag) && tag[end] != tagQuote {
			if tag[end] == tagEscape {
				end++
			}
			end++
		}
		if end >= len(tag) {
			return out
		}
		out = append(out, pair{key: tag[:colon], text: tag[:end+1]})
		tag = tag[end+1:]
	}
	return out
}

// Parents links each symbol to the innermost other symbol containing
// it, in place.
//
// Containment is decided by span rather than by a query, so it holds for
// every grammar without a pattern having to say what encloses what, and
// an engine at any tier can use it. The smallest span that strictly
// contains a symbol is its parent.
//
// A symbol whose parent shares its identity is left at the top level.
// That happens where a grammar nests one declaration inside another of
// the same name and kind, and a symbol that is its own parent would make
// a caller building a tree loop.
func Parents(symbols []sema.Symbol) {
	order := make([]int, len(symbols))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return width(symbols[order[a]]) < width(symbols[order[b]])
	})

	for _, i := range order {
		for _, j := range order {
			if i == j || width(symbols[j]) <= width(symbols[i]) {
				continue
			}
			if contains(symbols[j], symbols[i]) {
				if symbols[j].ID != symbols[i].ID {
					symbols[i].Parent = symbols[j].ID
				}
				break
			}
		}
	}
}

func width(s sema.Symbol) int {
	return s.Span.End.Offset - s.Span.Start.Offset
}

func contains(outer, inner sema.Symbol) bool {
	return outer.Span.Start.Offset <= inner.Span.Start.Offset &&
		outer.Span.End.Offset >= inner.Span.End.Offset
}

// documentation reads the documentation attached to a declaration.
//
// Which forms count and where they sit are the language's decision, so
// both come from its [lang.CommentStyle]. A language that documents
// inside the declaration is read from the body; every other language is
// read from what is written above.
//
// A declaration with no documentation yields the empty string. So does
// one whose only comment is an ordinary comment, because a language that
// distinguishes the two means the distinction.
func documentation(node *ts.Node, content []byte, style lang.CommentStyle, kind sema.Kind) string {
	if !documents(kind) {
		return ""
	}
	if style.Documents().Inside {
		return inside(node, content, style)
	}
	return above(node, content, style)
}

// signature returns a declaration without its body.
//
// The body is the part a caller reading an outline did not ask for, and
// every grammar served here names that field body.
//
// Not every declaration uses that field. Go builds a struct from a
// struct_type holding an unnamed field list, so where no body field is
// found the first line is the signature, which is where a language that
// opens a block puts the brace.
//
// The annotations written onto a declaration sit inside its own span and
// are dropped, because they reach a caller as annotations and repeating
// them costs a line each.
//
// A declaration with no body is its own signature, which is right for a
// constant and for an alias.
func signature(node *ts.Node, content []byte, marks []sema.Annotation) string {
	from := enclosing(node, content)
	start, end := int(from.StartByte()), int(from.EndByte())
	if end > len(content) || start >= end {
		return ""
	}

	for _, mark := range marks {
		if mark.Span.Start.Offset >= start && mark.Span.End.Offset <= end && mark.Span.End.Offset > start {
			start = mark.Span.End.Offset
		}
	}

	if body := bodyOf(from, 0); body != nil && int(body.StartByte()) > start {
		// A body can begin after something written above it, as Ruby's
		// does after a comment, so the text is taken back to the line
		// the declaration closes on rather than to the body's start.
		return trimmed(balanced(string(content[start:body.StartByte()])))
	}
	// A declaration whose body the grammar does not name ends where it
	// opens the block, and a declaration with neither ends on its line.
	text := string(content[start:end])
	if at := strings.IndexByte(text, bodyOpen); at >= 0 {
		text = text[:at]
	}
	if at := strings.IndexByte(text, '\n'); at >= 0 {
		text = text[:at]
	}
	return trimmed(text)
}

// balanced returns the leading lines of a signature that close every
// bracket they open, so a signature written across several lines stays
// whole and one followed by anything else does not take it.
func balanced(text string) string {
	depth, taken := 0, 0
	for _, line := range strings.SplitAfter(text, "\n") {
		taken += len(line)
		depth += strings.Count(line, "(") - strings.Count(line, ")")
		depth += strings.Count(line, "[") - strings.Count(line, "]")
		// A blank line closes nothing. Breaking on one would end the
		// signature before it began, where a declaration is written
		// under the annotations that were stripped from it.
		if depth <= 0 && strings.TrimSpace(line) != "" {
			break
		}
	}
	return text[:taken]
}

func trimmed(text string) string {
	return strings.TrimRight(strings.TrimSpace(text), signatureTail)
}

// enclosing returns the node a signature starts at.
//
// A grammar often wraps one declaration in a statement carrying its
// keyword: Go writes `type Store struct{}` as a type_declaration holding
// a type_spec, and a signature without the keyword reads wrong.
//
// The climb stops at a parent holding anything besides this declaration,
// which is what keeps a method inside an interface from taking the
// interface's own text and a decorated function from taking its
// decorator. That is a stricter rule than the one documentation uses,
// because documentation is written above a whole declaration while a
// signature is a part of one.
//
// It never climbs past an opening brace. A parent whose text reaches the
// node through one has opened a body, so the node is a member of it
// rather than the same declaration written wider: a Go interface holding
// one method would otherwise give that method the interface's own text.
func enclosing(node *ts.Node, content []byte) *ts.Node {
	out := node
	for {
		parent := out.Parent()
		if parent == nil || parent.NamedChildCount() != 1 {
			return out
		}
		if parent.StartPosition().Row != out.StartPosition().Row {
			return out
		}
		if first := parent.NamedChild(0); first == nil || !first.Equals(*out) {
			return out
		}
		gap := content[parent.StartByte():out.StartByte()]
		if strings.ContainsAny(string(gap), blockOpen) {
			return out
		}
		out = parent
	}
}

// bodyOf finds the field holding a declaration's body.
//
// It is not always a child of the declaring node: Go writes a struct's
// fields under the type child rather than the spec, so the search
// descends. It stops at bodyDepth, because past that it would find the
// body of something nested inside this declaration rather than this
// declaration's own.
func bodyOf(node *ts.Node, depth int) *ts.Node {
	if depth > bodyDepth {
		return nil
	}
	if body := node.ChildByFieldName(string(FieldNameBody)); body != nil {
		return body
	}
	for i := range node.NamedChildCount() {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if body := bodyOf(child, depth+1); body != nil {
			return body
		}
	}
	return nil
}

// documents reports whether a kind is one a documentation tool attaches
// a comment to.
//
// No language documents a parameter or a label as an entity of its own:
// a parameter is described inside the enclosing declaration's comment,
// which is what @param and its equivalents are for. Without this a
// receiver written at the start of a method's line would take the
// method's own documentation, because the comment does sit above it.
func documents(kind sema.Kind) bool {
	switch kind {
	case sema.KindParameter, sema.KindTypeParameter, sema.KindLabel:
		return false
	default:
		return true
	}
}

// above reads the documentation written before a declaration.
//
// It walks back over the preceding siblings, passing over the
// annotations written between the documentation and the declaration, and
// stops at the first sibling that is neither. A blank line ends the
// comment too: a comment separated from a declaration documents
// something else, which is the rule every one of these languages' own
// documentation tools applies.
func above(node *ts.Node, content []byte, style lang.CommentStyle) string {
	from := outermost(node)

	var lines []string
	next := from
	for sibling := from.PrevNamedSibling(); sibling != nil; sibling = sibling.PrevNamedSibling() {
		if annotationNodes[NodeKind(sibling.Kind())] {
			next = sibling
			continue
		}
		if detached(sibling, next) {
			break
		}
		// Which node kind holds a comment is the grammar's business and
		// they disagree: comment, line_comment, block_comment,
		// doc_comment, html_comment. What counts as documentation is the
		// language's own decision, so the sibling's text is put to it
		// and a sibling that is not documentation ends the walk, whether
		// it is an ordinary comment or the declaration above.
		text, isDoc := style.Documentation(sibling.Utf8Text(content))
		if !isDoc {
			break
		}
		lines = append(lines, text)
		next = sibling
	}

	// The walk ran upwards, so the comment reads bottom to top.
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return strings.Join(lines, "\n")
}

// outermost returns the node a comment documenting this declaration
// would be written above.
//
// A grammar usually wraps a declaration in a statement and a query
// captures the inner node: Go writes `type Store struct{}` as a
// type_declaration holding a type_spec, and the comment is a sibling of
// the declaration rather than of the spec. Climbing to the outermost
// node that starts on the same line and starts with this one lands on
// what the comment is written above, which is the same rule the
// languages' own documentation tools apply: a comment above `const (`
// documents the group, and one above a line inside it documents that
// line.
//
// A wrapper node is climbed whichever line it starts on, because it
// exists to hold the declaration together with what is written before
// it, as Python's decorated_definition does.

func outermost(node *ts.Node) *ts.Node {
	out := node
	for {
		parent := out.Parent()
		if parent == nil {
			return out
		}
		if wrapperNodes[NodeKind(parent.Kind())] {
			out = parent
			continue
		}
		if parent.StartPosition().Row != out.StartPosition().Row {
			return out
		}
		if first := parent.NamedChild(0); first == nil || !first.Equals(*out) {
			return out
		}
		out = parent
	}
}

// detached reports whether a blank line separates two nodes.
func detached(above, below *ts.Node) bool {
	return below.StartPosition().Row > above.EndPosition().Row+1
}

// inside reads the documentation written as the first statement of a
// declaration's body, which is how Python documents.
func inside(node *ts.Node, content []byte, style lang.CommentStyle) string {
	body := node.ChildByFieldName(string(FieldNameBody))
	if body == nil {
		return ""
	}
	first := body.NamedChild(0)
	if first == nil {
		return ""
	}
	if NodeKind(first.Kind()) == NodeExpressionStatement {
		first = first.NamedChild(0)
	}
	if first == nil || NodeKind(first.Kind()) != NodeString {
		return ""
	}
	text, isDoc := style.Documentation(first.Utf8Text(content))
	if !isDoc {
		return ""
	}
	return text
}
