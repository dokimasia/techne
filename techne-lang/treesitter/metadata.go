// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package treesitter

import (
	"bytes"
	"sort"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
)

// modifiers returns the modifier keywords of a declaration, in source order
// and without duplicates. It reads the children of the declaring node and,
// when the grammar wraps the declaration, the children of the wrapper, as
// Python wraps a decorated function in a decorated_definition.
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
				// Java nests annotations with their arguments inside the
				// modifiers node, so the keywords come from its children.
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

// keywords returns the modifier keywords under a node that groups them,
// without the annotations nested in it. A node without keyword children is
// one keyword, as Rust's visibility_modifier writes pub(crate).
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

// annotations returns the annotations of a declaration, in source order.
//
// A grammar attaches an annotation in one of four places: as a child of the
// declaration, as TypeScript does; as a child of a wrapper, as Python's
// decorated_definition does; as a preceding sibling, as Rust's
// attribute_item is; or inside the modifiers node, as Java does. A node that
// groups more than one annotation, such as C#'s attribute_list, yields one
// annotation per member.
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
		if grouped := nested(n); len(grouped) > 1 {
			for _, one := range grouped {
				add(one)
			}
			return
		}
		// A group of one keeps its punctuation, which a tool that
		// reproduces the annotation needs.
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
			if modifierNodes[NodeKind(child.Kind())] {
				fromChildren(child, depth+1)
			}
		}
	}

	fromChildren(node, 0)
	if parent := node.Parent(); parent != nil && wrapperNodes[NodeKind(parent.Kind())] {
		fromChildren(parent, 0)
	}
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

// nested returns the attribute nodes a node groups. It returns nothing for
// a node that is one annotation.
func nested(node *ts.Node) []*ts.Node {
	var out []*ts.Node
	for i := range node.NamedChildCount() {
		if child := node.NamedChild(i); child != nil && NodeKind(child.Kind()) == NodeAttribute {
			out = append(out, child)
		}
	}
	return out
}

// annotationName returns the name of an annotation without its punctuation,
// its arguments and its qualifier: Serializable for [System.Serializable],
// derive for #[derive(Debug)].
func annotationName(text string) string {
	name := strings.TrimSpace(text)
	name = strings.TrimPrefix(name, rustInnerAttributeOpen)
	name = strings.TrimPrefix(name, rustAttributeOpen)
	// C# writes [Foo] and C writes [[foo]].
	name = strings.TrimLeft(name, attributeOpen)
	name = strings.TrimRight(name, attributeClose)
	name = strings.TrimPrefix(name, annotationPrefix)
	name = strings.TrimSpace(name)

	if at := strings.IndexAny(name, annotationBreak); at >= 0 {
		name = name[:at]
	}
	if at := strings.LastIndexAny(name, annotationQualifier); at >= 0 && at+1 < len(name) {
		name = name[at+1:]
	}
	return name
}

// tags returns one annotation per key of the struct tag of a Go field,
// named by the key, so a caller asks for a json tag as it asks for any
// annotation.
func tags(node *ts.Node, content []byte, p source.Path) []sema.Annotation {
	tag := node.ChildByFieldName(string(FieldNameTag))
	if tag == nil {
		return nil
	}
	var out []sema.Annotation
	for _, one := range pairs(unquote(tag.Utf8Text(content))) {
		out = append(out, sema.Annotation{Name: one.key, Text: one.text, Span: spanOf(p, *tag)})
	}
	return out
}

// pair is one key of a struct tag and the key:"value" text it is written
// as.
type pair struct{ key, text string }

