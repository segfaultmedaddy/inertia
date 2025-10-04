package inertia

import (
	"net/http"

	"go.inout.gg/inertia/internal/inertiassr"
)

type (
	// SsrClient communicates with a server-side rendering service to pre-render Inertia pages.
	SsrClient = inertiassr.SsrClient

	// SsrTemplateData contains the HTML head and body sections returned by SSR rendering.
	SsrTemplateData = inertiassr.SsrTemplateData
)

// NewHTTPSsrClient creates an HTTP-based SSR client that sends render requests to the specified URL.
// If client is nil, http.DefaultClient is used.
func NewHTTPSsrClient(url string, client *http.Client) SsrClient {
	return inertiassr.NewHTTPSsrClient(url, client)
}
