//go:build !headless && ui_custom

package serve

import (
	"net/http"

	"csgclaw/web/custom"
)

const embeddedUIAvailable = true

func bundledUIHandler() (http.Handler, error) { return custom.Handler() }
