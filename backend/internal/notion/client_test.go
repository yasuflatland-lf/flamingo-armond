package notion

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
		Transport:   srv.Client().Transport,
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
		Transport:   srv.Client().Transport,
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
		Transport:   srv.Client().Transport,
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

// recordingRoundTripper reads and stores each request body it receives, then
// returns the pre-programmed status codes in order (falling back to 200 once
// the list is exhausted). It lets a test assert what payload each retry sends.
type recordingRoundTripper struct {
	statuses []int
	bodies   [][]byte
	idx      int
}

func (s *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		body = b
	}
	s.bodies = append(s.bodies, body)

	status := http.StatusOK
	if s.idx < len(s.statuses) {
		status = s.statuses[s.idx]
	}
	s.idx++
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}, nil
}

func TestRetryAfterRoundTripperRewindsRequestBodyOnRetry(t *testing.T) {
	t.Parallel()

	const payload = `{"children":[{"type":"paragraph","paragraph":{"rich_text":[]}}]}`
	stub := &recordingRoundTripper{statuses: []int{http.StatusTooManyRequests, http.StatusOK}}

	req, err := http.NewRequest(
		http.MethodPatch,
		"https://api.notion.com/v1/blocks/block-id/children",
		bytes.NewBufferString(payload),
	)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	rt := NewRetryAfterRoundTripper(RetryConfig{
		MaxAttempts: 3,
		MaxElapsed:  5 * time.Second,
		Transport:   stub,
		Sleep: func(_ context.Context, _ time.Duration) error {
			return nil
		},
	})

	res, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if len(stub.bodies) != 2 {
		t.Fatalf("attempts = %d, want 2", len(stub.bodies))
	}
	if got := string(stub.bodies[1]); got != payload {
		t.Fatalf("attempt 2 body = %q, want full payload %q", got, payload)
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
		Transport:   srv.Client().Transport,
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

func TestRetryAfterRoundTripperSurfacesResponseForNonRewindableBody(t *testing.T) {
	t.Parallel()

	stub := &recordingRoundTripper{statuses: []int{http.StatusTooManyRequests}}

	req, err := http.NewRequest(
		http.MethodPatch,
		"https://api.notion.com/v1/blocks/block-id/children",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	// A body with no GetBody cannot be rewound, so a retry would re-send an
	// empty payload. The round-tripper must surface the 429 response instead.
	req.Body = io.NopCloser(strings.NewReader("x"))
	req.GetBody = nil

	rt := NewRetryAfterRoundTripper(RetryConfig{
		MaxAttempts: 3,
		MaxElapsed:  5 * time.Second,
		Transport:   stub,
		Sleep: func(_ context.Context, _ time.Duration) error {
			t.Fatal("sleep must not run when the body cannot be rewound")
			return nil
		},
	})

	res, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: unexpected error %v", err)
	}
	if res == nil {
		t.Fatal("RoundTrip: response must be surfaced, got nil")
	}
	if res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", res.StatusCode)
	}
	if len(stub.bodies) != 1 {
		t.Fatalf("transport invocations = %d, want 1 (no retry)", len(stub.bodies))
	}
}

func TestRetryAfterRoundTripperGetBodyErrorWraps(t *testing.T) {
	t.Parallel()

	const payload = `{"children":[]}`
	stub := &recordingRoundTripper{statuses: []int{http.StatusTooManyRequests}}

	req, err := http.NewRequest(
		http.MethodPatch,
		"https://api.notion.com/v1/blocks/block-id/children",
		bytes.NewBuffer([]byte(payload)),
	)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	// GetBody failing on rewind is an unrecoverable error; the round-tripper
	// must wrap and return it rather than retry with a broken body.
	wantErr := errors.New("rewind boom")
	req.GetBody = func() (io.ReadCloser, error) {
		return nil, wantErr
	}

	rt := NewRetryAfterRoundTripper(RetryConfig{
		MaxAttempts: 3,
		MaxElapsed:  5 * time.Second,
		Transport:   stub,
		Sleep: func(_ context.Context, _ time.Duration) error {
			return nil
		},
	})

	res, err := rt.RoundTrip(req)
	if err == nil {
		t.Fatalf("RoundTrip: expected error, got response %v", res)
	}
	if !strings.Contains(err.Error(), "notion: rewind request body") {
		t.Fatalf("err = %q, want it to contain %q", err.Error(), "notion: rewind request body")
	}
}
