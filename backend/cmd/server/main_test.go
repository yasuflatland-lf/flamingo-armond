package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(newRouter())
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
