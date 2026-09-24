// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"bufio"
	"io/fs"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/techne/core/source"
)

// ignores contains the patterns of the .gitignore files a walk has read,
// keyed by the directory of each file.
//
// It applies the rules of gitignore(5) to the .gitignore file of each
// directory:
//
//   - The last matching pattern decides, and a deeper file takes precedence
//     over the files above it.
//   - A path under an excluded directory is excluded, whatever a later
//     pattern says about the path.
//   - A pattern without a slash matches the base name at any depth. A
//     pattern with a slash matches from the directory of its file.
//   - A trailing slash matches directories only. A trailing /** matches
//     everything inside a directory, but not the directory.
//
// It does not read .git/info/exclude, core.excludesFile or the index, so a
// walk returns the same files on every machine, and a tracked file under an
// excluded path counts as excluded.
type ignores struct {
	rules map[string][]pattern
}

// pattern is one line of a .gitignore file.
type pattern struct {
	// negate reports a leading "!", which re-includes what an earlier
	// pattern excluded.
	negate bool
	// dirOnly reports a trailing slash, which limits the pattern to
	// directories.
	dirOnly bool
	// anchored reports a slash before the last character, which makes the
	// pattern match from the directory of its file.
	anchored bool
	// parts are the segments of the pattern, in path.Match syntax.
	parts []string
}

// read adds the patterns of the .gitignore file in dir, if there is one.
func (i *ignores) read(fsys fs.FS, dir string) {
	content, err := fs.ReadFile(fsys, path.Join(dir, ".gitignore"))
	if err != nil {
		return
	}
	var patterns []pattern
	lines := bufio.NewScanner(strings.NewReader(string(content)))
	for lines.Scan() {
		if one, ok := parse(lines.Text()); ok {
			patterns = append(patterns, one)
		}
	}
	if len(patterns) == 0 {
		return
	}
	if i.rules == nil {
		i.rules = map[string][]pattern{}
	}
	i.rules[dir] = patterns
}

// skips reports whether the patterns exclude p itself. It does not check
// the directories above p. A walk prunes an excluded directory before it
// reaches the paths inside it.
func (i *ignores) skips(p string, isDir bool) bool {
	if len(i.rules) == 0 {
		return false
	}
	excluded := false
	for _, dir := range ancestors(p) {
		rest := p
		if dir != "." {
			rest = strings.TrimPrefix(p, dir+"/")
		}
		for _, one := range i.rules[dir] {
			if one.matches(rest, isDir) {
				excluded = !one.negate
			}
		}
	}
	return excluded
}

// excludes reports whether the patterns exclude p or a directory above it.
// No pattern excludes the workspace root.
func (i *ignores) excludes(p string, isDir bool) bool {
	for _, dir := range ancestors(p) {
		if dir != "." && i.skips(dir, true) {
			return true
		}
	}
	return i.skips(p, isDir)
}

// ancestors returns the directories above p, from the workspace root down.
// It returns nil for the workspace root, and a list that starts with "."
// for every other path.
func ancestors(p string) []string {
	if p == "." {
		return nil
	}
	var above []string
	for dir := path.Dir(p); dir != "." && dir != "/"; dir = path.Dir(dir) {
		above = append(above, dir)
	}
	slices.Reverse(above)
	return append([]string{"."}, above...)
}

// parse reads one line of a .gitignore file, and reports false for a blank
// line or a comment.
func parse(line string) (pattern, bool) {
	text := trimmed(line)
	if text == "" || strings.HasPrefix(text, "#") {
		return pattern{}, false
	}

	var out pattern
	if rest, ok := strings.CutPrefix(text, "!"); ok {
		out.negate, text = true, rest
	}
	if rest, ok := strings.CutSuffix(text, "/"); ok {
		out.dirOnly, text = true, rest
	}
	if rest, ok := strings.CutPrefix(text, "/"); ok {
		out.anchored, text = true, rest
	}
	if text == "" {
		return pattern{}, false
	}
	out.anchored = out.anchored || strings.Contains(text, "/")
	out.parts = strings.Split(classes(text), "/")
	return out, true
}

// trimmed removes the trailing spaces of line that a backslash does not
// escape.
func trimmed(line string) string {
	trailing := -1
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case ' ':
			if trailing < 0 {
				trailing = i
			}
		case '\\':
			i++
			if i == len(line) {
				return line
			}
			trailing = -1
		default:
			trailing = -1
		}
	}
	if trailing >= 0 {
		return line[:trailing]
	}
	return line
}

// classes rewrites each negated class of a gitignore pattern, which opens
// with "[!", to the form that opens with "[^", which path.Match reads.
func classes(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		switch {
		case text[i] == '\\' && i+1 < len(text):
			out.WriteString(text[i : i+2])
			i++
		case text[i] == '[' && i+1 < len(text) && text[i+1] == '!':
			out.WriteString("[^")
			i++
		default:
			out.WriteByte(text[i])
		}
	}
	return out.String()
}

// matches reports whether the pattern matches rest, a path relative to the
// directory of the pattern's file.
func (p pattern) matches(rest string, isDir bool) bool {
	if p.dirOnly && !isDir {
		return false
	}
	if !p.anchored {
		ok, err := path.Match(p.parts[0], path.Base(rest))
		return err == nil && ok
	}
	return covers(p.parts, strings.Split(rest, "/"))
}

// covers reports whether parts match every segment of segments. A "**" part
// matches any number of segments. A trailing "**" matches one segment or
// more.
func covers(parts, segments []string) bool {
	for len(parts) > 0 {
		if parts[0] == "**" {
			parts = parts[1:]
			if len(parts) == 0 {
				return len(segments) > 0
			}
			for skip := range len(segments) + 1 {
				if covers(parts, segments[skip:]) {
					return true
				}
			}
			return false
		}
		if len(segments) == 0 {
			return false
		}
		if ok, err := path.Match(parts[0], segments[0]); err != nil || !ok {
			return false
		}
		parts, segments = parts[1:], segments[1:]
	}
	return len(segments) == 0
}

// GeneratedError reports a path that the .gitignore files of the workspace
// exclude.
type GeneratedError struct{ Scope source.Path }

// Error returns the path.
func (g GeneratedError) Error() string {
	return "lang: the .gitignore files of the workspace exclude " + string(g.Scope)
}
