package inertia

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"runtime"
	"slices"
	"strings"

	"github.com/alitto/pond/v2"
	"github.com/go-json-experiment/json"
	"go.inout.gg/foundations/debug"
	"go.inout.gg/foundations/must"

	"go.segfaultmedaddy.com/inertia/internal/inertiabase"
	"go.segfaultmedaddy.com/inertia/internal/inertiaheader"
	"go.segfaultmedaddy.com/inertia/internal/inertiaredirect"
)

const (
	// DefaultRootViewID is the default root HTML element ID to which
	// the Inertia.js app is mounted.
	DefaultRootViewID = "app"
)

// DefaultConcurrency is the default concurrency level for props resolution
// marked as concurrently resolvable.
var DefaultConcurrency = runtime.GOMAXPROCS(0) //nolint:gochecknoglobals

// Page represents an Inertia.js page that is sent to the client.
type Page = inertiabase.Page

// Config configures the Renderer behavior and capabilities.
type Config struct {
	// SSRClient enables server-side rendering of Inertia pages.
	//
	// If nil, only client-side rendering is used.
	SSRClient SSRClient

	// RootViewAttrs are HTML attributes applied to the root element.
	RootViewAttrs map[string]string

	// Version identifies the current asset version (e.g., build hash or timestamp).
	Version string

	// RootViewID is the HTML element ID where the Inertia app mounts.
	//
	// Defaults to "app" if not specified.
	RootViewID string

	// JSONMarshalOptions configures JSON serialization for page props and data.
	JSONMarshalOptions []json.Options

	// Concurrency sets the default maximum number of props that can be resolved concurrently.
	// It only affects props marked as concurrent.
	//
	// Defaults to runtime.GOMAXPROCS(0).
	Concurrency int
}

func (c *Config) defaults() {
	c.RootViewID = cmp.Or(c.RootViewID, DefaultRootViewID)
	c.Concurrency = cmp.Or(c.Concurrency, DefaultConcurrency)

	debug.Assert(c.RootViewID != "", "RooViewID must be non-empty string")
}

// Renderer handles Inertia.js page responses, supporting both client-side and server-side rendering.
// It manages HTML template rendering, JSON serialization, and prop resolution.
//
// Create a Renderer using New or FromFS constructor functions.
type Renderer struct {
	ssrClient          SSRClient
	jsonMarshalOptions []json.Options
	t                  *template.Template
	rootViewID         string
	version            string
	rootViewAttrs      []pair[[]byte, []byte]
	concurrency        int
}

// New creates a Renderer with the provided HTML template and configuration.
//
// If config is nil, default values are used:
//   - RootViewID: "app"
//   - Concurrency: GOMAXPROCS(0)
func New(t *template.Template, config *Config) *Renderer {
	if config == nil {
		//nolint:exhaustruct
		config = &Config{}
	}

	config.defaults()

	attrs := make([]pair[[]byte, []byte], 0, len(config.RootViewAttrs))
	for key, value := range config.RootViewAttrs {
		attrs = append(attrs, pair[[]byte, []byte]{[]byte(key), []byte(value)})
	}

	r := &Renderer{
		t:                  t,
		ssrClient:          config.SSRClient,
		jsonMarshalOptions: config.JSONMarshalOptions,
		version:            config.Version,
		rootViewID:         config.RootViewID,
		rootViewAttrs:      attrs,
		concurrency:        config.Concurrency,
	}

	debug.Assert(r.t != nil, "expected t to be defined")
	debug.Assert(r.rootViewID != "", "expected RootViewID to be defined")

	return r
}

// FromFS creates a Renderer by loading an HTML template from a file system.
//
// If config is nil, default values are used.
func FromFS(fsys fs.FS, path string, config *Config) (*Renderer, error) {
	debug.Assert(fsys != nil, "expected fsys to be defined")
	debug.Assert(path != "", "expected path to be defined")

	t := template.New("inertia")

	t, err := t.ParseFS(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("inertia: failed to parse templates: %w", err)
	}

	return New(t, config), nil
}

