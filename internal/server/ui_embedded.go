//go:build !headless

package server

import (
	webui "csgclaw/web"
	"net/http"
)

// UIAvailable reports whether this binary includes the built-in Web UI.
const UIAvailable = true

func uiHandler() http.Handler { return webui.Handler() }
