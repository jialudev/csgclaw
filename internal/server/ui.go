package server

import (
	"net/http"

	webui "csgclaw/web"
)

func uiHandler() http.Handler {
	return webui.Handler()
}
