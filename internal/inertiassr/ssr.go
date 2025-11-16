package inertiassr

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/go-json-experiment/json"
	"go.inout.gg/foundations/debug"

	"go.segfaultmedaddy.com/inertia/internal/inertiabase"
	"go.segfaultmedaddy.com/inertia/internal/inertiaheader"
)

var _ SSRClient = (*ssr)(nil)

type SSRTemplateData struct {
	Head string `json:"head"`
	Body string `json:"body"`
}

//go:generate mockgen -destination ssr_mock.go -package inertiassr . SSRClient
type SSRClient interface {
	// Render makes a request to the server-side rendering service with the given page data.
	Render(context.Context, *inertiabase.Page) (*SSRTemplateData, error)
}

// ssr is an HTTP client that makes requests to a server-side rendering service.
type ssr struct {
	client *http.Client
	url    string
}

func NewHTTPSsrClient(url string, client *http.Client) SSRClient {
	debug.Assert(url != "", "url must be provided")
	debug.Assert(client != nil, "client must be provided")

	return &ssr{client, url}
}

func (s *ssr) Render(ctx context.Context, p *inertiabase.Page) (*SSRTemplateData, error) {
	debug.Assert(p != nil, "page must be set")

	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("inertia: failed to marshal page: %w", err)
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("inertia: failed to create HTTP request: %w", err)
	}

	r.Header.Set(inertiaheader.HeaderContentType, inertiaheader.ContentTypeJSON)

	resp, err := s.client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("inertia: failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("inertia: unexpected HTTP status code: %d", resp.StatusCode)
	}

	var data SSRTemplateData
	if err := json.UnmarshalRead(resp.Body, &data); err != nil {
		return nil, fmt.Errorf("inertia: failed to decode JSON response: %w", err)
	}

	return &data, nil
}
