// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package scala_test

import (
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/lsp"
	"go.dokimi.dev/techne/lang/scala"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the language scala", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, scala.Declaration().Language, scala.Language, "the language of the declaration")
			assert.Equal(t, string(scala.Language), "scala", "the value of Language")
		})

		t.Run("claims the extensions of Scala", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, scala.Declaration().Extensions, []string{".scala", ".sc"}, "the extensions of Scala")
		})

		t.Run("lists the manifests of a Scala project", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, scala.Declaration().Manifests, []string{
				"build.sbt", "build.mill", "build.mill.yaml", "build.sc", "project.scala",
				"pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts",
			}, "the manifests of Scala")
		})

		t.Run("returns the path without its extension as the unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, scala.Declaration().Namespace("src/main/scala/Store.scala"), "src/main/scala/Store",
				"the unit of src/main/scala/Store.scala")
		})

		t.Run("reports VisibilityUnknown for every name", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"Store", "store"} {
				assert.Equal(t, scala.Declaration().Visibility(name), sema.VisibilityUnknown, "the visibility of "+name)
			}
		})

		t.Run("ignores the placeholder", func(t *testing.T) {
			t.Parallel()
			assert.True(t, scala.Declaration().Blank["_"], "the blank identifiers of Scala")
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("runs metals without arguments", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, scala.Server().Command, []string{"metals"}, "the command of Metals")
		})

		t.Run("opens a file as scala", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, scala.Server().LanguageID, lsp.IdentityScala, "the language identifier of Metals")
		})

		t.Run("imports every build without a prompt", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, scala.Server().Settings,
				map[string]any{"metals": map[string]any{"autoImportBuilds": "all"}}, "the settings of Metals")
		})

		t.Run("waits two minutes for the build import", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, scala.Server().Loading, 2*time.Minute, "the loading time of Metals")
		})
	})
}