// MustFromFS is like FromFS, but panics if an error occurs.
func MustFromFS(fsys fs.FS, path string, config *Config) *Renderer {
	return must.Must(FromFS(fsys, path, config))
}

// Version returns the current asset version string used for client version validation.
func (r *Renderer) Version() string { return r.version }

// Render sends an Inertia page response, automatically choosing the format:
//   - JSON for Inertia requests (XHR navigation)
//   - HTML for initial page loads or non-Inertia requests
//
// The renderCtx configures props, validation errors, and other page-specific settings.
func (r *Renderer) Render(w http.ResponseWriter, req *http.Request, name string, renderCtx RenderContext) error {
	renderCtx.Concurrency = max(cmp.Or(renderCtx.Concurrency, r.concurrency), 0)

	page, err := r.newPage(req, name, renderCtx)
	if err != nil {
		return err
	}

	if isInertiaRequest(req) {
		d("Received inertia request, sending JSON response: %s",
			req.Header.Get(inertiaheader.HeaderReferer))

		w.Header().Set(inertiaheader.HeaderXInertia, "true")
		w.Header().Set(inertiaheader.HeaderContentType, inertiaheader.ContentTypeJSON)
		w.WriteHeader(http.StatusOK)

		if err := json.MarshalWrite(w, page, r.jsonMarshalOptions...); err != nil {
			return fmt.Errorf("inertia: failed to encode JSON response: %w", err)
		}

		return nil
	}

	w.Header().Set(inertiaheader.HeaderContentType, inertiaheader.ContentTypeHTML)
	w.WriteHeader(http.StatusOK)

	data := TemplateData{T: renderCtx.T, InertiaHead: "", InertiaBody: ""}

	if r.ssrClient != nil {
		ssrData, err := r.ssrClient.Render(req.Context(), page)
		if err != nil {
			return fmt.Errorf("inertia: failed to render SSR data: %w", err)
		}

		data.InertiaHead = template.HTML(ssrData.Head) //nolint:gosec
		data.InertiaBody = template.HTML(ssrData.Body) //nolint:gosec
	} else {
		body, err := r.makeRootView(page)
		if err != nil {
			return fmt.Errorf("inertia: failed to create an HTML container: %w", err)
		}

		data.InertiaBody = body
	}

	if err := r.t.Execute(w, &data); err != nil {
		return fmt.Errorf("inertia: failed to execute HTML template: %w", err)
	}

	return nil
}

func (r *Renderer) newPage(req *http.Request, componentName string, renderCtx RenderContext) (*Page, error) {
	rawProps := make([]Prop, 0, len(renderCtx.Props)+1)
	rawProps = append(rawProps, renderCtx.Props...)
	rawProps = append(rawProps, r.makeValidationErrors(renderCtx.ValidationErrorer, renderCtx.ErrorBag))

	props, err := r.makeProps(req, componentName, rawProps, renderCtx.Concurrency)
	if err != nil {
		return nil, err
	}

	deferredProps := r.makeDeferredProps(req, componentName, rawProps)
	mergeProps := r.makeMergeProps(
		rawProps,
		extractHeaderValueList(req.Header.Get(inertiaheader.HeaderXInertiaReset)),
	)

	return &Page{
		Component:      componentName,
		Props:          props,
		DeferredProps:  deferredProps,
		MergeProps:     mergeProps,
		URL:            req.RequestURI,
		Version:        r.version,
		ClearHistory:   renderCtx.ClearHistory,
		EncryptHistory: renderCtx.EncryptHistory,
	}, nil
}

