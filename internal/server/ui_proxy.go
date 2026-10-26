package server

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type UIProxyOptions struct {
	Context        context.Context
	Listener       net.Listener
	BackendAPIBase string
	BackendToken   string
	SessionToken   string
	OnListening    func()
}

func ValidateBackendConnection(apiBase, token string) (*url.URL, error) {
	u, err := url.Parse(apiBase)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(u.Path, "/") {
		return nil, fmt.Errorf("backend API base must be an HTTP(S) API prefix ending with / and contain no credentials or query")
	}
	if strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n") {
		return nil, fmt.Errorf("backend service credential is required")
	}
	return u, nil
}

// CheckBackendConnection verifies the control protocol and service credential
// without starting an Engine or following redirects with the credential.
func CheckBackendConnection(ctx context.Context, apiBase, token string) error {
	base, err := ValidateBackendConnection(apiBase, token)
	if err != nil {
		return err
	}
	target := base.ResolveReference(&url.URL{Path: "control/capabilities"})
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("backend connection failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("backend credential or control protocol was rejected (HTTP %d)", resp.StatusCode)
	}
	var capabilities struct {
		Version   int    `json:"control_api_version"`
		BackendID string `json:"backend_id"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&capabilities) != nil || capabilities.Version != 1 || capabilities.BackendID == "" {
		return fmt.Errorf("backend control protocol is not supported")
	}
	return nil
}
func uiProxyHandler(opts UIProxyOptions) (http.Handler, error) {
	base, err := ValidateBackendConnection(opts.BackendAPIBase, opts.BackendToken)
	if err != nil {
		return nil, err
	}
	if opts.Listener == nil {
		return nil, fmt.Errorf("UI proxy requires a loopback listener")
	}
	proxy := &httputil.ReverseProxy{FlushInterval: -1, ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "backend is unavailable", http.StatusBadGateway)
	}}
	proxy.Rewrite = func(request *httputil.ProxyRequest) {
		suffix := strings.TrimPrefix(request.In.URL.Path, "/api/v1/")
		escaped := strings.TrimPrefix(request.In.URL.EscapedPath(), "/api/v1/")
		request.Out.URL.Path = "/" + suffix
		request.Out.URL.RawPath = "/" + escaped
		request.SetURL(base)
		request.Out.Header.Del("Accept-Encoding")
		request.Out.Header.Del("Cookie")
		request.Out.Header.Del("Origin")
		request.Out.Header.Del("Referer")
		request.Out.Header.Del("X-CSGClaw-Caller-Agent")
		request.Out.Header.Set("Authorization", "Bearer "+opts.BackendToken)
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		// The UI host never opens files or directories on the backend machine.
		if strings.HasSuffix(response.Request.URL.Path, "/bootstrap/config") && response.StatusCode == 200 {
			var value map[string]any
			decoder := json.NewDecoder(io.LimitReader(response.Body, 1024*1024))
			err := decoder.Decode(&value)
			response.Body.Close()
			if err != nil {
				return err
			}
			value["directory_picker_available"] = false
			value["backend_connection_mode"] = "external"
			body, err := json.Marshal(value)
			if err != nil {
				return err
			}
			response.Body = io.NopCloser(bytes.NewReader(body))
			response.ContentLength = int64(len(body))
			response.Header.Del("Content-Length")
		}
		// Remote cookies belong to the backend origin, not the local renderer host.
		response.Header.Del("Set-Cookie")
		return nil
	}
	ui := uiFallbackHandler()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			ui.ServeHTTP(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+opts.SessionToken)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			http.NotFound(w, r)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		for _, part := range strings.Split(rel, "/") {
			if part == "." || part == ".." {
				http.Error(w, "invalid API path", http.StatusBadRequest)
				return
			}
		}
		if rel == "local/directory-picker" || strings.HasSuffix(rel, "/workspace/open") || strings.HasSuffix(rel, "/workspace/location") || (strings.Contains(rel, "/attachments/") && (strings.HasSuffix(rel, "/open") || strings.HasSuffix(rel, "/location"))) {
			http.Error(w, "host action unavailable for an external backend", http.StatusUnprocessableEntity)
			return
		}
		proxy.ServeHTTP(w, r)
	})
	return desktopRendererSecurityHandler(handler, opts.Listener.Addr(), DesktopOptions{BaseURL: "http://" + opts.Listener.Addr().String(), SessionToken: opts.SessionToken})
}

// RunUIProxy hosts only UI assets and authenticated forwarding. It owns no
// Engine, IM service, task, or backend process, so closing it cannot stop work.
func RunUIProxy(opts UIProxyOptions) error {
	if !UIAvailable {
		return fmt.Errorf("this binary has no UI assets")
	}
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	handler, err := uiProxyHandler(opts)
	if err != nil {
		return err
	}
	srv := newHTTPServer(opts.Listener.Addr().String(), handler)
	done := make(chan error, 1)
	go func() { done <- srv.Serve(opts.Listener) }()
	if opts.OnListening != nil {
		opts.OnListening()
	}
	select {
	case err := <-done:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-opts.Context.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Close()
		_ = srv.Shutdown(ctx)
		err := <-done
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
