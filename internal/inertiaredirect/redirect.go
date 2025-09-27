package inertiaredirect

import (
	"net/http"

	"go.inout.gg/foundations/debug"
)

//nolint:gochecknoglobals
var d = debug.Debuglog("inertia/redirect")

// Redirect redirects the client to the specified URL.
//
// It follows the redirect specification described here: https://inertiajs.com/redirects
func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	// Redirect GET requests with a 302
	statusCode := http.StatusSeeOther
	if r.Method == http.MethodGet {
		statusCode = http.StatusFound
	}

	d("Redirecting to %s with status code %d", url, statusCode)

	http.Redirect(w, r, url, statusCode)
}