// makeRootView creates a root view element with the given page data.
func (r *Renderer) makeRootView(page *Page) (template.HTML, error) {
	var w strings.Builder

	_ = must.Must(w.WriteString(`<div id="`))
	_ = must.Must(w.WriteString(r.rootViewID))
	_ = must.Must(w.WriteRune('"'))
	_ = must.Must(w.WriteRune(' '))

	_ = must.Must(w.WriteString(`data-page="`))

	pageBytes, err := json.Marshal(page, r.jsonMarshalOptions...)
	if err != nil {
		return "", fmt.Errorf("inertia: an error occurred while rendering page: %w", err)
	}

	template.HTMLEscape(&w, pageBytes)
	_ = must.Must(w.WriteRune('"'))
	_ = must.Must(w.WriteRune(' '))

	if r.rootViewAttrs != nil {
		for _, kv := range r.rootViewAttrs {
			// Skip the data-page attribute as it's already set.
			if bytes.Equal(kv.key, []byte("data-page")) {
				continue
			}

			_ = must.Must(w.Write(kv.key))
			_ = must.Must(w.WriteRune('='))
			_ = must.Must(w.WriteRune('"'))
			template.HTMLEscape(&w, kv.value)
			_ = must.Must(w.WriteRune('"'))
			_ = must.Must(w.WriteRune(' '))
		}
	}

	_ = must.Must(w.WriteString(`></div>`))

	//nolint:gosec
	return template.HTML(w.String()), nil
}

func (r *Renderer) makeProps(
	req *http.Request,
	componentName string,
	props []Prop,
	concurrency int,
) (map[string]any, error) {
	ctx := req.Context()

	// If the request is a partial, we need to filter the props.
	if isPartialComponentRequest(req, componentName) {
		whitelist := extractHeaderValueList(req.Header.Get(
			inertiaheader.HeaderXInertiaPartialData))
		blacklist := extractHeaderValueList(req.Header.Get(
			inertiaheader.HeaderXInertiaPartialExcept))

		return r.resolvePartialComponentRequest(ctx, props, whitelist, blacklist, concurrency)
	}

	m := make(map[string]any, len(props))

	for _, prop := range props {
		// Skip lazy (deferred, optional) props on the first render.
		if prop.lazy {
			continue
		}

		val, err := prop.value(ctx)
		if err != nil {
			return nil, fmt.Errorf("inertia: failed to resolve prop %s: %w", prop.key, err)
		}

		m[prop.key] = val
	}

	return m, nil
}

func (r *Renderer) resolvePartialComponentRequest(
	ctx context.Context,
	props []Prop,
	whitelist, blacklist []string,
	concurrency int,
) (map[string]any, error) {
	m := make(map[string]any, len(props))
	concurrentProps := make([]Prop, 0, len(props))

	for _, prop := range props {
		key := prop.key
		if prop.ignorable {
			// It should be fine to go through slices here, as the number of props is expected to be small.
			if len(whitelist) > 0 && !slices.Contains(whitelist, key) ||
				len(blacklist) > 0 && slices.Contains(blacklist, key) {
				continue
			}
		}

		if prop.concurrent {
			concurrentProps = append(concurrentProps, prop)
		} else {
			val, err := prop.value(ctx)
			if err != nil {
				return nil, fmt.Errorf("inertia: failed to resolve prop %s: %w", prop.key, err)
			}

			m[key] = val
		}
	}

	if len(concurrentProps) > 0 {
		pool := pond.NewResultPool[pair[string, any]](concurrency)
		group := pool.NewGroupContext(ctx)

		for _, prop := range concurrentProps {
			group.SubmitErr(func() (pair[string, any], error) {
				var kv pair[string, any]

				val, err := prop.value(ctx)
				if err != nil {
					return kv, fmt.Errorf(
						"inertia: failed to resolve prop %s: %w",
						prop.key,
						err,
					)
				}

				kv.key = prop.key
				kv.value = val

				return kv, nil
			})
		}

		result, err := group.Wait()
		if err != nil {
			return nil, fmt.Errorf("inertia: failed to resolve concurrent props: %w", err)
		}

		for i, prop := range concurrentProps {
			m[prop.key] = result[i].value
		}
	}

	return m, nil
}

