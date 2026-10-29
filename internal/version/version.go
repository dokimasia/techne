// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package version

import (
	"runtime/debug"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// Development is the version string of a build that is not at the tag of a release.
const Development = "dev"

// The keys of the build settings that the go command records for a build of a git checkout.
const (
	revisionKey = "vcs.revision"
	timeKey     = "vcs.time"
	modifiedKey = "vcs.modified"
)

// dirty are the suffixes that the go command appends to the version of a build of a working tree
// with changes: +dirty, and .dirty after the build metadata +incompatible.
var dirty = []string{"+dirty", ".dirty"}

// Full returns the version of the running binary, as [Of] reads its build information.
func Full() string {
	info, _ := debug.ReadBuildInfo()
	return Of(info)
}

// Of returns the version string of a binary with the build information info, as [Format] writes the
// version of the main module without its v, the commit and the time of the commit. Of reads the
// commit from the setting vcs.revision and its time from vcs.time. It returns [Development] for a
// nil info, for a version that is not a semantic version, such as (devel), for a pseudo-version,
// and for a build of a working tree with changes.
func Of(info *debug.BuildInfo) string {
	if info == nil {
		return Development
	}
	var commit, date string
	modified := false
	for _, s := range info.Settings {
		switch s.Key {
		case revisionKey:
			commit = s.Value
		case timeKey:
			date = s.Value
		case modifiedKey:
			modified = s.Value == "true"
		}
	}
	v := info.Main.Version
	for _, suffix := range dirty {
		if strings.HasSuffix(v, suffix) {
			modified = true
		}
	}
	if !semver.IsValid(v) || module.IsPseudoVersion(v) || modified {
		return Development
	}
	return Format(strings.TrimPrefix(v, "v"), commit, date)
}

// Format returns the version string of a build of version, commit and date:
//
//   - [Development] for an empty version
//   - the version for an empty commit
//   - the version with the commit in parentheses for an empty date
//   - the version with the commit and the date in parentheses otherwise
//
// Format("1.2.3", "abc123", "2026-01-01") returns "1.2.3 (abc123, built 2026-01-01)".
func Format(version, commit, date string) string {
	switch {
	case version == "":
		return Development
	case commit == "":
		return version
	case date == "":
		return version + " (" + commit + ")"
	default:
		return version + " (" + commit + ", built " + date + ")"
	}
}
