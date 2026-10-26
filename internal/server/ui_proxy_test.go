package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestUIProxyKeepsCredentialInHostAndPreservesSubpath(t *testing.T) {
	const token = "backend-test-credential"
	var calls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("Cookie") != "" || r.Header.Get("Origin") != "" {
			t.Errorf("forwarded wrong credentials or browser state")
		}
		if r.URL.Path != "/tenant/api/v1/agents/a/events" {
			t.Errorf("lost API prefix: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: hello\n\n"))
	}))
	defer backend.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	session := strings.Repeat("s", 43)
	handler, err := uiProxyHandler(UIProxyOptions{Listener: listener, BackendAPIBase: backend.URL + "/tenant/api/v1/", BackendToken: token, SessionToken: session})
	if err != nil {
		t.Fatal(err)
	}
	call := func(path, authorization string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "http://"+listener.Addr().String()+path, nil)
		req.Header.Set("Authorization", authorization)
		req.Header.Set("Origin", "http://"+listener.Addr().String())
		req.Header.Set("Cookie", "local=private")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	if rec := call("/api/v1/agents/a/events", ""); rec.Code != 401 {
		t.Fatalf("anonymous proxy: %d", rec.Code)
	}
	rec := call("/api/v1/agents/a/events", "Bearer "+session)
	if rec.Code != 200 || rec.Body.String() != "data: hello\n\n" || strings.Contains(rec.Body.String(), token) {
		t.Fatalf("proxy: %d %s", rec.Code, rec.Body)
	}
	if rec = call("/api/v1/agents/a/workspace/open", "Bearer "+session); rec.Code != 422 {
		t.Fatalf("host action: %d", rec.Code)
	}
	if rec = call("/api/v1/../../outside", "Bearer "+session); rec.Code != 400 {
		t.Fatalf("escaped prefix: %d", rec.Code)
	}
	if calls.Load() != 1 {
		t.Fatal("rejected requests reached backend")
	}
}
func TestUIProxyShutdownLeavesBackendAvailable(t *testing.T) {
	if !UIAvailable {
		t.Skip("requires UI host build")
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"backend_id": "test-backend", "control_api_version": 1})
	}))
	defer backend.Close()
	if err := CheckBackendConnection(context.Background(), backend.URL+"/api/v1/", "test-token"); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- RunUIProxy(UIProxyOptions{Context: ctx, Listener: listener, BackendAPIBase: backend.URL + "/api/v1/", BackendToken: "test-token", SessionToken: strings.Repeat("s", 43), OnListening: func() { close(started) }})
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("UI proxy did not start")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(backend.URL + "/healthz")
	if err != nil {
		t.Fatal("backend was stopped with UI", err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("backend stopped")
	}
}
func TestBackendHandshakeDoesNotFollowCredentialRedirects(t *testing.T) {
	var reached atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached.Store(true) }))
	defer target.Close()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer backend.Close()
	if CheckBackendConnection(context.Background(), backend.URL+"/api/v1/", "test-token") == nil || reached.Load() {
		t.Fatal("followed a credential-bearing redirect")
	}
}
