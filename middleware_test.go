package inertia

import (
	"html/template"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.segfaultmedaddy.com/inertia/internal/inertiaheader"
	"go.segfaultmedaddy.com/inertia/internal/inertiatest"
)

//nolint:gochecknoglobals
var tpl = template.Must(template.New("<inertia-test>").Parse(`<!doctype html>
<html>
<head>{{ .InertiaHead }}</head>
<body>{{ .InertiaBody }}</body>
</html>
`))

func newMiddleware(h http.Handler, renderer *Renderer, opts ...func(*MiddlewareConfig)) http.Handler {
	if renderer == nil {
		renderer = New(tpl, nil)
	}

	mux := http.NewServeMux()
	middleware := NewMiddleware(renderer, opts...)(mux)

	mux.HandleFunc("/inertia", h.ServeHTTP)

	return middleware
}

func TestMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("sets Vary header to X-Inertia", func(t *testing.T) {
		t.Parallel()

		// arrange
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		r, w := inertiatest.NewRequest(http.MethodGet, "/inertia", &inertiatest.RequestConfig{
			Inertia: true,
			Version: "",
		})

		// act
		middleware := newMiddleware(handler, New(tpl, &Config{Version: ""}))
		middleware.ServeHTTP(w, r)

		// assert
		assert.Equal(t, inertiaheader.HeaderXInertia,
			w.Header().Get(inertiaheader.HeaderVary))
	})

	t.Run("non-inertia request passes through unchanged", func(t *testing.T) {
		t.Parallel()

		// arrange
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("hello"))
		})

		r, w := inertiatest.NewRequest(http.MethodGet, "/inertia", nil)

		// act
		middleware := newMiddleware(handler, nil)
		middleware.ServeHTTP(w, r)

		// assert
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "hello", w.Body.String())
	})

	t.Run("version mismatch triggers handler", func(t *testing.T) {
		t.Parallel()

		// arrange
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		renderer := New(tpl, &Config{Version: "2.0.0"})
		r, w := inertiatest.NewRequest(http.MethodGet, "/inertia", &inertiatest.RequestConfig{
			Inertia: true,
			Version: "1.0.0",
		})

		// act
		middleware := newMiddleware(handler, renderer)
		middleware.ServeHTTP(w, r)

		// assert
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.NotEmpty(t, w.Header().Get(inertiaheader.HeaderXInertiaLocation))
	})

	t.Run("version mismatch triggers custom handler", func(t *testing.T) {
		t.Parallel()

		// arrange
		called := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		renderer := New(tpl, &Config{Version: "2.0.0"})
		r, w := inertiatest.NewRequest(http.MethodGet, "/inertia", &inertiatest.RequestConfig{
			Inertia: true,
			Version: "1.0.0",
		})

		// act
		middleware := newMiddleware(handler, renderer, func(c *MiddlewareConfig) {
			c.VersionMismatchHandler = func(w http.ResponseWriter, _ *http.Request) {
				called = true

				w.WriteHeader(http.StatusConflict)
			}
		})
		middleware.ServeHTTP(w, r)

		// assert
		assert.True(t, called)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("empty response triggers handler", func(t *testing.T) {
		t.Parallel()

		// arrange
		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			// intentionally empty - no writes to response
		})

		r, w := inertiatest.NewRequest(http.MethodGet, "/inertia", &inertiatest.RequestConfig{
			Inertia: true,
			Version: "",
		})

		// act
		middleware := newMiddleware(handler, New(tpl, &Config{Version: ""}))
		middleware.ServeHTTP(w, r)

		// assert
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("empty response triggers custom handler", func(t *testing.T) {
		t.Parallel()

		// arrange
		called := false
		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			// intentionally empty
		})

		r, w := inertiatest.NewRequest(http.MethodGet, "/inertia", &inertiatest.RequestConfig{
			Inertia: true,
			Version: "",
		})

		// act
		middleware := newMiddleware(handler, New(tpl, &Config{Version: ""}), func(c *MiddlewareConfig) {
			c.EmptyResponseHandler = func(w http.ResponseWriter, _ *http.Request) {
				called = true

				w.WriteHeader(http.StatusTeapot)
			}
		})
		middleware.ServeHTTP(w, r)

		// assert
		assert.True(t, called)
	})

	t.Run("stores renderer in context for Render", func(t *testing.T) {
		t.Parallel()

		// arrange
		var renderErr error

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			renderErr = Render(w, r, "TestComponent", RenderContext{})
		})

		r, w := inertiatest.NewRequest(http.MethodGet, "/inertia", &inertiatest.RequestConfig{
			Inertia: true,
			Version: "",
		})

		// act
		middleware := newMiddleware(handler, New(tpl, &Config{Version: ""}))
		middleware.ServeHTTP(w, r)

		// assert
		assert.NoError(t, renderErr)
	})

	t.Run("redirects PUT/PATCH/DELETE with 303", func(t *testing.T) {
		t.Parallel()

		redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/somewhere", http.StatusFound)
		})

		testCases := []struct {
			name           string
			method         string
			expectedStatus int
		}{
			{"PATCH should redirect with 303", http.MethodPatch, http.StatusSeeOther},
			{"PUT should redirect with 303", http.MethodPut, http.StatusSeeOther},
			{"DELETE should redirect with 303", http.MethodDelete, http.StatusSeeOther},
			{"GET should redirect with 302", http.MethodGet, http.StatusFound},
			{"POST should redirect with 302", http.MethodPost, http.StatusFound},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				// arrange
				r, w := inertiatest.NewRequest(tc.method, "/inertia", &inertiatest.RequestConfig{
					Inertia: true,
				})

				// act
				middleware := newMiddleware(redirectHandler, nil)
				middleware.ServeHTTP(w, r)

				// assert
				assert.Equal(t, tc.expectedStatus, w.Code)
				assert.Equal(t, "/somewhere", w.Header().Get("Location"))
			})
		}
	})
}
