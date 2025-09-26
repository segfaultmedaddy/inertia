// inertiaframe implements an opinionated framework around Go's HTTP and Inertia
// library, abstracting out protocol-level details and providing a simple
// message-based API.
package inertiaframe

import (
	"cmp"
	"context"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"mime"
	"net/http"

	"github.com/go-playground/form/v4"
	"go.inout.gg/foundations/debug"
	"go.inout.gg/foundations/http/httperror"
	"go.inout.gg/foundations/http/httpmiddleware"
	"go.inout.gg/foundations/must"

	"go.inout.gg/inertia"
	"go.inout.gg/inertia/internal/inertiaheader"
	"go.inout.gg/inertia/internal/inertiaredirect"
)

var d = debug.Debuglog("inertiaframe") //nolint:gochecknoglobals

var DefaultFormDecoder = form.NewDecoder() //nolint:gochecknoglobals

var ErrEmptyResponse = errors.New("inertiaframe: empty response")

var (
	_ RawResponseWriter = (*redirectMessage)(nil)
	_ RawResponseWriter = (*redirectBackMessage)(nil)
	_ RawResponseWriter = (*externalRedirectMessage)(nil)
	_ Response          = (*resp)(nil)
)

type kCtx struct{}

var kCtxKey = kCtx{} //nolint:gochecknoglobals

// WithProps sets the props on the request context and returns
// the updated request.
//
// WithProps can be used to gather props in multiple places, e.g., in middleware.
//
// Any overlapping props between the shared context and the response props
// will be replaced with the response props.
//
// Prefer to use the response props directly instead of using this function,
// and opt in only when necessary.
func WithProps(r *http.Request, props inertia.Proper) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), kCtxKey, props))
}

// RedirectBack redirects the user back to the previous page.
//
// The previous page is determined from the Referer header and
// falls back to the session if the header is not present.
func RedirectBack(w http.ResponseWriter, r *http.Request) {
	referer := r.Header.Get(inertiaheader.HeaderReferer)
	if referer == "" {
		sess, err := sessionFromRequest(r)
		if err != nil {
			d("failed to get session from request, using default '/'")

			referer = "/"
		} else {
			referer = sess.Referer()
		}
	}

	d("redirecting back to %s", referer)

	inertiaredirect.Redirect(w, r, referer)
}

// DefaultValidationErrorHandler is a default error handler for validation errors.
//
// It saves flash messages and redirects back to the previous page.
func DefaultValidationErrorHandler(w http.ResponseWriter, r *http.Request, errorer inertia.ValidationErrorer) {
	errorBag := inertia.ErrorBagFromRequest(r)
	sess := must.Must(sessionFromRequest(r))

	sess.ErrorBag_ = errorBag
	sess.ValidationErrors_ = errorer.ValidationErrors()

	must.Must1(sess.Save(w))

	RedirectBack(w, r)
}

//nolint:gochecknoglobals
var DefaultErrorHandler httperror.ErrorHandler = httperror.ErrorHandlerFunc(
	func(w http.ResponseWriter, r *http.Request, err error) {
		var errorer inertia.ValidationErrorer
		if errors.As(err, &errorer) {
			DefaultValidationErrorHandler(w, r, errorer)
			return
		}

		httperror.DefaultErrorHandler(w, r, err)
	},
)

const (
	mediaTypeJSON      = "application/json"
	mediaTypeForm      = "application/x-www-form-urlencoded"
	mediaTypeMultipart = "multipart/form-data"
)

// Request is a request sent by a client.
type Request[M any] struct {
	// Message is a decoded message sent by a client.
	//
	// Message can implement RawRequestExtractor to intercept request data extraction.
	Message *M
}

// newRequest creates a new request.
func newRequest[M any](m M) *Request[M] {
	return &Request[M]{Message: &m}
}

