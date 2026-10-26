//go:build headless

package server

import "net/http"

// UIAvailable reports whether this binary includes the built-in Web UI.
const UIAvailable = false

func uiHandler() http.Handler { return http.NotFoundHandler() }
