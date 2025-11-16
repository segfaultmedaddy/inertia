package inertia

import (
	"net/http"

	"go.segfaultmedaddy.com/inertia/internal/inertiassr"
)

type (
	// SSRClient communicates with a server-side rendering service to pre-render Inertia pages.
	SSRClient = inertiassr.SSRClient

	// SsrTemplateData contains the HTML head and body sections returned by SSR rendering.
	SsrTemplateData = inertiassr.SSRTemplateData
)

// NewHTTPSsrClient creates an HTTP-based SSR client that sends render requests to the specified URL.
// If client is nil, http.DefaultClient is used.
func NewHTTPSsrClient(url string, client *http.Client) SSRClient {
	if client == nil {
		client = http.DefaultClient
	}

	return inertiassr.NewHTTPSsrClient(url, client)
}
