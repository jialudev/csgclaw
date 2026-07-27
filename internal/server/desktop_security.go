package server

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type DesktopOptions struct {
	BaseURL           string
	SessionToken      string
	ServerAccessToken string
}

func desktopSecurityHandler(next http.Handler, listenerAddr net.Addr, opts DesktopOptions) (http.Handler, error) {
	if next == nil {
		return nil, fmt.Errorf("desktop handler is required")
	}
	token := strings.TrimSpace(opts.SessionToken)
	if len(token) < 43 {
		return nil, fmt.Errorf("desktop session token must contain at least 256 bits")
	}

	baseURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/"))
	if err != nil || baseURL.Scheme != "http" || baseURL.User != nil || baseURL.Path != "" || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, fmt.Errorf("invalid desktop base URL")
	}
	if baseURL.Hostname() != "127.0.0.1" {
		return nil, fmt.Errorf("desktop base URL must use 127.0.0.1")
	}
	if listenerAddr == nil || baseURL.Host != listenerAddr.String() {
		return nil, fmt.Errorf("desktop base URL does not match listener")
	}

	expectedOrigin := baseURL.Scheme + "://" + baseURL.Host
	expectedSessionAuthorization := "Bearer " + token
	expectedServerAuthorization := ""
	if serverToken := strings.TrimSpace(opts.ServerAccessToken); serverToken != "" {
		expectedServerAuthorization = "Bearer " + serverToken
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setDesktopSecurityHeaders(w.Header())
		if r.Host != baseURL.Host {
			http.Error(w, "invalid host", http.StatusBadRequest)
			return
		}
		if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && origin != expectedOrigin {
			http.Error(w, "invalid origin", http.StatusForbidden)
			return
		}
		if desktopAuthExemptPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		got := r.Header.Get("Authorization")
		sessionAuthorized := subtle.ConstantTimeCompare([]byte(got), []byte(expectedSessionAuthorization)) == 1
		serverAuthorized := expectedServerAuthorization != "" &&
			subtle.ConstantTimeCompare([]byte(got), []byte(expectedServerAuthorization)) == 1
		if !sessionAuthorized && !serverAuthorized {
			w.Header().Set("WWW-Authenticate", `Bearer realm="csgclaw-desktop"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}), nil
}

func desktopAuthExemptPath(path string) bool {
	switch path {
	case "/healthz", "/api/v1/auth/callback", "/api/v1/connectors/github/oauth/callback":
		return true
	default:
		return false
	}
}

func setDesktopSecurityHeaders(header http.Header) {
	header.Set("Content-Security-Policy", strings.Join([]string{
		"default-src 'self'",
		"base-uri 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob: https:",
		"font-src 'self' data:",
		"connect-src 'self' https: wss:",
		"worker-src 'self' blob:",
		"frame-src 'none'",
		"form-action 'self' https:",
	}, "; "))
	header.Set("Cross-Origin-Opener-Policy", "same-origin")
	header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), display-capture=(), usb=(), serial=(), hid=()")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
}
