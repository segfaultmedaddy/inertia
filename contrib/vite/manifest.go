package vite

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
)

type rawManifest = map[string]*ManifestEntry

// Manifest represents a parsed Vite build manifest (manifest.json).
// It maps entry points to their compiled assets and dependencies.
type Manifest struct {
	raw rawManifest
}

// ManifestEntry describes a single asset in the Vite build manifest.
// It contains the asset's output path, dependencies, and metadata.
type ManifestEntry struct {
	Source         string   `json:"src"`
	File           string   `json:"file"`
	Name           string   `json:"name"`
	CSS            []string `json:"css"`
	Assets         []string `json:"assets"`
	Imports        []string `json:"imports"`
	DynamicImports []string `json:"dynamicImports"`
	IsEntry        bool     `json:"isEntry"`
	IsDynamicEntry bool     `json:"isDynamicEntry"`
}

// HTML resolves a manifest entry and returns all required CSS and JS tags.
//
// It recursively walks the import graph to include all dependencies.
// Returns (css, js, error) where css and js are ready-to-use HTML tags.
func (m *Manifest) HTML(name string) ([]template.HTML, []template.HTML, error) {
	seen := make(map[string]bool)

	entry, ok := m.raw[name]
	if !ok {
		return nil, nil, fmt.Errorf("inertia: entry %s not found in manifest", name)
	}

	var (
		css []template.HTML
		js  []template.HTML
	)

	var walk func(*ManifestEntry)

	walk = func(e *ManifestEntry) {
		if seen[e.Name] {
			return
		}

		seen[e.Name] = true

		for _, link := range e.CSS {
			//nolint:gosec
			css = append(css, template.HTML(fmt.Sprintf(
				`<link rel="stylesheet" href="%s" />`, link)))
		}

		for _, link := range e.Assets {
			//nolint:gosec
			js = append(js, template.HTML(fmt.Sprintf(
				`<script type="module" src="%s"></script>`, link)))
		}

		for _, i := range e.Imports {
			walk(m.raw[i])
		}
	}

	walk(entry)

	return css, js, nil
}

// ParseManifest parses a Vite build manifest from JSON bytes.
//
// The manifest maps entry point names to their compiled assets and dependencies.
func ParseManifest(b []byte) (*Manifest, error) {
	var raw rawManifest

	err := json.Unmarshal(b, &raw)
	if err != nil {
		return nil, fmt.Errorf("inertia: failed to unmarshal manifest: %w", err)
	}

	return &Manifest{raw: raw}, nil
}

// ParseManifestFromFS reads and parses a Vite manifest from a file system.
func ParseManifestFromFS(fsys fs.FS, path string) (*Manifest, error) {
	b, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("inertia: failed to read manifest file: %w", err)
	}

	return ParseManifest(b)
}
