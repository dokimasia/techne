// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import "bytes"

// Indent is how a file indents its lines.
type Indent struct {
	// Spaces reports whether the file indents with spaces rather than tabs.
	Spaces bool
	// Width is the number of spaces of one level of indentation, and the width of a tab in a
	// file that indents with tabs.
	Width int
}

// The bytes that [Indentation] reads: the white space of indentation, the bytes of a line
// ending, and the comma that ends a line whose next line can align under it.
const (
	space   = ' '
	tab     = '\t'
	blanks  = " \t"
	endings = "\r\n"
	comma   = ','
)

const (
	// defaultWidth is the width of a level of indentation in a file that shows none.
	defaultWidth = 4
	// widest is the widest level of spaces that [Indentation] infers.
	widest = 8
	// scanned is the number of lines at the start of a file that [Indentation] reads.
	scanned = 10000
)

// widths are the widths of a level of spaces that [Indentation] infers, in the order in which
// a tie between two of them is decided.
var widths = [...]int{2, 4, 6, 8, 3, 5, 7}

// Indentation returns how content indents, from the first 10,000 lines that contain more than
// white space:
//
//   - Tabs when more of those lines start with a tab than with two or more spaces, and spaces
//     otherwise.
//   - For spaces, the width among 2, 4, 6, 8, 3, 5 and 7 by which the indentation of one line
//     most often differs from the line before, where a tab counts as the spaces that replace
//     it. Two spaces replace four when the width 2 occurs at least two thirds as often. A line
//     whose change in indentation aligns it under a word of a line that ends with a comma is
//     no level, unless its change is 4.
//   - Four spaces for content that shows neither, and tabs of width 4.
func Indentation(content []byte) Indent { return guessed(content, true, defaultWidth) }

// guessed returns the indentation of content, with spaces and width as the indentation of
// content that shows none.
func guessed(content []byte, spaces bool, width int) Indent {
	var (
		tabbed, spaced int
		counts         [widest + 1]int
		previous       []byte
		previousLead   int
		read           int
	)
	for line := range bytes.Lines(content) {
		if read == scanned {
			break
		}
		read++
		line = bytes.TrimRight(line, endings)
		lead := len(line) - len(bytes.TrimLeft(line, blanks))
		if lead == len(line) {
			continue
		}
		switch {
		case bytes.IndexByte(line[:lead], tab) >= 0:
			tabbed++
		case lead > 1:
			spaced++
		}
		change, aligned := level(previous, previousLead, line, lead)
		if aligned && (!spaces || change != width) {
			continue
		}
		if change <= widest {
			counts[change]++
		}
		previous, previousLead = line, lead
	}

	if tabbed != spaced {
		spaces = tabbed < spaced
	}
	if spaces {
		best := 0
		for _, w := range widths {
			if counts[w] > best {
				best, width = counts[w], w
			}
		}
		if width == 4 && counts[4] > 0 && counts[2] > 0 && 3*counts[2] >= 2*counts[4] {
			width = 2
		}
	}
	return Indent{Spaces: spaces, Width: width}
}

// level returns the change in indentation from line a to line b in spaces, where aLead and
// bLead are the lengths of their indentation, and reports whether b looks aligned under a word
// of a rather than indented. It returns 0 when the part of either indentation after their
// common prefix mixes tabs and spaces, and when the change in spaces is no whole number of the
// change in tabs.
func level(a []byte, aLead int, b []byte, bLead int) (int, bool) {
	common := 0
	for common < aLead && common < bLead && a[common] == b[common] {
		common++
	}
	aSpaces, aTabs := counted(a[common:aLead])
	bSpaces, bTabs := counted(b[common:bLead])
	if aSpaces > 0 && aTabs > 0 || bSpaces > 0 && bTabs > 0 {
		return 0, false
	}
	tabs, spaces := distance(aTabs, bTabs), distance(aSpaces, bSpaces)
	if tabs == 0 {
		aligned := spaces > 0 && bSpaces > 0 && bSpaces-1 < len(a) && bSpaces < len(b) &&
			b[bSpaces] != space && a[bSpaces-1] == space && a[len(a)-1] == comma
		return spaces, aligned
	}
	if spaces%tabs == 0 {
		return spaces / tabs, false
	}
	return 0, false
}

// counted returns the number of spaces in indent and the number of its other bytes, which are
// tabs.
func counted(indent []byte) (spaces, tabs int) {
	spaces = bytes.Count(indent, []byte{space})
	return spaces, len(indent) - spaces
}

// distance returns the absolute difference of a and b.
func distance(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
