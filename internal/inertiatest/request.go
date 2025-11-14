package inertiatest

import (
	"cmp"
	"net/http"
	"net/http/httptest"
	"strings"

	"go.segfaultmedaddy.com/inertia/internal/inertiaheader"
)

type RequestConfig struct {
	Version          string
	PartialComponent string
	Whitelist        []string
	Blacklist        []string
	ResetProps       []string
	Inertia          bool
}

// NewRequest creates a new request with an empty body.
func NewRequest(
	method string,
	target string,
	config *RequestConfig,
) (*http.Request, *httptest.ResponseRecorder) {
	r := httptest.NewRequest(method, target, nil)

	//nolint:exhaustruct
	config = cmp.Or(config, &RequestConfig{})

	if config.Inertia {
		r.Header.Set(inertiaheader.HeaderXInertia, "true")
	}

	if config.Version != "" {
		r.Header.Set(inertiaheader.HeaderXInertiaVersion, config.Version)
	}

	if len(config.Whitelist) > 0 {
		r.Header.Set(inertiaheader.HeaderXInertiaPartialData, strings.Join(config.Whitelist, ","))
	}

	if len(config.Blacklist) > 0 {
		r.Header.Set(inertiaheader.HeaderXInertiaPartialExcept, strings.Join(config.Blacklist, ","))
	}

	if len(config.ResetProps) > 0 {
		r.Header.Set(inertiaheader.HeaderXInertiaReset, strings.Join(config.ResetProps, ","))
	}

	if config.PartialComponent != "" {
		r.Header.Set(inertiaheader.HeaderXInertiaPartialComponent, config.PartialComponent)
	}

	return r, httptest.NewRecorder()
}