// ResponseOptions is a configuration for inertia response.
type ResponseOptions struct {
	// ClearHistory determines whether the history should be cleared by
	// the client.
	ClearHistory bool

	// EncryptHistory determines whether the history should be encrypted by
	// the client.
	EncryptHistory bool

	// Concurrency determines the maximum number of concurrent resolutions of lazy
	// props that can be made during response resolution.
	Concurrency int
}

type ResponseOption func(*ResponseOptions)

// ResponseOptioner is an interface that
type ResponseOptioner interface {
	Options() ResponseOptions
}

type externalRedirectMessage struct{ url string }

func (m *externalRedirectMessage) Proper() inertia.Proper { return nil }
func (m *externalRedirectMessage) Component() string      { return "" }

func (m *externalRedirectMessage) Write(w http.ResponseWriter, r *http.Request) error {
	inertia.Location(w, r, m.url)
	return nil
}

// NewExternalRedirectResponse creates a new response that redirects the client to an
// external URL.
//
// External URL is any URL that is not powered by Inertia.js.
func NewExternalRedirectResponse(url string) Response {
	return &externalRedirectMessage{url: url}
}

type redirectBackMessage struct{}

func (m *redirectBackMessage) Proper() inertia.Proper { return nil }
func (m *redirectBackMessage) Component() string      { return "" }

func (m *redirectBackMessage) Write(w http.ResponseWriter, r *http.Request) error {
	RedirectBack(w, r)
	return nil
}

// NewRedirectBackResponse creates a new response that redirects the client
// back to the previous page.
func NewRedirectBackResponse() Response {
	return &redirectBackMessage{}
}

type redirectMessage struct{ url string }

func (m *redirectMessage) Proper() inertia.Proper { return nil }
func (m *redirectMessage) Component() string      { return "" }

func (m *redirectMessage) Write(w http.ResponseWriter, r *http.Request) error {
	RedirectBack(w, r)
	return nil
}

// NewRedirectResponse creates a new response that redirects the client to the
// specified URL.
func NewRedirectResponse(url string) Response {
	return &redirectMessage{url: url}
}

// Message is used to send a message to the client. It can be
// used to guide the client to render a component or redirect to a
// specific URL.
//
// If the Message implements a RawResponseWriter, the default
// behavior is prevented and the writer is used instead to
// write the response data.
//
// The Component() method must return a non-empty string.
type Message interface {
	// Component returns the component name to be rendered.
	//
	// Executor panics if Component returns an empty string,
	// unless the message implements RawResponseWriter.
	//
	// If the message is implementing RawResponseWriter, the default
	// behavior is prevented and the writer is used instead to
	// write the response data.
	Component() string
}

// Response is an interface that represents a response message.
//
// If the response implements RawResponseWriter the default
// behavior is prevented and the writer is used instead to
// write the response data.
type Response interface {
	Message
	Proper() inertia.Proper
}

// RawRequestExtractor allows to extract data from the raw http.Request.
// If a request message implements RawRequestExtractor, the default
// behavior is prevented and the extractor is used instead to
// extract the request data.
type RawRequestExtractor interface {
	// Extract extracts data from the raw http.Request.
	Extract(*http.Request) error
}

// RawResponseWriter allows to write data to the http.ResponseWriter.
// If a response message implements RawResponseWriter, the default
// behavior is prevented and the writer is used instead to
// write the response data.
type RawResponseWriter interface {
	Write(http.ResponseWriter, *http.Request) error
}

// Meta is the metadata of an endpoint.
type Meta struct {
	// HTTP method of the endpoint.
	Method string

	// HTTP path of the endpoint. It supports the same path pattern as
	// the http.ServeMux.
	Path string
}

// Validate validates the given data using the.
type Validator interface {
	Validate(any) error
}

type Endpoint[R any] interface {
	// Execute executes the endpoint for the given request.
	//
	// If the returned error can automatically be converted to an Inertia
	// error, it will be converted and passed down to the client.
	Execute(context.Context, *Request[R]) (Response, error)

	// Meta returns the metadata of the endpoint. It is used to configure
	// the endpoint's behavior when mounted on a given http.ServeMux.
	Meta() *Meta
}

