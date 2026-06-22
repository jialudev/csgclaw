package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"csgclaw/internal/csghubauth"
)

const csghubAuthCallbackPath = "/api/v1/csghub/auth/callback"

type csghubAuthLoginRequest struct {
	ReturnURL   string `json:"return_url,omitempty"`
	CallbackURL string `json:"-"`
}

var csghubAuthStatus = func(r *http.Request) (csghubauth.Status, error) {
	return csghubauth.Default().Status(r.Context())
}

var csghubAuthLogin = func(r *http.Request, req csghubAuthLoginRequest) (csghubauth.LoginResponse, error) {
	return csghubauth.Default().Login(r.Context(), csghubauth.LoginOptions{
		ReturnURL:   req.ReturnURL,
		CallbackURL: req.CallbackURL,
	})
}

var csghubAuthLogout = func(r *http.Request) (csghubauth.Status, error) {
	return csghubauth.Default().Logout(r.Context())
}

var csghubAuthCallback = func(r *http.Request) (string, error) {
	values := r.URL.Query()
	if values.Get("jwt_token") == "" {
		if token := bearerToken(r.Header.Get("Authorization")); token != "" {
			values = cloneURLValues(values)
			values.Set("jwt_token", token)
		}
	}
	return csghubauth.Default().CompleteCallback(r.Context(), values)
}

func (h *Handler) handleCSGHubAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := csghubAuthStatus(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) handleCSGHubAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	redirectURL, err := csghubAuthCallback(r)
	if err != nil {
		status := http.StatusBadRequest
		if !csghubauth.IsCallbackValidationError(err) {
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

func (h *Handler) handleCSGHubAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req csghubAuthLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, fmt.Sprintf("decode request: %v", err), http.StatusBadRequest)
		return
	}
	if req.ReturnURL == "" {
		req.ReturnURL = r.Referer()
	}
	if req.CallbackURL == "" {
		req.CallbackURL = csghubAuthLocalCallbackURL(r)
	}
	resp, err := csghubAuthLogin(r, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleCSGHubAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := csghubAuthLogout(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func csghubAuthLocalCallbackURL(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Host)
	if host == "" && r.URL != nil {
		host = strings.TrimSpace(r.URL.Host)
	}
	if !isLocalRequestHost(host) {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	u := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   csghubAuthCallbackPath,
	}
	return u.String()
}

func isLocalRequestHost(hostport string) bool {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return false
	}
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	switch strings.ToLower(strings.Trim(host, "[]")) {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
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