// pairs splits a struct tag into its key:"value" pairs by the syntax that
// reflect.StructTag documents: pairs separated by spaces, a key without a
// space, a quote or a colon, and a value that is a quoted Go string. It
// stops at the first pair that breaks the syntax.
func pairs(tag string) []pair {
	var out []pair
	for len(tag) > 0 {
		tag = strings.TrimLeft(tag, " ")
		colon := strings.Index(tag, tagSeparator)
		if colon <= 0 || colon+1 >= len(tag) || tag[colon+1] != tagQuote {
			return out
		}
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

// Parents sets the Parent of each symbol to the ID of the smallest other
// symbol in the same file whose span contains it and is wider, as
// [sema.Containers] defines it. Parents leaves the Parent empty when there
// is no such symbol, and when that symbol has the same ID, which a grammar
// produces for a declaration nested in another of the same name and kind.
// It returns the containers that sema.Containers returns.
//
// Parents runs in O(n log n) time.
func Parents(symbols []sema.Symbol) []int {
	containers := sema.Containers(symbols)
	link(symbols, containers)
	return containers
}

// link sets the Parent of each symbol to the ID of its container, of the
// indexes that [sema.Containers] returns. It leaves the Parent empty for a
// symbol without a container, and for a container with the same ID.
func link(symbols []sema.Symbol, containers []int) {
	for i, container := range containers {
		if container >= 0 && symbols[container].ID != symbols[i].ID {
			symbols[i].Parent = symbols[container].ID
		}
	}
}

// visibility returns the visibility of found[i], whose containers are the
// indexes that Parents returns. A declaration that no code outside its
// scope can name is sema.Unexported: a kind that sema.Kind.Declares
// excludes, and a variable or constant inside a function, a method or a
// constructor. The declaration of the language reads the visibility of
// every other declaration from its name.
func (e *Engine) visibility(found []declaration, containers []int, i int) sema.Visibility {
	kind := found[i].kind
	if !kind.Declares() || (kind == sema.KindVariable || kind == sema.KindConstant) && local(found, containers, i) {
		return sema.Unexported
	}
	return e.declared.Visibility(found[i].name)
}

// local reports whether a function, a method or a constructor contains
// found[i].
func local(found []declaration, containers []int, i int) bool {
	for at := containers[i]; at >= 0; at = containers[at] {
		switch found[at].kind {
		case sema.KindFunction, sema.KindMethod, sema.KindConstructor:
			return true
		}
	}
	return false
}

// documentation returns the documentation of a declaration: the
// documentation comments above it, or the docstring that opens its body in
// a language that documents inside a declaration. It returns an empty
// string for a kind that no language documents, and for a declaration whose
// only comment is not in a documentation form.
func documentation(node *ts.Node, content []byte, style lang.CommentStyle, kind sema.Kind) string {
	if !documents(kind) {
		return ""
	}
	if style.Documents().Inside {
		return inside(node, content, style)
	}
	return above(node, content, style)
}

// signature returns the text of a declaration without its body, on one line
// and cut to [lang.LineLimit] bytes.
//
// The body is the child named body. For an aggregate without one, such as a
// Go struct type, the signature ends at the brace that opens the body. A
// value written over more than one line ends before its assignment. The
// annotations inside the span are dropped, because the symbol lists them.
func signature(node *ts.Node, content []byte, marks []sema.Annotation, kind sema.Kind, style lang.CommentStyle) string {
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
		// A comment between the signature and the body documents what
		// follows it, as Ruby writes one before a method body.
		return trimmed(uncommented(string(content[start:body.StartByte()]), style))
	}
	text := string(content[start:end])
	if aggregate(kind) {
		if at := opens(text); at >= 0 {
			text = text[:at]
		}
	} else if strings.ContainsRune(text, '\n') {
		if at := assigns(text); at >= 0 {
			text = text[:at]
		}
	}
	return trimmed(text)
}

// aggregate reports whether kind is written with a body in braces.
func aggregate(kind sema.Kind) bool {
	switch kind {
	case sema.KindStruct, sema.KindInterface, sema.KindUnion, sema.KindEnum,
		sema.KindAnnotation, sema.KindModule, sema.KindImplementation:
		return true
	default:
		return false
	}
}

// opens returns the offset of the brace that opens the body of the
// declaration in text, or -1. A brace inside parentheses or brackets
// belongs to a value, not to the declaration.
func opens(text string) int {
	depth := 0
	for at := 0; at < len(text); at++ {
		switch text[at] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case bodyOpen:
			if depth <= 0 {
				return at
			}
		}
	}
	return -1
}

// assigns returns the offset of the = that assigns the value of the
// declaration in text, or -1. It skips an = inside brackets and the
// operators ==, =>, !=, <=, >= and the compound assignments.
func assigns(text string) int {
	depth := 0
	for at := 0; at < len(text); at++ {
		switch text[at] {
		case '(', '[', '{', '<':
			depth++
		case ')', ']', '}', '>':
			depth--
		case '=':
			if depth > 0 {
				continue
			}
			if at+1 < len(text) && (text[at+1] == '=' || text[at+1] == '>') {
				at++
				continue
			}
			if at > 0 && strings.ContainsRune("=!<>+-*/%&|^:", rune(text[at-1])) {
				continue
			}
			return at
		}
	}
	return -1
}

