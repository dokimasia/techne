// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lang

import (
	"encoding/json"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

// NodeMajor returns the major version of the npm package name in the workspace of fsys, and
// reports whether the workspace names the package. The version comes from the first of these:
//
//   - the version of node_modules/<name>/package.json, the package that npm installed
//   - the range that the root package.json names for the package in its dependencies, its
//     development dependencies, its catalog, or the catalog of its workspaces, in that order.
//     bun reads a catalog for a dependency whose range is catalog:, which names no version
//     itself.
//
// A range contributes the first number it writes, so ^7.0.2, ~7.1 and >=7 name 7. A range
// without a number, such as latest or workspace:*, names no version, and so does a nil fsys.
func NodeMajor(fsys fs.FS, name string) (int, bool) {
	if fsys == nil {
		return 0, false
	}
	var installed struct {
		Version string `json:"version"`
	}
	if read(fsys, path.Join("node_modules", name, "package.json"), &installed) {
		if major, found := leading(installed.Version); found {
			return major, true
		}
	}
	var root struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Catalog         map[string]string `json:"catalog"`
		Workspaces      json.RawMessage   `json:"workspaces"`
	}
	if !read(fsys, "package.json", &root) {
		return 0, false
	}
	// The workspaces of a package are a list of patterns or an object with a catalog.
	var workspaces struct {
		Catalog map[string]string `json:"catalog"`
	}
	_ = json.Unmarshal(root.Workspaces, &workspaces)
	for _, ranges := range []map[string]string{
		root.Dependencies, root.DevDependencies, root.Catalog, workspaces.Catalog,
	} {
		if spec, named := ranges[name]; named && !strings.HasPrefix(spec, "catalog:") {
			return leading(strings.TrimPrefix(spec, "npm:"+name+"@"))
		}
	}
	return 0, false
}

// read decodes the JSON file at p of fsys into into, and reports whether it could.
func read(fsys fs.FS, p string, into any) bool {
	content, err := fs.ReadFile(fsys, p)
	return err == nil && json.Unmarshal(content, into) == nil
}

// leading returns the first number that spec writes, and reports whether it writes one.
func leading(spec string) (int, bool) {
	start := strings.IndexFunc(spec, func(r rune) bool { return r >= '0' && r <= '9' })
	if start < 0 {
		return 0, false
	}
	end := start
	for end < len(spec) && spec[end] >= '0' && spec[end] <= '9' {
		end++
	}
	major, err := strconv.Atoi(spec[start:end])
	return major, err == nil
}