// makeDeferredProps creates a map of deferred props that should be resolved
// on the client side.
func (r *Renderer) makeDeferredProps(req *http.Request, componentName string, props []Prop) map[string][]string {
	// If the request is partial, then the client already got information
	// about the deferred props in the initial request so we don't need to
	// send them again.
	if isPartialComponentRequest(req, componentName) {
		return nil
	}

	m := make(map[string][]string, len(props))

	for _, prop := range props {
		if !prop.deferred {
			continue
		}

		if _, ok := m[prop.group]; !ok {
			m[prop.group] = []string{}
		}

		m[prop.group] = append(m[prop.group], prop.key)
	}

	return m
}

// makeMergeProps creates a list of props that should be merged instead of
// being replaced on the client side.
func (r *Renderer) makeMergeProps(props []Prop, blacklist []string) []string {
	mergeProps := make([]string, 0, len(props))

	for _, p := range props {
		if len(blacklist) > 0 && slices.Contains(blacklist, p.key) || !p.mergeable {
			continue
		}

		mergeProps = append(mergeProps, p.key)
	}

	return mergeProps
}

func (r *Renderer) makeValidationErrors(errorers []ValidationErrorer, errorBag string) Prop {
	m := make(map[string]string)

	for _, errorer := range errorers {
		errs := errorer.ValidationErrors()
		for _, err := range errs {
			m[err.Field()] = err.Error()
		}
	}

	if errorBag != DefaultErrorBag {
		return NewAlways(errorBag, map[string]map[string]string{"errors": m})
	}

	return NewAlways("errors", m)
}

// TemplateData contains the data passed to the HTML template during rendering.
type TemplateData struct {
	// T is custom application data available to the template.
	T any

	// InertiaHead contains SSR-generated head elements (title, meta tags, etc.).
	InertiaHead template.HTML

	// InertiaBody contains the rendered page content.
	InertiaBody template.HTML
}

// Location redirects to an external URL outside of the Inertia app.
//
// For Inertia requests, it uses a 409 Conflict response with X-Inertia-Location header.
// For regular requests, it performs a standard HTTP redirect.
func Location(w http.ResponseWriter, r *http.Request, url string) {
	if isInertiaRequest(r) {
		h := w.Header()

		h.Del(inertiaheader.HeaderVary)
		h.Del(inertiaheader.HeaderXInertia)
		h.Set(inertiaheader.HeaderXInertiaLocation, url) // redirect URL
		w.WriteHeader(http.StatusConflict)               // 409 Conflict

		return
	}

	inertiaredirect.Redirect(w, r, url)
}

// Redirect sends a redirect response to the Inertia app page.
func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	inertiaredirect.Redirect(w, r, url)
}

// ErrorBagFromRequest extracts the error bag name from the X-Inertia-Error-Bag header.
//
// Returns the default error bag (empty string) if the header is not present.
// Used to scope validation errors to specific forms on a page.
func ErrorBagFromRequest(r *http.Request) string {
	errorBag := r.Header.Get(inertiaheader.HeaderXInertiaErrorBag)
	if errorBag == "" {
		return DefaultErrorBag
	}

	return errorBag
}

// isInertiaRequest checks if the request is made by Inertia.js.
func isInertiaRequest(req *http.Request) bool {
	return req.Header.Get(inertiaheader.HeaderXInertia) == "true"
}

// isPartialComponentRequest checks if the request is a partial component request
// matching the given componentName.
func isPartialComponentRequest(req *http.Request, componentName string) bool {
	return req.Header.Get(inertiaheader.HeaderXInertiaPartialComponent) == componentName
}

// extractHeaderValueList extracts a list of values from a comma-separated inertiaheader.Header value.
func extractHeaderValueList(h string) []string {
	if h == "" {
		return nil
	}

	fields := strings.Split(h, ",")
	for i, f := range fields {
		fields[i] = strings.TrimSpace(f)
	}

	return fields
}

// pair is a key-value pair.
type pair[K any, V any] struct {
	key   K
	value V
}
