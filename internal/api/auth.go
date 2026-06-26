package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"csgclaw/internal/auth"
	"csgclaw/internal/config"
)

const authCallbackPath = "/api/v1/auth/callback"

type authLoginRequest struct {
	ReturnURL   string `json:"return_url,omitempty"`
	CallbackURL string `json:"-"`
}

var appAuthStatus = func(r *http.Request) (auth.Status, error) {
	return auth.Default().Status(r.Context())
}

var appAuthLogin = func(r *http.Request, req authLoginRequest) (auth.LoginResponse, error) {
	return auth.Default().Login(r.Context(), auth.LoginOptions{
		ReturnURL:   req.ReturnURL,
		CallbackURL: req.CallbackURL,
	})
}

var appAuthLogout = func(r *http.Request) (auth.Status, error) {
	return auth.Default().Logout(r.Context())
}

var appAuthCallback = func(r *http.Request, opts auth.CallbackOptions) (string, error) {
	values := r.URL.Query()
	if values.Get("jwt_token") == "" {
		if token := bearerToken(r.Header.Get("Authorization")); token != "" {
			values = cloneURLValues(values)
			values.Set("jwt_token", token)
		}
	}
	return auth.Default().CompleteCallback(r.Context(), values, opts)
}

func (h *Handler) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := appAuthStatus(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	redirectURL, err := appAuthCallback(r, auth.CallbackOptions{
		AllowedReturnURLBase: h.authCallbackURL(r),
	})
	if err != nil {
		status := http.StatusBadRequest
		if !auth.IsCallbackValidationError(err) {
			status = http.StatusBadGateway
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Location", redirectURL)
	w.WriteHeader(http.StatusFound)
}

func (h *Handler) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req authLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, fmt.Sprintf("decode request: %v", err), http.StatusBadRequest)
		return
	}
	if req.ReturnURL == "" {
		req.ReturnURL = r.Referer()
	}
	if req.CallbackURL == "" {
		req.CallbackURL = h.authCallbackURL(r, req.ReturnURL)
	}
	resp, err := appAuthLogin(r, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := appAuthLogout(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) authCallbackURL(r *http.Request, pageURLs ...string) string {
	if h != nil && strings.TrimSpace(h.server.AdvertiseBaseURL) != "" {
		return authCallbackURLFromBase(config.ResolveAdvertiseBaseURL(h.server))
	}
	for _, pageURL := range pageURLs {
		if callbackURL := authCallbackURLFromPageURL(pageURL); callbackURL != "" {
			return callbackURL
		}
	}
	if callbackURL := authRequestCallbackURL(r); callbackURL != "" {
		return callbackURL
	}
	if h != nil && (strings.TrimSpace(h.server.ListenAddr) != "" || strings.TrimSpace(h.server.AdvertiseBaseURL) != "") {
		return authCallbackURLFromBase(config.ResolveAdvertiseBaseURL(h.server))
	}
	return ""
}

func authRequestCallbackURL(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Host)
	if host == "" && r.URL != nil {
		host = strings.TrimSpace(r.URL.Host)
	}
	if host == "" {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	u := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   authCallbackPath,
	}
	return u.String()
}

func authCallbackURLFromPageURL(pageURL string) string {
	u, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	u.RawQuery = ""
	u.Fragment = ""
	return authCallbackURLFromBase(u.String())
}

func authCallbackURLFromBase(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	u.Path = strings.TrimRight(u.Path, "/") + authCallbackPath
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func bearerToken(authHeader string) string {
	const prefix = "bearer "
	authHeader = strings.TrimSpace(authHeader)
	if len(authHeader) <= len(prefix) || strings.ToLower(authHeader[:len(prefix)]) != prefix {
		return ""
	}
	return strings.TrimSpace(authHeader[len(prefix):])
}

func cloneURLValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, list := range values {
		cloned[key] = append([]string(nil), list...)
	}
	return cloned
}