// Mux is a universal interface for routing HTTP requests.
type Mux interface {
	// Handle handles the given HTTP request at the specified path.
	//
	// The pattern is a string following the http.ServeMux format:
	// "<http-method> <path>".
	Handle(pattern string, h http.Handler)
}

type MountOpts struct {
	// Middleware is the middleware used to handle requests.
	// If Middleware is nil, no middleware will be used.
	Middleware httpmiddleware.Middleware

	// Validator is the validator used to validate the request data.
	//
	// If no validator is specified requests won't be validated.
	Validator Validator

	// FormDecoder is the decoder used to parse incoming request data
	// when the request type is application/x-www-form-urlencoded or
	// multipart/form-data.
	//
	// If FormDecoder is nil, the DefaultFormDecoder will be used.
	FormDecoder *form.Decoder

	// ErrorHandler is the error handler used to handle errors.
	//
	// If ErrorHandler is nil, the DefaultErrorHandler will be used.
	ErrorHandler httperror.ErrorHandler

	// JSONUnmarshalOptions is the options used to unmarshal JSON requests.
	//
	// It can be used to customize the JSON decoding behavior, such as
	// allowing to parse protobuf message with protojson.
	JSONUnmarshalOptions []json.Options
}

// Mount mounts the executor on the given mux.
//
// Endpoint must specify the HTTP method and path via Endpoint.Meta().
// The mounted endpoint automatically handles requests with JSON and form
// data.
//
// The message M is validated using the validator specified in the MountOpts.
// Validation errors are automatically handled and passed to the client
// according to Inertia protocol.
func Mount[M any](mux Mux, e Endpoint[M], opts *MountOpts) {
	if opts == nil {
		//nolint:exhaustruct
		opts = &MountOpts{}
	}

	opts.ErrorHandler = cmp.Or(opts.ErrorHandler, DefaultErrorHandler)
	opts.FormDecoder = cmp.Or(opts.FormDecoder, DefaultFormDecoder)

	debug.Assert(e != nil, "Executor must not be nil")
	debug.Assert(opts.ErrorHandler != nil, "Executor must specify the error handler")
	debug.Assert(opts.Validator != nil, "Executor must specify the validator")

	m := e.Meta()

	debug.Assert(m.Method != "", "Executor must specify the HTTP method")
	debug.Assert(m.Path != "", "Executor must specify the HTTP path")

	pattern := fmt.Sprintf("%s %s", m.Method, m.Path)

	d("Mounting executor on pattern: %s", pattern)

	h := newHandler(e, opts.ErrorHandler, opts.Validator, opts.FormDecoder, opts.JSONUnmarshalOptions)
	if opts.Middleware != nil {
		h = opts.Middleware.Middleware(h)
	}

	mux.Handle(pattern, h)
}

