package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func pathValue(r *http.Request, key string) string {
	if r == nil {
		return ""
	}
	if value := strings.TrimSpace(chi.URLParam(r, key)); value != "" {
		return value
	}
	return strings.TrimSpace(r.PathValue(key))
}
