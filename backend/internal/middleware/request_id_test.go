package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	mw "backend/internal/middleware"
)

func noopHandler(c *echo.Context) error { return nil }

func applyMiddleware(t *testing.T, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw.RequestID()(noopHandler)
	if err := handler(c); err != nil {
		t.Fatalf("middleware returned unexpected error: %v", err)
	}
	return rec
}

func TestRequestID_EmptyHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var capturedID string
	handler := mw.RequestID()(func(c *echo.Context) error {
		capturedID = mw.RequestIDFromContext(c.Request().Context())
		return nil
	})

	if err := handler(c); err != nil {
		t.Fatalf("middleware error: %v", err)
	}

	if capturedID == "" {
		t.Error("expected a generated request ID in context, got empty string")
	}
	responseID := rec.Header().Get(mw.RequestIDHeader)
	if responseID == "" {
		t.Error("expected X-Request-ID response header to be set")
	}
	if capturedID != responseID {
		t.Errorf("context ID %q != response header ID %q", capturedID, responseID)
	}
}

func TestRequestID_UpstreamHeaderRespected(t *testing.T) {
	const upstreamID = "upstream-id-abc123"
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(mw.RequestIDHeader, upstreamID)

	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var capturedID string
	handler := mw.RequestID()(func(c *echo.Context) error {
		capturedID = mw.RequestIDFromContext(c.Request().Context())
		return nil
	})

	if err := handler(c); err != nil {
		t.Fatalf("middleware error: %v", err)
	}

	if capturedID != upstreamID {
		t.Errorf("expected context ID %q, got %q", upstreamID, capturedID)
	}
	if got := rec.Header().Get(mw.RequestIDHeader); got != upstreamID {
		t.Errorf("expected response header %q, got %q", upstreamID, got)
	}
}

func TestRequestID_OverlongHeaderRegenerated(t *testing.T) {
	overlong := strings.Repeat("x", 129)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(mw.RequestIDHeader, overlong)

	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var capturedID string
	handler := mw.RequestID()(func(c *echo.Context) error {
		capturedID = mw.RequestIDFromContext(c.Request().Context())
		return nil
	})

	if err := handler(c); err != nil {
		t.Fatalf("middleware error: %v", err)
	}

	if capturedID == overlong {
		t.Error("overlong incoming ID must not be propagated; expected a new generated ID")
	}
	if capturedID == "" {
		t.Error("expected a generated replacement ID, got empty string")
	}
	responseID := rec.Header().Get(mw.RequestIDHeader)
	if responseID == overlong {
		t.Error("overlong ID must not appear in response header")
	}
	if responseID != capturedID {
		t.Errorf("context ID %q != response header ID %q", capturedID, responseID)
	}
}

func TestRequestIDFromContext_BareCtx(t *testing.T) {
	got := mw.RequestIDFromContext(context.Background())
	if got != "" {
		t.Errorf("expected empty string from bare context, got %q", got)
	}
}