// newHandler creates a new http.Handler for the given endpoint.
func newHandler[M any](
	endpoint Endpoint[M],
	errorHandler httperror.ErrorHandler,
	validator Validator,
	formDecoder *form.Decoder,
	jsonUnmarshalOptions []json.Options,
) http.Handler {
	handleError := httperror.WithErrorHandler(errorHandler)

	return handleError(httperror.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		var (
			msg       M
			renderCtx inertia.RenderContext
		)

		ctx := r.Context()

		if extract, ok := any(msg).(RawRequestExtractor); ok {
			if err := extract.Extract(r); err != nil {
				return fmt.Errorf("inertiaframe: failed to extract request data: %w", err)
			}
		} else if r.Method != http.MethodGet {
			mediaType, _, err := mime.ParseMediaType(
				r.Header.Get(inertiaheader.HeaderContentType))
			if err != nil {
				return fmt.Errorf("inertiaframe: failed to parse Content-Type header: %w", err)
			}

			// Inertia accepts only JSON or multipart/form-data.
			switch mediaType {
			case mediaTypeJSON:
				{
					d("received JSON request")

					if err := jsonv2.UnmarshalRead(r.Body, &msg, jsonUnmarshalOptions...); err != nil {
						return fmt.Errorf("inertiaframe: failed to decode request: %w", err)
					}
				}
			case mediaTypeForm, mediaTypeMultipart:
				{
					d("received form request")

					if err := r.ParseForm(); err != nil {
						return fmt.Errorf("inertiaframe: failed to parse form data: %w", err)
					}

					if err := formDecoder.Decode(&msg, r.Form); err != nil {
						return fmt.Errorf("inertiaframe: failed to decode form data: %w", err)
					}
				}
			}
		}

		if validator != nil {
			if err := validator.Validate(&msg); err != nil {
				d("failed to validate request")

				return fmt.Errorf("inertiaframe: failed to validate request: %w", err)
			}
		}

		resp, err := endpoint.Execute(ctx, newRequest(msg))
		if err != nil {
			return fmt.Errorf("inertiaframe: failed to execute: %w", err)
		}

		if resp == nil {
			d("received empty response")

			return ErrEmptyResponse
		}

		if writer, ok := resp.(RawResponseWriter); ok {
			if err := writer.Write(w, r); err != nil {
				return fmt.Errorf("inertiaframe: failed to write response: %w", err)
			}

			return nil
		}

		if optioner, ok := resp.(ResponseOptioner); ok {
			opts := optioner.Options()

			renderCtx.ClearHistory = opts.ClearHistory
			renderCtx.EncryptHistory = opts.EncryptHistory
			renderCtx.Concurrency = opts.Concurrency
		}

		var props []inertia.Prop
		if proper, ok := r.Context().Value(kCtxKey).(inertia.Proper); ok {
			d("has shared props")

			props = proper.Props()
		}

		proper := resp.Proper()
		if proper.Len() > 0 {
			d("response has props")

			props = append(props, proper.Props()...)
		}

		renderCtx.Props = props

		sess, err := sessionFromRequest(r)
		if err != nil {
			return fmt.Errorf("inertiaframe: failed to get session: %w", err)
		}

		errors := sess.ValidationErrors()

		if errors != nil {
			renderCtx.ErrorBag = sess.ErrorBag()
			renderCtx.AddValidationErrorer(inertia.ValidationErrors(errors))
		}

		componentName := resp.Component()
		debug.Assert(componentName != "", "component must not be empty, when using non RawResponseWriter")

		if err := inertia.Render(w, r, componentName, renderCtx); err != nil {
			return fmt.Errorf("inertiaframe: failed to render: %w", err)
		}

		return nil
	}))
}

// Struct creates a new response from a struct annotated with inertia tags.
//
// The struct must implement the Message interface.
func Struct(m Message, opts ...ResponseOption) (Response, error) {
	proper, err := inertia.ParseStruct(m)
	if err != nil {
		return nil, fmt.Errorf("inertiaframe: failed to parse props: %w", err)
	}

	var options ResponseOptions
	if len(opts) > 0 {
		for _, opt := range opts {
			opt(&options)
		}
	}

	return &resp{proper, m.Component(), options}, nil
}

// Proper creates a new response from an inertia.Proper.
func Proper(proper inertia.Proper, component string, opts ...ResponseOption) (Response, error) {
	var options ResponseOptions
	if len(opts) > 0 {
		for _, opt := range opts {
			opt(&options)
		}
	}

	return &resp{proper, component, options}, nil
}

// resp represents a response to an Inertia request.
//
// It is a helper that implements the Response interface and is used
// to create a response from a struct or a inertia.Proper.
type resp struct {
	proper    inertia.Proper
	component string
	options   ResponseOptions
}

func (r *resp) Component() string        { return r.component }
func (r *resp) Proper() inertia.Proper   { return r.proper }
func (r *resp) Options() ResponseOptions { return r.options }
