// Package vite provides Vite integration for Inertia.js applications.
//
// In development mode, it injects the Vite client and React Refresh scripts.
// In production mode, it resolves assets from the Vite manifest file to include
// the correct hashed filenames and dependencies.
package vite

import (
	"cmp"
	"fmt"
	"html/template"
	"io/fs"

	"go.inout.gg/foundations/must"
)

const DefaultViteAddress = "http://localhost:5173"

type Config struct {
	Manifest     Manifest
	TemplateName string
	ViteAddress  string
}

func (c *Config) defaults() {
	c.ViteAddress = cmp.Or(c.ViteAddress, DefaultViteAddress)
	c.TemplateName = cmp.Or(c.TemplateName, "inertia")
}

// NewTemplate creates an html/template with Vite support from a template string.
//
// Available template functions and sub-templates:
//   - {{viteResource "path/to/file.js"}}: Include an asset (dev: proxied URL, prod: manifest-resolved)
//   - {{template "viteClient"}}: Vite development client (dev only, blank in production)
//   - {{template "viteReactRefresh"}}: React Fast Refresh support (dev only, blank in production)
//
// In development mode, assets are loaded from the Vite dev server at ViteAddress.
// In production mode (build tag: -tags=production), assets are resolved from the manifest.
func NewTemplate(content string, config *Config) (*template.Template, error) {
	if config == nil {
		//nolint:exhaustruct
		config = &Config{}
	}

	config.defaults()

	t := newTemplate(config)
	if _, err := t.Parse(content); err != nil {
		return nil, fmt.Errorf("inertia: failed to parse template: %w", err)
	}

	return t, nil
}

// Must is like New but panics on error.
func Must(content string, c *Config) *template.Template {
	return must.Must(NewTemplate(content, c))
}

// FromFS creates an html/template with Vite support by loading templates from a file system.
// See NewTemplate for available template functions and behavior.
func FromFS(fsys fs.FS, path string, cfg *Config) (*template.Template, error) {
	if cfg == nil {
		//nolint:exhaustruct
		cfg = &Config{}
	}

	cfg.defaults()

	t := newTemplate(cfg)
	if _, err := t.ParseFS(fsys, path); err != nil {
		return nil, fmt.Errorf("inertia: failed to parse template: %w", err)
	}

	return t, nil
}