// uncommented removes the trailing lines of text that are blank or open a
// comment of the language.
func uncommented(text string, style lang.CommentStyle) string {
	lines := strings.Split(text, "\n")
	for len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last != "" && !comments(last, style) {
			break
		}
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// comments reports whether line opens a comment of the language.
func comments(line string, style lang.CommentStyle) bool {
	if open := strings.TrimSpace(style.Line); open != "" && strings.HasPrefix(line, open) {
		return true
	}
	if style.BlockOpen != "" && strings.HasPrefix(line, style.BlockOpen) {
		return true
	}
	for _, form := range style.Doc {
		if form.Open != "" && strings.HasPrefix(line, form.Open) {
			return true
		}
	}
	return false
}

// trimmed returns a signature on one line, without the punctuation that
// removing the body leaves, cut by [lang.Clipped] to [lang.LineLimit] bytes.
func trimmed(text string) string {
	return lang.Clipped(oneLine(strings.TrimRight(strings.TrimSpace(text), signatureTail)), lang.LineLimit)
}

// oneLine joins a signature written over more than one line into one line,
// with single spaces and no space inside brackets.
func oneLine(text string) string {
	if !strings.ContainsAny(text, "\n\r\t") {
		return text
	}
	out := strings.Join(strings.Fields(text), " ")
	for _, tighten := range [][2]string{
		{"( ", "("},
		{" )", ")"},
		{" ,", ","},
		{",)", ")"},
		{"[ ", "["},
		{" ]", "]"},
		{"< ", "<"},
		{" >", ">"},
	} {
		out = strings.ReplaceAll(out, tighten[0], tighten[1])
	}
	return out
}

// enclosing returns the node the signature of a declaration starts at.
//
// It climbs from the declaring node to each parent whose own text belongs
// to the signature, as Go's type_declaration contributes the keyword type
// to its type_spec. It stops at a parent that contains more than the
// declaration, so a method in an interface does not take the text of the
// interface and a decorated function does not take its decorator.
func enclosing(node *ts.Node, content []byte) *ts.Node {
	out := node
	for {
		parent := out.Parent()
		if parent == nil || !declares(parent, out, content) {
			return out
		}
		out = parent
	}
}

// declares reports whether the text of parent belongs to the signature of
// node: parent writes no bracket before node, and nothing between node and
// the body except node itself. C writes a function as a return type and a
// declarator side by side, so the declarator alone is half the signature.
func declares(parent, node *ts.Node, content []byte) bool {
	if bytes.ContainsAny(content[parent.StartByte():node.StartByte()], blockOpen) {
		return false
	}
	body := parent.ChildByFieldName(string(FieldNameBody))
	if body == nil {
		return parent.NamedChildCount() == 1
	}
	if node.EndByte() > body.StartByte() {
		return false
	}
	for i := range parent.NamedChildCount() {
		child := parent.NamedChild(i)
		if child == nil || child.Equals(*node) || child.Equals(*body) {
			continue
		}
		if child.StartByte() > node.StartByte() && child.EndByte() <= body.StartByte() {
			return false
		}
	}
	return true
}

// bodyOf returns the body field of a declaration, searching at most
// bodyDepth levels below node, as Go writes the fields of a struct under its
// type child.
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

// documents reports whether a documentation tool documents a declaration of
// kind on its own. Parameters, type parameters and labels are documented
// inside the comment of the declaration that contains them.
func documents(kind sema.Kind) bool {
	switch kind {
	case sema.KindParameter, sema.KindTypeParameter, sema.KindLabel:
		return false
	default:
		return true
	}
}

// above returns the documentation comments before a declaration, joined by
// newlines and unwrapped by [lang.CommentStyle.Unwrapped]. It passes over the
// annotations between the comments and the declaration, and stops at a
// blank line and at any sibling that is not a documentation comment of the
// language.
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
		text, isDoc := style.Documentation(sibling.Utf8Text(content))
		if !isDoc {
			break
		}
		lines = append(lines, text)
		next = sibling
	}
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return style.Unwrapped(strings.Join(lines, "\n"))
}

// outermost returns the node that the documentation of a declaration is
// written above.
//
// It climbs to each parent that starts on the line of the node and whose
// first named child is the node, as Go's type_declaration contains a
// type_spec. It always climbs into a wrapper node, such as Python's
// decorated_definition. A comment above `const (` therefore documents the
// group, and a comment above a line inside the group documents that line.
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

// bodied returns the body of a declaration, passing through wrapper nodes
// only. A Python comprehension names its element expression body, and a
// variable bound to one has no body.
func bodied(node *ts.Node) *ts.Node {
	inner := node
	for wrapperNodes[NodeKind(inner.Kind())] {
		count := inner.NamedChildCount()
		if count == 0 {
			break
		}
		last := inner.NamedChild(count - 1)
		if last == nil {
			break
		}
		inner = last
	}
	return inner.ChildByFieldName(string(FieldNameBody))
}

// detached reports whether a blank line separates above from below.
func detached(above, below *ts.Node) bool {
	return below.StartPosition().Row > above.EndPosition().Row+1
}

// inside returns the docstring that opens the body of a declaration, or an
// empty string.
func inside(node *ts.Node, content []byte, style lang.CommentStyle) string {
	body := bodied(node)
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
