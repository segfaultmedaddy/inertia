package inertiaheader

const (
	HeaderXInertia                 = "X-Inertia"                   // client/server
	HeaderXInertiaVersion          = "X-Inertia-Version"           // client
	HeaderXInertiaLocation         = "X-Inertia-Location"          // client/server, redirect URL
	HeaderXInertiaPartialData      = "X-Inertia-Partial-Data"      // client, whitelist
	HeaderXInertiaPartialExcept    = "X-Inertia-Partial-Except"    // client, blacklist
	HeaderXInertiaPartialComponent = "X-Inertia-Partial-Component" // client
	HeaderXInertiaReset            = "X-Inertia-Reset"             // client, force reload
	HeaderXInertiaErrorBag         = "X-Inertia-Error-Bag"         // client

	HeaderVary        = "Vary"
	HeaderContentType = "Content-Type"
	HeaderReferer     = "Referer"
)

const (
	ContentTypeHTML = "text/html"
	ContentTypeJSON = "application/json"
)
