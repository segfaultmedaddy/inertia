// Package vite provides a minimal integration for Vite.
// It adds support for Vite Client and Vite React Refresh in development mode.
// It also provides a support for bundling Vite resources declared
// in the Vite manifest file.
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

// NewTemplate creates a new template from a string.
//
// The resulting template will have built-in support for Vite.
// To include Vite React Refresh, use {{template "viteReactRefresh"}}
// and Vite client, use {{template "viteClient"}}.
// To include a Vite resource, use {{viteResource "path/to/resource.js"}}.
// When running with -tags=production, "viteClient" and "viteReactRefresh"
// templates are blank.
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

// FromFS creates a new template from a file system.
// See New for more information.
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
