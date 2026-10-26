package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExternalUIPairingOriginAndRevocation(t *testing.T) {
	h, _ := newControlTestHandler(t)
	const origin = "http://localhost:5173"
	if err := h.SetUIOrigins([]string{origin}); err != nil {
		t.Fatal(err)
	}
	call := func(method, path, from, token string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Origin", from)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		return rec
	}
	if got := call("OPTIONS", "/api/v1/agents/a/configuration", origin, "", nil); got.Code != 204 || got.Header().Get("Access-Control-Allow-Origin") != origin {
		t.Fatalf("preflight: %d", got.Code)
	}
	if got := call("POST", "/api/v1/control/pair", "https://evil.example", "service-test", nil); got.Code != 403 {
		t.Fatalf("origin: %d", got.Code)
	}
	pair := call("POST", "/api/v1/control/pair", origin, "service-test", nil)
	if pair.Code != 200 || len(pair.Result().Cookies()) != 1 {
		t.Fatalf("pair: %d %s", pair.Code, pair.Body)
	}
	cookie := pair.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe cookie")
	}
	if got := call("GET", "/api/v1/agents/a/configuration", origin, "", cookie); got.Code != 200 {
		t.Fatalf("cookie auth: %d %s", got.Code, got.Body)
	}
	if got := call("POST", "/api/v1/control/logout", origin, "", cookie); got.Code != 204 {
		t.Fatalf("logout: %d", got.Code)
	}
	if got := call("GET", "/api/v1/agents/a/configuration", origin, "", cookie); got.Code != 401 {
		t.Fatalf("revoked: %d", got.Code)
	}
}
func TestExternalUIOriginsRequireExactOrigins(t *testing.T) {
	for _, origin := range []string{"*", "https://example.com/path", "file:///tmp/ui", "https://u:p@example.com", "https://example.com?token=x"} {
		if ValidateUIOrigins([]string{origin}) == nil {
			t.Fatalf("accepted %s", origin)
		}
	}
}
