package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type pairedUI struct {
	origin, authorization string
	expires               time.Time
}
type externalUI struct {
	origins  map[string]bool
	mu       sync.Mutex
	sessions map[string]pairedUI
}

func ValidateUIOrigins(origins []string) error {
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
			return fmt.Errorf("UI origin must be an exact HTTP(S) origin without a path: %q", origin)
		}
	}
	return nil
}
func (h *Handler) SetUIOrigins(origins []string) error {
	if err := ValidateUIOrigins(origins); err != nil {
		return err
	}
	ui := &externalUI{origins: map[string]bool{}, sessions: map[string]pairedUI{}}
	for _, origin := range origins {
		ui.origins[origin] = true
	}
	h.externalUI = ui
	return nil
}
func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
func (h *Handler) externalUIRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.externalUI == nil || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		cross := origin != "" && origin != requestOrigin(r)
		if cross && len(h.externalUI.origins) > 0 {
			if !h.externalUI.origins[origin] {
				controlError(w, 403, "origin_denied", "UI origin is not allowed")
				return
			}
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Expose-Headers", "ETag, Content-Disposition, Content-Length")
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, If-Match, X-Content-SHA256, Last-Event-ID")
				w.WriteHeader(204)
				return
			}
		}
		if cookie, err := r.Cookie("csgclaw_ui_session"); err == nil {
			h.externalUI.mu.Lock()
			session, ok := h.externalUI.sessions[cookie.Value]
			if ok && time.Now().After(session.expires) {
				delete(h.externalUI.sessions, cookie.Value)
				ok = false
			}
			h.externalUI.mu.Unlock()
			effectiveOrigin := origin
			if effectiveOrigin == "" {
				effectiveOrigin = requestOrigin(r)
			}
			if ok && session.origin == effectiveOrigin && h.validateServerAccessToken(session.authorization) {
				clone := r.Clone(r.Context())
				clone.Header = r.Header.Clone()
				clone.Header.Set("Authorization", session.authorization)
				r = clone
			}
		}
		if cross && len(h.externalUI.origins) > 0 && !h.validateServerAccessToken(r.Header.Get("Authorization")) {
			controlError(w, 401, "unauthorized", "pair this UI or provide a service credential")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (h *Handler) pairExternalUI(w http.ResponseWriter, r *http.Request) {
	if h.externalUI == nil || !h.validateServerAccessToken(r.Header.Get("Authorization")) || r.Header.Get("Authorization") == "" {
		controlError(w, 401, "unauthorized", "pairing requires a service credential")
		return
	}
	origin := r.Header.Get("Origin")
	if origin == "" || (origin != requestOrigin(r) && !h.externalUI.origins[origin]) {
		controlError(w, 403, "origin_denied", "pairing requires an allowed browser origin")
		return
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		controlError(w, 503, "pairing_unavailable", "could not create a session")
		return
	}
	id := base64.RawURLEncoding.EncodeToString(b)
	expires := time.Now().Add(12 * time.Hour)
	h.externalUI.mu.Lock()
	for key, session := range h.externalUI.sessions {
		if time.Now().After(session.expires) {
			delete(h.externalUI.sessions, key)
		}
	}
	if len(h.externalUI.sessions) >= 128 {
		h.externalUI.mu.Unlock()
		controlError(w, 429, "pairing_limit", "too many paired UIs")
		return
	}
	h.externalUI.sessions[id] = pairedUI{origin: origin, authorization: r.Header.Get("Authorization"), expires: expires}
	h.externalUI.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "csgclaw_ui_session", Value: id, Path: "/api/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: 12 * 60 * 60})
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"expires_at": expires})
}
func (h *Handler) unpairExternalUI(w http.ResponseWriter, r *http.Request) {
	if h.externalUI == nil {
		controlError(w, 503, "pairing_unavailable", "pairing is unavailable")
		return
	}
	origin := r.Header.Get("Origin")
	if origin == "" || (origin != requestOrigin(r) && !h.externalUI.origins[origin]) {
		controlError(w, 403, "origin_denied", "logout requires an allowed origin")
		return
	}
	if cookie, err := r.Cookie("csgclaw_ui_session"); err == nil {
		h.externalUI.mu.Lock()
		delete(h.externalUI.sessions, cookie.Value)
		h.externalUI.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "csgclaw_ui_session", Value: "", Path: "/api/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(204)
}
