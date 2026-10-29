// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package version_test

import (
	"runtime/debug"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/internal/version"
)

// The keys of the build settings that the go command records for a build of a git checkout.
const (
	vcsKey      = "vcs"
	revisionKey = "vcs.revision"
	timeKey     = "vcs.time"
	modifiedKey = "vcs.modified"
)

// The build settings of a build of a git checkout.
const (
	revision = "0123456789abcdef0123456789abcdef01234567"
	stamp    = "2026-09-24T00:00:00Z"
)

func TestVersion(t *testing.T) {
	t.Parallel()

	t.Run("Full", func(t *testing.T) {
		t.Parallel()

		t.Run("returns dev for the test binary", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, version.Full(), version.Development, "the version of the test binary")
		})
	})

	t.Run("Of", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			giveVersion  string
			giveSettings []debug.BuildSetting
			want         string
		}{
			{
				name:        "returns the release of a build at a tag with its commit and its time",
				giveVersion: "v1.2.3",
				giveSettings: []debug.BuildSetting{
					{Key: vcsKey, Value: "git"},
					{Key: revisionKey, Value: revision},
					{Key: timeKey, Value: stamp},
					{Key: modifiedKey, Value: "false"},
				},
				want: "1.2.3 (" + revision + ", built " + stamp + ")",
			},
			{
				name:        "returns the release of go install of a version",
				giveVersion: "v1.2.3-rc.1",
				want:        "1.2.3-rc.1",
			},
			{
				name:        "returns dev for a build that records (devel)",
				giveVersion: "(devel)",
				want:        version.Development,
			},
			{
				name:         "returns dev for a build at a commit after the last tag",
				giveVersion:  "v1.2.4-0.20260924000000-0123456789ab",
				giveSettings: []debug.BuildSetting{{Key: revisionKey, Value: revision}},
				want:         version.Development,
			},
			{
				name:         "returns dev for a release version of a working tree with changes",
				giveVersion:  "v1.2.3",
				giveSettings: []debug.BuildSetting{{Key: modifiedKey, Value: "true"}},
				want:         version.Development,
			},
			{
				name:        "returns dev for a version with +dirty",
				giveVersion: "v1.2.3+dirty",
				want:        version.Development,
			},
			{
				name:        "returns dev for a version with .dirty after +incompatible",
				giveVersion: "v2.0.0+incompatible.dirty",
				want:        version.Development,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				info := &debug.BuildInfo{
					Main:     debug.Module{Path: "go.dokimi.dev/techne", Version: tt.giveVersion},
					Settings: tt.giveSettings,
				}
				assert.Equal(t, version.Of(info), tt.want, "the version string")
			})
		}

		t.Run("returns dev for a binary without build information", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, version.Of(nil), version.Development, "the version string")
		})
	})

	t.Run("Format", func(t *testing.T) {
		t.Parallel()

		t.Run("returns dev for an empty version", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, version.Format("", "abc123", "2026-01-01"), version.Development, "the version string")
		})

		t.Run("returns the version for an empty commit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, version.Format("1.2.3", "", "2026-01-01"), "1.2.3", "the version string")
		})

		t.Run("returns the version with the commit for an empty date", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, version.Format("1.2.3", "abc123", ""), "1.2.3 (abc123)", "the version string")
		})

		t.Run("returns the date of the build beside the commit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, version.Format("1.2.3", "abc123", "2026-01-01"), "1.2.3 (abc123, built 2026-01-01)",
				"the version string")
		})
	})
}
