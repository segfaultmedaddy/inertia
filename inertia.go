// Package inertia implements the protocol for communication with
// the Inertia.js client-side framework.
//
// The package implements all parts of the protocol and provides an idiomatic
// Go API on top of the standard "net/http" and "html/template" packages.
// It exposes a Renderer rendering the response compliant with the Inertia.js protocol
// and a Middleware for handling Inertia.js requests.
//
// For detailed protocol documentation, visit https://inertiajs.com/the-protocol
package inertia

import "go.inout.gg/foundations/debug"

//nolint:gochecknoglobals
var d = debug.Debuglog("inertia")
