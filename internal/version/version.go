// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package version

// Stamped at link time with -X. Unexported so nothing can set them at
// run time and claim a build this is not.
var (
	tag    = ""
	commit = ""
)

// Dev is what an unstamped build reports.
const Dev = "dev"

// String reports the version this binary was built from.
//
// An unstamped build reports [Dev] rather than an empty string, so a
// client logging the version records something a reader can act on.
func String() string {
	switch {
	case tag == "" && commit == "":
		return Dev
	case tag == "":
		return Dev + "+" + commit
	case commit == "":
		return tag
	default:
		return tag + "+" + commit
	}
}
