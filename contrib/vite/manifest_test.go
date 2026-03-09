package vite

import (
	"html/template"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseManifest(t *testing.T) {
	t.Parallel()

	t.Run("successful parsing", func(t *testing.T) {
		t.Parallel()

		// arrange
		content, err := os.ReadFile("testdata/manifest.json")
		require.NoError(t, err)

		// act
		manifest, err := ParseManifest(content)

		// assert
		require.NoError(t, err)

		sharedJS, ok := manifest.raw["_shared-B7PI925R.js"]
		require.True(t, ok)
		assert.Equal(t, &ManifestEntry{
			File: "assets/shared-B7PI925R.js",
			Name: "shared",
			CSS:  []string{"assets/shared-ChJ_j-JJ.css"},
		}, sharedJS)

		bar, ok := manifest.raw["views/bar.js"]
		require.True(t, ok)
		assert.Equal(t, &ManifestEntry{
			File:           "assets/bar-gkvgaI9m.js",
			Name:           "bar",
			Source:         "views/bar.js",
			IsEntry:        true,
			Imports:        []string{"_shared-B7PI925R.js"},
			DynamicImports: []string{"baz.js"},
		}, bar)

		foo, ok := manifest.raw["views/foo.js"]
		require.True(t, ok)
		assert.Equal(t, &ManifestEntry{
			File:    "assets/foo-BRBmoGS9.js",
			Name:    "foo",
			Source:  "views/foo.js",
			IsEntry: true,
			Imports: []string{"_shared-B7PI925R.js"},
			CSS:     []string{"assets/foo-5UjPuW-k.css"},
		}, foo)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()

		// arrange
		invalidJSON := []byte(`{invalid json}`)

		// act
		_, err := ParseManifest(invalidJSON)

		// assert
		assert.Error(t, err)
	})
}

func TestParseManifestFromFS(t *testing.T) {
	t.Parallel()

	t.Run("successful parsing", func(t *testing.T) {
		t.Parallel()

		// arrange
		dir := os.DirFS("testdata")

		// act
		manifest, err := ParseManifestFromFS(dir, "manifest.json")

		// assert
		require.NoError(t, err)

		sharedJS, ok := manifest.raw["_shared-B7PI925R.js"]
		require.True(t, ok)
		assert.Equal(t, &ManifestEntry{
			File: "assets/shared-B7PI925R.js",
			Name: "shared",
			CSS:  []string{"assets/shared-ChJ_j-JJ.css"},
		}, sharedJS)

		foo, ok := manifest.raw["views/foo.js"]
		require.True(t, ok)
		assert.Equal(t, &ManifestEntry{
			File:    "assets/foo-BRBmoGS9.js",
			Name:    "foo",
			Source:  "views/foo.js",
			IsEntry: true,
			Imports: []string{"_shared-B7PI925R.js"},
			CSS:     []string{"assets/foo-5UjPuW-k.css"},
		}, foo)
	})

	t.Run("file not found", func(t *testing.T) {
		t.Parallel()

		// arrange
		dir := os.DirFS("testdata")

		// act
		_, err := ParseManifestFromFS(dir, "nonexistent.json")

		// assert
		assert.Error(t, err)
	})

	t.Run("invalid file path", func(t *testing.T) {
		t.Parallel()

		// arrange
		dir := os.DirFS(filepath.Join("testdata", "nonexistent"))

		// act
		_, err := ParseManifestFromFS(dir, "manifest.json")

		// assert
		assert.Error(t, err)
	})
}

func TestManifestHTML(t *testing.T) {
	t.Parallel()

	// arrange
	content, err := os.ReadFile("testdata/manifest.json")
	require.NoError(t, err)

	manifest, err := ParseManifest(content)
	require.NoError(t, err)

	t.Run("resolves CSS and JS tags for entry with imports", func(t *testing.T) {
		t.Parallel()

		// act
		css, _, err := manifest.HTML("views/foo.js")

		// assert
		require.NoError(t, err)
		assert.Contains(t, css, template.HTML(`<link rel="stylesheet" href="assets/foo-5UjPuW-k.css" />`))
		assert.Contains(t, css, template.HTML(`<link rel="stylesheet" href="assets/shared-ChJ_j-JJ.css" />`))
	})

	t.Run("resolves imported dependency CSS", func(t *testing.T) {
		t.Parallel()

		// act
		css, _, err := manifest.HTML("views/bar.js")

		// assert
		require.NoError(t, err)
		assert.Contains(t, css, template.HTML(`<link rel="stylesheet" href="assets/shared-ChJ_j-JJ.css" />`))
	})

	t.Run("entry not found returns error", func(t *testing.T) {
		t.Parallel()

		// act
		_, _, err := manifest.HTML("nonexistent.js")

		// assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonexistent.js")
	})
}
