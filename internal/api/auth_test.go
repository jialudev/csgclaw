package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"csgclaw/internal/auth"
	"csgclaw/internal/config"
)

func TestHandleAuthStatus(t *testing.T) {
	loggedInAt := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	restore := stubAuthStatus(func(*http.Request) (auth.Status, error) {
		return auth.Status{
			Authenticated: true,
			UserID:        "alice",
			UserUUID:      "user-1",
			Avatar:        "https://example.test/avatar.png",
			BaseURL:       "https://hub.example.test",
			PortalURL:     "https://hub.example.test/portal",
			LoggedInAt:    &loggedInAt,
		}, nil
	})
	defer restore()

	rec := httptest.NewRecorder()
	(&Handler{}).Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got auth.Status
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.Authenticated || got.UserID != "alice" || got.BaseURL != "https://hub.example.test" {
		t.Fatalf("response = %+v, want authenticated alice", got)
	}
}

func TestHandleAuthLogin(t *testing.T) {
	var gotReturnURL string
	var gotCallbackURL string
	restore := stubAuthLogin(func(_ *http.Request, req authLoginRequest) (auth.LoginResponse, error) {
		gotReturnURL = req.ReturnURL
		gotCallbackURL = req.CallbackURL
		return auth.LoginResponse{LoginURL: "https://iam.example.test/login"}, nil
	})
	defer restore()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"return_url":"http://127.0.0.1:18080/#/dms/room-1"}`))
	req.Host = "127.0.0.1:18080"
	(&Handler{}).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got auth.LoginResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.LoginURL != "https://iam.example.test/login" {
		t.Fatalf("LoginURL = %q", got.LoginURL)
	}
	if gotReturnURL != "http://127.0.0.1:18080/#/dms/room-1" {
		t.Fatalf("return_url = %q", gotReturnURL)
	}
	if gotCallbackURL != "http://127.0.0.1:18080/api/v1/auth/callback" {
		t.Fatalf("callback_url = %q", gotCallbackURL)
	}
}

func TestHandleAuthLoginUsesReturnURLOrigin(t *testing.T) {
	var gotCallbackURL string
	restore := stubAuthLogin(func(_ *http.Request, req authLoginRequest) (auth.LoginResponse, error) {
		gotCallbackURL = req.CallbackURL
		return auth.LoginResponse{LoginURL: "https://iam.example.test/login"}, nil
	})
	defer restore()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"return_url":"https://current.example.test/#/workspace"}`))
	req.Host = "fallback.example.test"
	(&Handler{}).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotCallbackURL != "https://current.example.test/api/v1/auth/callback" {
		t.Fatalf("callback_url = %q", gotCallbackURL)
	}
}

func TestHandleAuthLoginUsesAdvertiseBaseURL(t *testing.T) {
	var gotCallbackURL string
	restore := stubAuthLogin(func(_ *http.Request, req authLoginRequest) (auth.LoginResponse, error) {
		gotCallbackURL = req.CallbackURL
		return auth.LoginResponse{LoginURL: "https://iam.example.test/login"}, nil
	})
	defer restore()

	srv := &Handler{}
	srv.SetServerConfig(config.ServerConfig{
		AdvertiseBaseURL: "https://csgclaw.example.test/base/",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"return_url":"https://csgclaw.example.test/base/#/workspace"}`))
	req.Host = "evil.example.test"
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotCallbackURL != "https://csgclaw.example.test/base/api/v1/auth/callback" {
		t.Fatalf("callback_url = %q", gotCallbackURL)
	}
}

func TestAuthCallbackURLUsesRequestHostWhenUnconfigured(t *testing.T) {
	srv := &Handler{}
	srv.SetServerConfig(config.ServerConfig{ListenAddr: "0.0.0.0:18080"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.Host = "evil.example.test"
	if got := srv.authCallbackURL(req); got != "http://evil.example.test/api/v1/auth/callback" {
		t.Fatalf("authCallbackURL() = %q", got)
	}
}

func TestHandleAuthCallback(t *testing.T) {
	var gotToken string
	restore := stubAuthCallback(func(r *http.Request, opts auth.CallbackOptions) (string, error) {
		gotToken = r.URL.Query().Get("jwt_token")
		if opts.AllowedReturnURLBase != "http://127.0.0.1:18080/api/v1/auth/callback" {
			t.Fatalf("AllowedReturnURLBase = %q", opts.AllowedReturnURLBase)
		}
		return "http://127.0.0.1:18080/#/workspace", nil
	})
	defer restore()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback?jwt_token=jwt-value", nil)
	req.Host = "127.0.0.1:18080"
	(&Handler{}).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusFound, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "http://127.0.0.1:18080/#/workspace" {
		t.Fatalf("Location = %q", got)
	}
	if gotToken != "jwt-value" {
		t.Fatalf("jwt_token = %q", gotToken)
	}
}

func TestHandleAuthLogout(t *testing.T) {
	restore := stubAuthLogout(func(*http.Request) (auth.Status, error) {
		return auth.Status{}, nil
	})
	defer restore()

	rec := httptest.NewRecorder()
	(&Handler{}).Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got auth.Status
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Authenticated {
		t.Fatalf("response = %+v, want unauthenticated", got)
	}
}

func stubAuthStatus(fn func(*http.Request) (auth.Status, error)) func() {
	previous := appAuthStatus
	appAuthStatus = fn
	return func() { appAuthStatus = previous }
}

func stubAuthLogin(fn func(*http.Request, authLoginRequest) (auth.LoginResponse, error)) func() {
	previous := appAuthLogin
	appAuthLogin = fn
	return func() { appAuthLogin = previous }
}

func stubAuthCallback(fn func(*http.Request, auth.CallbackOptions) (string, error)) func() {
	previous := appAuthCallback
	appAuthCallback = fn
	return func() { appAuthCallback = previous }
}

func stubAuthLogout(fn func(*http.Request) (auth.Status, error)) func() {
	previous := appAuthLogout
	appAuthLogout = fn
	return func() { appAuthLogout = previous }
}
