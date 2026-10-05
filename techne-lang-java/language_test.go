// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java_test

import (
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/lang/java"
	"go.dokimi.dev/techne/lang/lsp"
)

// outside is the argument of the launcher of jdtls that the test pins: the JVM system property
// that keeps the Eclipse metadata of a project in the data directory of jdtls.
const outside = "--jvm-arg=-Djava.import.generatesMetadataFilesAtProjectRoot=false"

// The sections that the test pins: jdtls requests the width of a level of indentation of a file
// under tabSize, and whether the file indents with spaces under insertSpaces.
const (
	tabSize      = "java.format.tabSize"
	insertSpaces = "java.format.insertSpaces"
)

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("Declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the language java", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, java.Declaration().Language, java.Language, "the language of the declaration")
			assert.Equal(t, string(java.Language), "java", "the value of Language")
		})

		t.Run("claims the extension of Java", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, java.Declaration().Extensions, []string{".java"}, "the extensions of Java")
		})

		t.Run("lists the manifests of a Java project", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, java.Declaration().Manifests, []string{
				"pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts",
			}, "the manifests of Java")
		})

		t.Run("returns the path without its extension as the unit", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, java.Declaration().Namespace("src/main/java/pkg/Store.java"), "src/main/java/pkg/Store",
				"the unit of src/main/java/pkg/Store.java")
		})

		t.Run("reports VisibilityUnknown for every name", func(t *testing.T) {
			t.Parallel()
			for _, name := range []string{"Store", "helper", "LIMIT"} {
				assert.Equal(t, java.Declaration().Visibility(name), sema.VisibilityUnknown, "the visibility of "+name)
			}
		})
	})

	t.Run("Server", func(t *testing.T) {
		t.Parallel()

		t.Run("declares that the server returns the imported file at an import", func(t *testing.T) {
			t.Parallel()
			assert.True(t, java.Server().Imports, "Imports of jdtls")
		})

		t.Run("keeps the Eclipse metadata of a project out of its root", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, java.Server().Command, []string{"jdtls", outside}, "the command of jdtls")
		})

		t.Run("opens a file as java", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, java.Server().LanguageID, lsp.IdentityJava, "the language identifier of jdtls")
		})

		t.Run("waits two minutes for the build import", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, java.Server().Loading, 2*time.Minute, "the loading time of jdtls")
		})

		t.Run("extracts a method by the kind of its code action", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, java.Server().Extracts, lsp.Refactor{Kind: "refactor.extract.function"},
				"the extraction of jdtls")
		})

		t.Run("names the sections under which jdtls requests the indentation of a file", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, java.Server().Indentation, lsp.Indentation{Size: tabSize, Spaces: insertSpaces},
				"the indentation sections of jdtls")
		})
	})
}
