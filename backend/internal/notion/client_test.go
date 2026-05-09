package notion

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRetryAfterRoundTripperRetries429ThenSucceeds(t *testing.T) {
	t.Parallel()

	attempts := 0
	var slept []time.Duration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := &http.Client{Transport: NewRetryAfterRoundTripper(RetryConfig{
		MaxAttempts: 3,
		MaxElapsed:  5 * time.Second,
		Sleep: func(_ context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		},
	})}
	res, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if len(slept) != 1 || slept[0] != time.Second {
		t.Fatalf("slept = %v, want [1s]", slept)
	}
}

func TestRetryAfterRoundTripperStopsAtMaxElapsed(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	client := &http.Client{Transport: NewRetryAfterRoundTripper(RetryConfig{
		MaxAttempts: 3,
		MaxElapsed:  time.Second,
		Sleep: func(_ context.Context, _ time.Duration) error {
			t.Fatal("sleep must not run when max elapsed is exceeded")
			return nil
		},
	})}
	_, err := client.Get(srv.URL)
	if !errors.Is(err, ErrRetryElapsed) {
		t.Fatalf("err = %v, want ErrRetryElapsed", err)
	}
}

func TestRetryAfterRoundTripperStopsAtMaxAttempts(t *testing.T) {
	t.Parallel()

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	client := &http.Client{Transport: NewRetryAfterRoundTripper(RetryConfig{
		MaxAttempts: 2,
		MaxElapsed:  5 * time.Second,
		Sleep: func(_ context.Context, _ time.Duration) error {
			return nil
		},
	})}
	_, err := client.Get(srv.URL)
	if !errors.Is(err, ErrRetryAttempts) {
		t.Fatalf("err = %v, want ErrRetryAttempts", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestRetryAfterRoundTripperRetries5xxWithFallback(t *testing.T) {
	t.Parallel()

	attempts := 0
	var slept []time.Duration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := &http.Client{Transport: NewRetryAfterRoundTripper(RetryConfig{
		MaxAttempts: 2,
		MaxElapsed:  5 * time.Second,
		Sleep: func(_ context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		},
	})}
	res, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if len(slept) != 1 || slept[0] != time.Second {
		t.Fatalf("slept = %v, want [1s]", slept)
	}
}
