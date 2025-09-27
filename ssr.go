package inertia

import (
	"net/http"

	"go.inout.gg/inertia/internal/inertiassr"
)

type (
	// SsrClient is a client that makes requests to a server-side rendering service.
	SsrClient       = inertiassr.SsrClient
	SsrTemplateData = inertiassr.SsrTemplateData
)

// NewHTTPSsrClient creates a new SsrClient that makes requests to the given HTTP client.
// If client is nil, http.DefaultClient is used.
func NewHTTPSsrClient(url string, client *http.Client) SsrClient {
	return inertiassr.NewHTTPSsrClient(url, client)
}
