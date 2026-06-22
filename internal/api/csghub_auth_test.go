package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"csgclaw/internal/csghubauth"
)

func TestHandleCSGHubAuthStatus(t *testing.T) {
	loggedInAt := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	restore := stubCSGHubAuthStatus(func(*http.Request) (csghubauth.Status, error) {
		return csghubauth.Status{
			Authenticated: true,
			UserID:        "alice",
			UserUUID:      "user-1",
			Avatar:        "https://example.test/avatar.png",
			CSGHubBaseURL: "https://hub.example.test",
			PortalURL:     "https://hub.example.test/portal",
			LoggedInAt:    &loggedInAt,
		}, nil
	})
	defer restore()

	rec := httptest.NewRecorder()
	(&Handler{}).Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/csghub/auth/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got csghubauth.Status
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.Authenticated || got.UserID != "alice" || got.CSGHubBaseURL != "https://hub.example.test" {
		t.Fatalf("response = %+v, want authenticated alice", got)
	}
}

func TestHandleCSGHubAuthLogin(t *testing.T) {
	var gotReturnURL string
	var gotCallbackURL string
	restore := stubCSGHubAuthLogin(func(_ *http.Request, req csghubAuthLoginRequest) (csghubauth.LoginResponse, error) {
		gotReturnURL = req.ReturnURL
		gotCallbackURL = req.CallbackURL
		return csghubauth.LoginResponse{LoginURL: "https://iam.example.test/login"}, nil
	})
	defer restore()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/csghub/auth/login", strings.NewReader(`{"return_url":"http://127.0.0.1:18080/#/dms/room-1"}`))
	req.Host = "127.0.0.1:18080"
	(&Handler{}).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got csghubauth.LoginResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.LoginURL != "https://iam.example.test/login" {
		t.Fatalf("LoginURL = %q", got.LoginURL)
	}
	if gotReturnURL != "http://127.0.0.1:18080/#/dms/room-1" {
		t.Fatalf("return_url = %q", gotReturnURL)
	}
	if gotCallbackURL != "http://127.0.0.1:18080/api/v1/csghub/auth/callback" {
		t.Fatalf("callback_url = %q", gotCallbackURL)
	}
}

func TestHandleCSGHubAuthCallback(t *testing.T) {
	var gotToken string
	restore := stubCSGHubAuthCallback(func(r *http.Request) (string, error) {
		gotToken = r.URL.Query().Get("jwt_token")
		return "http://127.0.0.1:18080/#/workspace", nil
	})
	defer restore()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/csghub/auth/callback?jwt_token=jwt-value", nil)
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

func TestHandleCSGHubAuthLogout(t *testing.T) {
	restore := stubCSGHubAuthLogout(func(*http.Request) (csghubauth.Status, error) {
		return csghubauth.Status{}, nil
	})
	defer restore()

	rec := httptest.NewRecorder()
	(&Handler{}).Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/csghub/auth/logout", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got csghubauth.Status
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Authenticated {
		t.Fatalf("response = %+v, want unauthenticated", got)
	}
}

func stubCSGHubAuthStatus(fn func(*http.Request) (csghubauth.Status, error)) func() {
	previous := csghubAuthStatus
	csghubAuthStatus = fn
	return func() { csghubAuthStatus = previous }
}

func stubCSGHubAuthLogin(fn func(*http.Request, csghubAuthLoginRequest) (csghubauth.LoginResponse, error)) func() {
	previous := csghubAuthLogin
	csghubAuthLogin = fn
	return func() { csghubAuthLogin = previous }
}

func stubCSGHubAuthCallback(fn func(*http.Request) (string, error)) func() {
	previous := csghubAuthCallback
	csghubAuthCallback = fn
	return func() { csghubAuthCallback = previous }
}

func stubCSGHubAuthLogout(fn func(*http.Request) (csghubauth.Status, error)) func() {
	previous := csghubAuthLogout
	csghubAuthLogout = fn
	return func() { csghubAuthLogout = previous }
}
