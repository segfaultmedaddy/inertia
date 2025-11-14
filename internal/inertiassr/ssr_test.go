package inertiassr

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.segfaultmedaddy.com/inertia/internal/inertiabase"
)

var defaultClient = http.DefaultClient

func TestSsrRender(t *testing.T) {
	t.Parallel()

	page := &inertiabase.Page{
		Component: "Test",
		Props:     map[string]any{"foo": "bar"},
	}
	pageJSON, err := json.Marshal(page)
	require.NoError(t, err)

	t.Run("successfully renders page", func(t *testing.T) {
		t.Parallel()

		expected := &SsrTemplateData{
			Head: "<head>Test</head>",
			Body: "<body>Content</body>",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			body := r.Body
			defer body.Close()

			buf, err := io.ReadAll(body)
			assert.NoError(t, err)

			// Assert that the request body matches the expected JSON
			assert.JSONEq(t, string(buf), string(pageJSON))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(expected))
		}))
		defer server.Close()

		client := NewHTTPSsrClient(server.URL, defaultClient)
		result, err := client.Render(t.Context(), page)

		require.NoError(t, err)
		assert.Equal(t, expected.Head, result.Head)
		assert.Equal(t, expected.Body, result.Body)
	})

	t.Run("handles server error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewHTTPSsrClient(server.URL, defaultClient)
		_, err := client.Render(t.Context(), page)
		assert.Error(t, err)
	})

	t.Run("handles invalid JSON response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte("invalid json"))
			assert.NoError(t, err)
		}))
		defer server.Close()

		client := NewHTTPSsrClient(server.URL, defaultClient)
		_, err := client.Render(t.Context(), page)
		assert.Error(t, err)
	})

	t.Run("handles invalid URL", func(t *testing.T) {
		t.Parallel()

		client := NewHTTPSsrClient("invalid-url", defaultClient)
		_, err := client.Render(t.Context(), page)
		assert.Error(t, err)
	})
}
