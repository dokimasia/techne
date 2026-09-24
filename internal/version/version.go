// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package version

// The variables of the build, which the release build sets with -X flags of the linker. Each is
// empty in a build without the flags.
var (
	// buildVersion is the version of the release, such as 1.2.3.
	buildVersion = ""

	// buildCommit is the commit of the release.
	buildCommit = ""

	// buildDate is the date of the commit.
	buildDate = ""
)

// Full returns the version of this build, as [Format] writes the variables that the release
// build sets.
func Full() string {
	return Format(buildVersion, buildCommit, buildDate)
}

// Format returns the version string of a build of version, commit and date:
//
//   - dev for an empty version
//   - the version for an empty commit
//   - the version with the commit in parentheses for an empty date
//   - the version with the commit and the date in parentheses otherwise
//
// Format("1.2.3", "abc123", "2026-01-01") returns "1.2.3 (abc123, built 2026-01-01)".
func Format(version, commit, date string) string {
	switch {
	case version == "":
		return "dev"
	case commit == "":
		return version
	case date == "":
		return version + " (" + commit + ")"
	default:
		return version + " (" + commit + ", built " + date + ")"
	}
}
