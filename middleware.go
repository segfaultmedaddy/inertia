package inertia

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"go.inout.gg/foundations/debug"
	"go.inout.gg/foundations/must"

	"go.segfaultmedaddy.com/inertia/internal/inertiaheader"
)

type ctxKey struct{}

//nolint:gochecknoglobals
var kCtxKey = ctxKey{}

// https://inertiajs.com/redirects#303-response-code
//
//nolint:gochecknoglobals
var seeOtherMethods = []string{http.MethodPatch, http.MethodPut, http.MethodDelete}

var DefaultEmptyResponseHandler = func(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "Empty response", http.StatusNoContent)
}

var DefaultVersionMismatchHandler = func(w http.ResponseWriter, r *http.Request) {
	Location(w, r, r.RequestURI)
}

// MiddlewareConfig configures the behavior of the Inertia.js middleware.
type MiddlewareConfig struct {
	// EmptyResponseHandler is called when a handler produces no response body.
	//
	// If nil, defaults to returning HTTP 204 No Content with an error message.
	EmptyResponseHandler http.HandlerFunc

	// VersionMismatchHandler is called when the client's asset version doesn't match the server's.
	//
	// If nil, defaults to redirecting the client to the current URL to reload the page with fresh assets.
	VersionMismatchHandler http.HandlerFunc
}

func (m *MiddlewareConfig) defaults() {
	if m.EmptyResponseHandler == nil {
		m.EmptyResponseHandler = DefaultEmptyResponseHandler
	}

	if m.VersionMismatchHandler == nil {
		m.VersionMismatchHandler = DefaultVersionMismatchHandler
	}

	debug.Assert(m.EmptyResponseHandler != nil, "EmptyResponseHandler must be set")
	debug.Assert(m.VersionMismatchHandler != nil, "VersionMismatchHandler must be set")
}

// NewMiddleware creates an HTTP middleware that enables Inertia.js protocol handling.
// It intercepts requests to determine if they are Inertia requests, handles version validation,
// and manages response formatting (JSON for subsequent Inertia requests, HTML otherwise).
//
// The middleware automatically handles HTTP 302 redirects by converting them to 303 for PUT/PATCH/DELETE
// requests as per the Inertia.js specification.
//
// Once the middleware is set up, Render can be used to create Inertia responses.
func NewMiddleware(renderer *Renderer, opts ...func(*MiddlewareConfig)) func(http.Handler) http.Handler {
	var config MiddlewareConfig
	for _, opt := range opts {
		opt(&config)
	}

	config.defaults()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			r = r.WithContext(context.WithValue(r.Context(), kCtxKey, renderer))

			h.Set(inertiaheader.HeaderVary, inertiaheader.HeaderXInertia)

			if !isInertiaRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			clientVersion := r.Header.Get(inertiaheader.HeaderXInertiaVersion)
			serverVersion := renderer.Version()
			if clientVersion != serverVersion {
				config.VersionMismatchHandler(w, r)
				return
			}

			rww := newResponseWriter(w)
			next.ServeHTTP(rww, r)

			if rww.statusCode == http.StatusFound &&
				slices.Contains(seeOtherMethods, r.Method) {
				rww.WriteHeader(http.StatusSeeOther)
			}

			if rww.Empty() {
				config.EmptyResponseHandler(w, r)
				return
			}

			rww.flush()
		})
	}
}

// RenderContext contains all configuration and data for rendering an Inertia.js page response.
// It includes props, validation errors, history management options, and performance settings.
type RenderContext struct {
	// T is custom data passed to the HTML template via html/template.
	T any

	// Props are the properties sent to the page component.
	Props []Prop

	// ErrorBag specifies the validation error bag name for scoped error handling.
	ErrorBag string

	// ValidationErrorer contains validation errors to be sent to the client.
	ValidationErrorer []ValidationErrorer

	// EncryptHistory instructs the client to encrypt the history state for this page.
	EncryptHistory bool

	// ClearHistory instructs the client to clear the history stack.
	ClearHistory bool

	// Concurrency sets the maximum number of concurrent prop resolutions for this page.
	// If 0, uses the renderer's default. Negative values mean sequential resolution.
	Concurrency int
}

// NewRenderContext creates a RenderContext configured with the provided options.
// Options are applied in order and can be combined to build up the desired page state.
func NewRenderContext(opts ...Option) RenderContext {
	var ctx RenderContext
	for _, opt := range opts {
		opt(&ctx)
	}

	return ctx
}

// AddValidationErrorer appends validation errors to the context.
// Multiple calls accumulate errors into a single error bag.
func (ctx *RenderContext) AddValidationErrorer(err ValidationErrorer) {
	if ctx.ValidationErrorer == nil {
		ctx.ValidationErrorer = make([]ValidationErrorer, 0, 1)
	}

	ctx.ValidationErrorer = append(ctx.ValidationErrorer, err)
}

// Option is a function that configures a RenderContext.
type Option func(*RenderContext)

// WithClearHistory instructs the client to clear its history stack when rendering this page.
func WithClearHistory() Option {
	return func(opt *RenderContext) { opt.ClearHistory = true }
}

// WithEncryptHistory instructs the client to encrypt the history state.
func WithEncryptHistory() Option {
	return func(opt *RenderContext) { opt.EncryptHistory = true }
}

// WithProps adds properties to the page component.
// Multiple calls append additional props to the existing set.
func WithProps(props Proper) Option {
	return func(renderCtx *RenderContext) {
		if props == nil {
			return
		}

		if renderCtx.Props == nil {
			renderCtx.Props = make([]Prop, 0, props.Len())
		}

		renderCtx.Props = append(renderCtx.Props, props.Props()...)
	}
}

// WithValidationErrors adds validation errors to be displayed on the page.
// Multiple calls append errors to the same or different error bags.
//
// The errorBag parameter allows scoping errors to specific forms on the same page.
func WithValidationErrors(errorers ValidationErrorer, errorBag string) Option {
	return func(renderCtx *RenderContext) {
		if errorers == nil {
			return
		}

		renderCtx.AddValidationErrorer(errorers)
		renderCtx.ErrorBag = errorBag
	}
}

// WithConcurrency sets the maximum number of props that can be resolved concurrently for this page.
// This only affects props marked as concurrent.
//
// A value of 0 uses the renderer's default concurrency level.
// Negative values allow unlimited concurrent resolution.
func WithConcurrency(concurrency int) Option {
	return func(renderCtx *RenderContext) {
		renderCtx.Concurrency = concurrency
	}
}

// Render sends an Inertia.js page response with the specified component and context.
// It automatically detects whether to send JSON (for Inertia requests) or HTML (for full page loads).
//
// This function requires the Inertia middleware to be installed in the request chain.
// Returns an error if the middleware is not found or if rendering fails.
func Render(w http.ResponseWriter, r *http.Request, componentName string, rCtx RenderContext) error {
	render, ok := r.Context().Value(kCtxKey).(*Renderer)
	if !ok {
		return errors.New(
			"inertia: renderer not found in request context - did you forget to use the middleware?",
		)
	}

	if err := render.Render(w, r, componentName, rCtx); err != nil {
		return err
	}

	return nil
}

// MustRender is like Render, but panics if an error occurs.
func MustRender(w http.ResponseWriter, req *http.Request, name string, r RenderContext) {
	must.Must1(Render(w, req, name, r))
}
