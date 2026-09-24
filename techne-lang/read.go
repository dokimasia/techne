// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/techne/core/source"
)

// Largest is the size in bytes above which an engine does not read a file.
// Outlining a minified 1 MiB TypeScript bundle takes 1.3s, and the time
// grows faster than the size: 4.2s at 2 MiB.
const Largest = 1 << 20

// LargeError reports a file larger than [Largest].
type LargeError struct {
	Path source.Path
	Size int64
}

// Error returns the path and the size of the file.
func (e LargeError) Error() string {
	return fmt.Sprintf("lang: %s is %d bytes, larger than the %d bytes an engine reads",
		e.Path, e.Size, Largest)
}

// Readable returns nil if an engine may read the file at p. It returns
// [GeneratedError] when the .gitignore files of the workspace exclude p,
// [LargeError] when the file is larger than [Largest], and an error when p
// does not exist. It reads the .gitignore files above p on every call.
func Readable(fsys fs.FS, p source.Path) error {
	_, info, _, err := located(fsys, p)
	if err != nil || info.IsDir() {
		return err
	}
	return Large(p, info.Size())
}

// Large returns a [LargeError] when size is larger than [Largest], and nil
// otherwise. An engine calls it for a file outside the workspace, such as a
// file of the standard library, which no .gitignore of the workspace covers.
func Large(p source.Path, size int64) error {
	if size > Largest {
		return LargeError{Path: p, Size: size}
	}
	return nil
}

// LineLimit is the length in bytes at which an engine cuts a line of source
// that an answer contains: a signature, the line of a relation site, or the
// line of a finding. A minifier writes a whole program on one line.
const LineLimit = 240

// ellipsis marks each end at which [Excerpt] or [Clipped] cut a text.
const ellipsis = "…"

// LineAt returns the line of content that contains offset, without its line
// terminator and cut by [Excerpt] around offset. It returns an empty string
// for an offset outside content.
func LineAt(content []byte, offset int) string {
	if offset < 0 || offset > len(content) {
		return ""
	}
	start := bytes.LastIndexByte(content[:offset], '\n') + 1
	end := len(content)
	if at := bytes.IndexByte(content[offset:], '\n'); at >= 0 {
		end = offset + at
	}
	return Excerpt(bytes.TrimSuffix(content[start:end], []byte("\r")), offset-start)
}

// Excerpt returns line when it is at most [LineLimit] bytes long. A longer
// line is cut to a window of at most LineLimit bytes around column, a byte
// offset in line. The window starts a quarter of LineLimit before column, but
// no earlier than the start of the line and no later than LineLimit bytes
// before its end. A cut falls on a rune boundary, and the window drops the
// white space at a cut and marks the cut with an ellipsis. Excerpt reads at
// most LineLimit bytes of line.
func Excerpt(line []byte, column int) string {
	if len(line) <= LineLimit {
		return string(line)
	}

	from := min(max(column-LineLimit/4, 0), len(line)-LineLimit)
	to := from + LineLimit
	for from < to && !utf8.RuneStart(line[from]) {
		from++
	}
	for to > from && to < len(line) && !utf8.RuneStart(line[to]) {
		to--
	}
	window := line[from:to]
	var out strings.Builder
	if from > 0 {
		out.WriteString(ellipsis)
		window = bytes.TrimLeftFunc(window, unicode.IsSpace)
	}
	if to < len(line) {
		window = bytes.TrimRightFunc(window, unicode.IsSpace)
	}
	out.Write(window)
	if to < len(line) {
		out.WriteString(ellipsis)
	}
	return out.String()
}

// Clipped returns text when it is at most limit bytes long. A longer text is
// cut at the last rune boundary within limit bytes, without the white space
// before the cut, and ends in an ellipsis.
func Clipped(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := max(limit, 0)
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return strings.TrimRightFunc(text[:cut], unicode.IsSpace) + ellipsis
}
