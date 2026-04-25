package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"backend/graph/resolver"
)

func noopAuthMW(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error { return next(c) }
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(newRouter(&resolver.Resolver{}, noopAuthMW))
	t.Cleanup(ts.Close)
	return ts
}

func getJSON(t *testing.T, url string) (int, map[string]string) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode JSON: %v (body=%q)", err, body)
	}
	return res.StatusCode, payload
}

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)

	status, body := getJSON(t, ts.URL+"/health")
	if status != http.StatusOK {
		t.Errorf("status = %d, want %d", status, http.StatusOK)
	}
	if got, want := body["status"], "ok"; got != want {
		t.Errorf("body[\"status\"] = %q, want %q", got, want)
	}
}

func TestRootEndpoint(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)

	status, body := getJSON(t, ts.URL+"/")
	if status != http.StatusOK {
		t.Errorf("status = %d, want %d", status, http.StatusOK)
	}
	if got, want := body["service"], "flamingo-armond-backend"; got != want {
		t.Errorf("body[\"service\"] = %q, want %q", got, want)
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, port, err := net.SplitHostPort(l.Addr().String())
	_ = l.Close()
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	return port
}

func waitHealthy(t *testing.T, port string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	url := "http://127.0.0.1:" + port + "/health"
	for time.Now().Before(deadline) {
		res, err := http.Get(url)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become healthy within %s", url, timeout)
}

func TestRunGracefulShutdown(t *testing.T) {
	tsJWKS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys": []}`))
	}))
	defer tsJWKS.Close()

	t.Setenv("SUPABASE_JWKS_URL", tsJWKS.URL)
	t.Setenv("SUPABASE_JWT_AUDIENCE", "authenticated")
	t.Setenv("SUPABASE_JWT_ISSUER", "http://issuer.test")

	port := freePort(t)
	t.Setenv("PORT", port)
	t.Setenv("SHUTDOWN_TIMEOUT", "2s")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, slog.New(slog.DiscardHandler))
	}()

	waitHealthy(t, port, 3*time.Second)

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return within timeout after context cancel")
	}
}

func TestRun_FailsWhenJWKSURLMissing(t *testing.T) {
	t.Setenv("SUPABASE_JWKS_URL", "")
	t.Setenv("SUPABASE_JWT_AUDIENCE", "authenticated")
	t.Setenv("SUPABASE_JWT_ISSUER", "http://issuer.test")

	err := run(context.Background(), slog.New(slog.DiscardHandler))
	if err == nil {
		t.Fatal("expected run to fail when SUPABASE_JWKS_URL is empty")
	}
}

func TestGraphQLHealth(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)

	body := bytes.NewBufferString(`{"query":"{ health }"}`)
	res, err := http.Post(ts.URL+"/query", "application/json", body)
	if err != nil {
		t.Fatalf("POST /query: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}

	var payload struct {
		Data struct {
			Health string `json:"health"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Errors) != 0 {
		t.Fatalf("errors = %+v, want none", payload.Errors)
	}
	if payload.Data.Health != "ok" {
		t.Fatalf("data.health = %q, want %q", payload.Data.Health, "ok")
	}
}
