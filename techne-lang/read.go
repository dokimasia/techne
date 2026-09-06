// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"fmt"
	"io/fs"
	"path"
	"strconv"

	"go.dokimi.dev/techne/core/source"
)

// Largest is the biggest file an engine reads.
//
// # Why there is a bound at all
//
// Parsing is linear and what follows it is not. Outlining one minified
// bundle with the TypeScript grammar, measured: 128ms at 256 KiB, 335ms
// at 512 KiB, 1.3s at 1 MiB, 4.2s at 2 MiB and 11.8s at 3.3 MiB. The
// parse itself is 520ms of that last figure; the rest is building the
// 52,976 declarations the file holds. A call that has two seconds cannot
// spend eleven of them on one file, and nothing about the answer is
// worth it — the declarations are a bundler's, not anyone's.
//
// # Why a megabyte
//
// It is the last point on that curve inside the bar, and it is far above
// anything written by hand. The largest hand-written file in the two
// repositories this was measured over is 80 KB, and the largest in
// techne itself is 48 KB. A file over a megabyte is generated, vendored
// or concatenated, and [ignores] catches it already wherever the project
// says so; this is the backstop for the projects that do not.
const Largest = 1 << 20

// LargeError is why a file was not read: it is past [Largest].
//
// Named rather than counted, so a caller told a file was left out can
// decide it wanted that one anyway and read it itself.
type LargeError struct {
	Path source.Path
	Size int64
}

func (l LargeError) Error() string {
	return "lang: " + string(l.Path) + " is " + strconv.FormatInt(l.Size, 10) +
		" bytes, past the " + strconv.Itoa(Largest) +
		" an engine reads: parsing it costs more than an answer is worth"
}

// Readable reports why a file should not be read, or nil.
//
// Two rules hold wherever an engine opens a file: what the workspace
// itself calls generated is not read, and neither is a file too big to
// parse inside the time a call has. [FilesIn] applies both to a walk.
// This is the same question asked about one path, and it is what the
// write path asks — a caller naming a file reaches an engine without
// passing a walk, and a change planned over a three-megabyte bundle
// costs the same whether a walk or a caller chose it.
//
// One gate rather than one per rule, because a read site that remembered
// to ask about the size and forgot to ask about the workspace is a read
// site that parses the build output.
//
// The .gitignore files above the path are read on every call. That is a
// handful of opens that mostly find nothing, and it is what makes the
// answer right when a file appears, moves or is newly ignored between
// two calls.
func Readable(fsys fs.FS, p source.Path) error {
	name := path.Clean(string(p))
	if name == "" {
		name = "."
	}

	info, err := fs.Stat(fsys, name)
	if err != nil {
		return fmt.Errorf("lang: %q: %w", p, err)
	}

	var skipping ignores
	for _, above := range ancestors(name) {
		skipping.reading(fsys, above)
	}
	if skipping.skips(name, info.IsDir()) {
		return GeneratedError{Scope: p}
	}

	if info.IsDir() {
		return nil
	}
	return Large(p, info.Size())
}

// Large reports whether a file is past [Largest], for a caller holding
// the file's size rather than a workspace to look it up in.
//
// Split out because half the rule holds where the other half has nothing
// to say. A server answers about a standard library and a module cache
// as well as about the workspace, and no .gitignore in the workspace
// speaks for a file outside it — but a generated file in a module cache
// costs the same to parse as one in the project.
func Large(p source.Path, size int64) error {
	if size > Largest {
		return LargeError{Path: p, Size: size}
	}
	return nil
}
