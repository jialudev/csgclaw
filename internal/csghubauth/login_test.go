package csghubauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

func TestCompleteCallbackStoresCredentials(t *testing.T) {
	var sawTokenAuth bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/user/alice/tokens":
			if got := r.URL.Query().Get("app"); got != "git" {
				t.Fatalf("app query = %q, want git", got)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer "+testJWT("alice", "user-1") {
				t.Fatalf("Authorization = %q", got)
			}
			sawTokenAuth = true
			writeJSON(t, w, map[string]any{
				"msg": "OK",
				"data": []map[string]any{{
					"token": "access-token",
				}},
			})
		case "/api/v1/user/alice":
			writeJSON(t, w, map[string]any{
				"msg": "OK",
				"data": map[string]any{
					"avatar": "https://example.test/avatar.png",
				},
			})
		case "/api/v1/namespaces/user-1/apikeys/builtin":
			if got := r.URL.Query().Get("current_user"); got != "alice" {
				t.Fatalf("builtin current_user = %q, want alice", got)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer "+testJWT("alice", "user-1") {
				t.Fatalf("builtin Authorization = %q", got)
			}
			writeJSON(t, w, map[string]any{
				"msg": "OK",
				"data": map[string]any{
					"token": "gk_aigateway-key",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	store := NewStore(filepath.Join(t.TempDir(), "csghub.json"))
	now := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	service := &Service{
		Store:         store,
		CSGHubBaseURL: api.URL,
		HTTPClient:    api.Client(),
		Now:           func() time.Time { return now },
	}

	returnURL := "http://127.0.0.1:18080/#/dms/room-1"
	redirectURL, err := service.CompleteCallback(context.Background(), url.Values{
		"jwt_token":  []string{testJWT("alice", "user-1")},
		"return_url": []string{returnURL},
	})
	if err != nil {
		t.Fatalf("CompleteCallback() error = %v", err)
	}
	if redirectURL != returnURL {
		t.Fatalf("callback redirect = %q", redirectURL)
	}
	if !sawTokenAuth {
		t.Fatal("token endpoint was not called")
	}

	record, ok, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !ok {
		t.Fatal("auth record not saved")
	}
	if record.AIGatewayBuiltinAPIKey != "gk_aigateway-key" || record.AccessToken != "access-token" {
		t.Fatalf("record = %+v, want saved credentials", record)
	}
	if record.UserID != "alice" || record.UserUUID != "user-1" {
		t.Fatalf("record user = %q/%q", record.UserID, record.UserUUID)
	}
	if record.CSGHubBaseURL != api.URL {
		t.Fatalf("record CSGHubBaseURL = %q, want %q", record.CSGHubBaseURL, api.URL)
	}
	if !record.LoggedInAt.Equal(now) {
		t.Fatalf("LoggedInAt = %s, want %s", record.LoggedInAt, now)
	}
}

func TestLoginUsesOpenCSGSSOCallbackURL(t *testing.T) {
	service := &Service{
		OpenCSGBaseURL: "https://opencsg.example.test",
	}

	returnURL := "http://127.0.0.1:18080/#/dms/room-1"
	callbackURL := "http://127.0.0.1:18080/api/v1/csghub/auth/callback"
	login, err := service.Login(context.Background(), LoginOptions{
		ReturnURL:   returnURL,
		CallbackURL: callbackURL,
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	parsedLogin, err := url.Parse(login.LoginURL)
	if err != nil {
		t.Fatalf("parse LoginURL: %v", err)
	}
	if got := parsedLogin.Scheme + "://" + parsedLogin.Host + parsedLogin.Path; got != "https://opencsg.example.test/sso/login" {
		t.Fatalf("login URL base = %q", got)
	}
	redirectURL := parsedLogin.Query().Get("redirect_url")
	if redirectURL == "" {
		t.Fatal("redirect_url is empty")
	}
	parsedRedirect, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("parse redirect_url: %v", err)
	}
	if got := parsedRedirect.Scheme + "://" + parsedRedirect.Host + parsedRedirect.Path; got != callbackURL {
		t.Fatalf("redirect callback = %q, want %q", got, callbackURL)
	}
	if got := parsedRedirect.Query().Get("return_url"); got != returnURL {
		t.Fatalf("return_url = %q, want %q", got, returnURL)
	}
}

func TestCallbackRejectsMissingParams(t *testing.T) {
	service := &Service{}
	_, err := service.completeCallback(context.Background(), url.Values{})
	if err == nil || !isCallbackValidationError(err) {
		t.Fatalf("completeCallback() error = %v, want validation error", err)
	}
}

func TestCallbackRejectsInvalidJWT(t *testing.T) {
	service := &Service{}
	_, err := service.completeCallback(context.Background(), url.Values{
		"jwt_token": []string{"not-a-jwt"},
	})
	if err == nil || !isCallbackValidationError(err) {
		t.Fatalf("completeCallback() error = %v, want validation error", err)
	}
}

func TestCallbackAllowsMissingBuiltinAPIKey(t *testing.T) {
	var sawBuiltin bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/user/alice/tokens":
			writeJSON(t, w, map[string]any{
				"msg":  "OK",
				"data": []map[string]any{{"token": "access-token"}},
			})
		case "/api/v1/user/alice":
			writeJSON(t, w, map[string]any{"msg": "OK", "data": map[string]any{}})
		case "/api/v1/namespaces/user-1/apikeys/builtin":
			if got := r.URL.Query().Get("current_user"); got != "alice" {
				t.Fatalf("builtin current_user = %q, want alice", got)
			}
			sawBuiltin = true
			http.Error(w, "not available", http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	store := NewStore(filepath.Join(t.TempDir(), "csghub.json"))
	service := &Service{Store: store, CSGHubBaseURL: api.URL, HTTPClient: api.Client()}
	redirect, err := service.completeCallback(context.Background(), url.Values{
		"jwt_token": []string{testJWT("alice", "user-1")},
	})
	if err != nil {
		t.Fatalf("completeCallback() error = %v", err)
	}
	if redirect != api.URL {
		t.Fatalf("redirect = %q", redirect)
	}
	if !sawBuiltin {
		t.Fatal("builtin endpoint was not attempted")
	}
	record, ok, err := store.Load()
	if err != nil || !ok {
		t.Fatalf("Load() = %+v, %v, %v", record, ok, err)
	}
	if record.AIGatewayBuiltinAPIKey != "" {
		t.Fatalf("AIGatewayBuiltinAPIKey = %q, want empty when builtin fetch fails", record.AIGatewayBuiltinAPIKey)
	}
}

func TestCallbackReturnURLAcceptsLangflowURLParam(t *testing.T) {
	returnURL := "http://127.0.0.1:18080/#/workspace"
	got := callbackReturnURL(url.Values{
		"url": []string{returnURL},
	})
	if got != returnURL {
		t.Fatalf("callbackReturnURL() = %q, want %q", got, returnURL)
	}
}

func TestCallbackReturnURLRejectsExternalURLs(t *testing.T) {
	got := callbackReturnURL(url.Values{
		"return_url": []string{"https://evil.example.test/callback"},
		"url":        []string{"https://also-evil.example.test/callback"},
	})
	if got != "" {
		t.Fatalf("callbackReturnURL() = %q, want empty for external URLs", got)
	}
}

func TestLogoutDeletesAuth(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "csghub.json"))
	if err := store.Save(Record{AccessToken: "token", CSGHubBaseURL: "https://hub.example.test"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	service := &Service{Store: store}

	status, err := service.Logout(context.Background())
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if status.Authenticated {
		t.Fatalf("Logout() status = %+v, want unauthenticated", status)
	}
	_, ok, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if ok {
		t.Fatal("auth record still exists after logout")
	}
}

func testJWT(userID, userUUID string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, err := json.Marshal(map[string]string{
		"current_user": userID,
		"uuid":         userUUID,
	})
	if err != nil {
		panic(err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func writeJSON(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
