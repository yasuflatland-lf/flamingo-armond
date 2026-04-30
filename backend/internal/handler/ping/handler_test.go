package ping

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

// fakePingRepo is an in-memory implementation of repository.PingRecordRepository.
type fakePingRepo struct {
	rows      int64
	countErr  error
	createErr error
	deleteErr error
}

func (f *fakePingRepo) Count(_ context.Context) (int64, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.rows, nil
}

func (f *fakePingRepo) Create(_ context.Context) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.rows++
	return nil
}

func (f *fakePingRepo) DeleteAll(_ context.Context) (int64, error) {
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	deleted := f.rows
	f.rows = 0
	return deleted, nil
}

const testToken = "secret-token"

func newTestContext(e *echo.Echo, token string) (*echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPost, "/internal/ping", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

func TestNew_PanicsOnEmptyToken(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty token, got none")
		}
	}()
	_ = New(&fakePingRepo{}, "")
}

func TestHandler_Create(t *testing.T) {
	e := echo.New()
	h := New(&fakePingRepo{rows: 0}, testToken)
	c, rec := newTestContext(e, testToken)

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"action":"created"`) {
		t.Errorf("body %q missing action:created", body)
	}
	if !strings.Contains(body, `"count":1`) {
		t.Errorf("body %q missing count:1", body)
	}
}

func TestHandler_Delete(t *testing.T) {
	e := echo.New()
	h := New(&fakePingRepo{rows: 1}, testToken)
	c, rec := newTestContext(e, testToken)

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"action":"deleted"`) {
		t.Errorf("body %q missing action:deleted", body)
	}
	if !strings.Contains(body, `"count":1`) {
		t.Errorf("body %q missing count:1", body)
	}
}

func TestHandler_Unauthorized_NoHeader(t *testing.T) {
	e := echo.New()
	h := New(&fakePingRepo{}, testToken)
	c, rec := newTestContext(e, "")

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(rec.Body.String(), `"error":"unauthorized"`) {
		t.Errorf("body %q missing error:unauthorized", rec.Body.String())
	}
}

func TestHandler_Unauthorized_WrongToken(t *testing.T) {
	e := echo.New()
	h := New(&fakePingRepo{}, testToken)
	c, rec := newTestContext(e, "wrong")

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandler_Unauthorized_MalformedHeader(t *testing.T) {
	e := echo.New()
	h := New(&fakePingRepo{}, testToken)
	req := httptest.NewRequest(http.MethodPost, "/internal/ping", nil)
	req.Header.Set("Authorization", "foo bar baz")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandler_Count_RepoError(t *testing.T) {
	e := echo.New()
	h := New(&fakePingRepo{countErr: errors.New("boom")}, testToken)
	c, rec := newTestContext(e, testToken)

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"error":"internal server error"`) {
		t.Errorf("body %q missing sanitized error message", body)
	}
	if strings.Contains(body, "boom") {
		t.Errorf("body %q must not contain raw error string", body)
	}
}

func TestHandler_Create_RepoError(t *testing.T) {
	e := echo.New()
	h := New(&fakePingRepo{rows: 0, createErr: errors.New("create boom")}, testToken)
	c, rec := newTestContext(e, testToken)

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"error":"internal server error"`) {
		t.Errorf("body %q missing sanitized error message", body)
	}
	if strings.Contains(body, "create boom") {
		t.Errorf("body %q must not contain raw error string", body)
	}
}

func TestHandler_DeleteAll_RepoError(t *testing.T) {
	e := echo.New()
	h := New(&fakePingRepo{rows: 1, deleteErr: errors.New("delete boom")}, testToken)
	c, rec := newTestContext(e, testToken)

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"error":"internal server error"`) {
		t.Errorf("body %q missing sanitized error message", body)
	}
	if strings.Contains(body, "delete boom") {
		t.Errorf("body %q must not contain raw error string", body)
	}
}

func TestHandler_RateLimiter_429(t *testing.T) {
	e := echo.New()
	repo := &fakePingRepo{}
	h := New(repo, testToken)

	// Register route with rate limiter to get real middleware chain.
	e.POST("/internal/ping", h.Handle, h.RateLimiter())

	got200 := false
	got429 := false
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/internal/ping", nil)
		req.Header.Set("Authorization", "Bearer "+testToken)
		// Use a fixed IP so all requests share the same rate-limiter bucket.
		req.Header.Set("X-Real-IP", "192.0.2.1")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			got200 = true
		}
		if rec.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got200 {
		t.Error("expected at least one 200 from initial requests, got none")
	}
	if !got429 {
		t.Error("expected at least one 429 after 10 rapid requests, got none")
	}
}
