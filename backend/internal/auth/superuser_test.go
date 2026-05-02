package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/labstack/echo/v5"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

type stubAdminChecker struct {
	fn func(ctx context.Context, userID string) (bool, error)
}

func (s stubAdminChecker) IsAdmin(ctx context.Context, userID string) (bool, error) {
	return s.fn(ctx, userID)
}

type stubRoleAssigner struct {
	fn func(ctx context.Context, userID, roleID string) error
}

func (s stubRoleAssigner) AssignToUser(ctx context.Context, userID, roleID string) error {
	return s.fn(ctx, userID, roleID)
}

// ---------------------------------------------------------------------------
// A. ParseSuperUserSet
// ---------------------------------------------------------------------------

func TestParseSuperUserSet(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		input  string
		expect map[string]struct{}
	}{
		{
			name:   "empty",
			input:  "",
			expect: map[string]struct{}{},
		},
		{
			name:   "single",
			input:  "a@x.com",
			expect: map[string]struct{}{"a@x.com": {}},
		},
		{
			name:   "spaces and case",
			input:  " A@X.com , b@y.com ",
			expect: map[string]struct{}{"a@x.com": {}, "b@y.com": {}},
		},
		{
			name:   "duplicates",
			input:  "a@x.com,A@X.COM,a@x.com",
			expect: map[string]struct{}{"a@x.com": {}},
		},
		{
			name:   "empty entries",
			input:  "a@x.com,,b@y.com,",
			expect: map[string]struct{}{"a@x.com": {}, "b@y.com": {}},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ParseSuperUserSet(tc.input)
			if len(got) != len(tc.expect) {
				t.Fatalf("expected len=%d, got len=%d: %v", len(tc.expect), len(got), got)
			}
			for k := range tc.expect {
				if _, ok := got[k]; !ok {
					t.Errorf("expected key %q in result %v", k, got)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// B. supabaseClaims EmailVerified parsing
// ---------------------------------------------------------------------------

func TestSupabaseClaims_EmailVerified(t *testing.T) {
	t.Parallel()

	t.Run("email_verified true", func(t *testing.T) {
		t.Parallel()
		payload := []byte(`{"email":"u@example.com","email_verified":true,"role":"authenticated"}`)
		var c supabaseClaims
		if err := json.Unmarshal(payload, &c); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !c.EmailVerified {
			t.Error("expected EmailVerified == true")
		}
	})

	t.Run("email_verified absent", func(t *testing.T) {
		t.Parallel()
		payload := []byte(`{"email":"u@example.com","role":"authenticated"}`)
		var c supabaseClaims
		if err := json.Unmarshal(payload, &c); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if c.EmailVerified {
			t.Error("expected EmailVerified == false (zero value when absent)")
		}
	})
}

// ---------------------------------------------------------------------------
// Helpers for middleware tests
// ---------------------------------------------------------------------------

// newEchoContext builds a minimal Echo context from an httptest request so that
// the middleware chain can be exercised without a full Echo HTTP server.
func newEchoContext(t *testing.T, req *http.Request) *echo.Context {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c
}

// runMiddleware runs promoter.Middleware()(next)(c) and returns the response recorder.
func runMiddleware(t *testing.T, promoter *SuperUserPromoter, u *AuthUser) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/query", nil)
	if u != nil {
		ctx := ContextWithUser(req.Context(), u)
		req = req.WithContext(ctx)
	}
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	next := func(c *echo.Context) error {
		return c.NoContent(http.StatusOK)
	}

	if err := promoter.Middleware()(next)(c); err != nil {
		t.Fatalf("middleware returned error: %v", err)
	}
	return rec
}

// captureDefaultLogger replaces the global slog default with a JSON logger
// writing to buf for the duration of the test. Must not be used with
// t.Parallel() because it mutates global state.
func captureDefaultLogger(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
}

// decodeLogLines parses newline-delimited JSON log lines from buf.
func decodeLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		records = append(records, rec)
	}
	return records
}

// makeSet builds a map[string]struct{} from a slice of email strings.
func makeSet(emails ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		m[e] = struct{}{}
	}
	return m
}

// ---------------------------------------------------------------------------
// C. Middleware tests M1–M9
// ---------------------------------------------------------------------------

// M1: empty emails map — IsAdmin and AssignToUser must never be called.
func TestSuperUserPromoter_M1_EmptyEmails(t *testing.T) {
	// Not parallel: captureDefaultLogger mutates global slog default.
	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return false, nil
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(map[string]struct{}{}, "role-id", checker, assigner)

	u := &AuthUser{Sub: "user-1", Email: "a@x.com", EmailVerified: true}
	rec := runMiddleware(t, promoter, u)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if n := isAdminCalls.Load(); n != 0 {
		t.Errorf("expected 0 IsAdmin calls, got %d", n)
	}
	if n := assignCalls.Load(); n != 0 {
		t.Errorf("expected 0 AssignToUser calls, got %d", n)
	}
}

// M2: anonymous request (UserFrom == nil) — no DB calls.
func TestSuperUserPromoter_M2_AnonymousRequest(t *testing.T) {
	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return false, nil
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner)

	rec := runMiddleware(t, promoter, nil /* anonymous */)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if n := isAdminCalls.Load(); n != 0 {
		t.Errorf("expected 0 IsAdmin calls, got %d", n)
	}
	if n := assignCalls.Load(); n != 0 {
		t.Errorf("expected 0 AssignToUser calls, got %d", n)
	}
}

// M3: email not in the configured set — no DB calls.
func TestSuperUserPromoter_M3_EmailNotInSet(t *testing.T) {
	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return false, nil
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("other@x.com"), "role-id", checker, assigner)

	u := &AuthUser{Sub: "user-1", Email: "a@x.com", EmailVerified: true}
	rec := runMiddleware(t, promoter, u)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if n := isAdminCalls.Load(); n != 0 {
		t.Errorf("expected 0 IsAdmin calls, got %d", n)
	}
	if n := assignCalls.Load(); n != 0 {
		t.Errorf("expected 0 AssignToUser calls, got %d", n)
	}
}

// M4: email matches but EmailVerified=false — Q5=B security gate, no DB calls.
func TestSuperUserPromoter_M4_EmailVerifiedFalse(t *testing.T) {
	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return false, nil
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner)

	u := &AuthUser{Sub: "user-1", Email: "a@x.com", EmailVerified: false}
	rec := runMiddleware(t, promoter, u)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if n := isAdminCalls.Load(); n != 0 {
		t.Errorf("expected 0 IsAdmin calls, got %d", n)
	}
	if n := assignCalls.Load(); n != 0 {
		t.Errorf("expected 0 AssignToUser calls, got %d", n)
	}
}

// M5: email matches + verified + already admin — IsAdmin called once, AssignToUser not called, no log.
func TestSuperUserPromoter_M5_AlreadyAdmin(t *testing.T) {
	// Not parallel: captureDefaultLogger mutates global slog default.
	var buf bytes.Buffer
	captureDefaultLogger(t, &buf)

	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return true, nil // already admin
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner)

	u := &AuthUser{Sub: "user-1", Email: "a@x.com", EmailVerified: true}
	rec := runMiddleware(t, promoter, u)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if n := isAdminCalls.Load(); n != 1 {
		t.Errorf("expected 1 IsAdmin call, got %d", n)
	}
	if n := assignCalls.Load(); n != 0 {
		t.Errorf("expected 0 AssignToUser calls, got %d", n)
	}
	if buf.Len() > 0 {
		t.Errorf("expected no log output, got: %s", buf.String())
	}
}

// M6: email matches + verified + not yet admin — AssignToUser called, INFO log with user_id.
func TestSuperUserPromoter_M6_SuccessfulPromotion(t *testing.T) {
	// Not parallel: captureDefaultLogger mutates global slog default.
	var buf bytes.Buffer
	captureDefaultLogger(t, &buf)

	const wantUserID = "user-promoted-1"
	const wantRoleID = "admin-role-uuid"

	var isAdminCalls, assignCalls atomic.Int64
	var gotUserID, gotRoleID string
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return false, nil
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, userID, roleID string) error {
		assignCalls.Add(1)
		gotUserID = userID
		gotRoleID = roleID
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), wantRoleID, checker, assigner)

	u := &AuthUser{Sub: wantUserID, Email: "a@x.com", EmailVerified: true}
	rec := runMiddleware(t, promoter, u)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if n := isAdminCalls.Load(); n != 1 {
		t.Errorf("expected 1 IsAdmin call, got %d", n)
	}
	if n := assignCalls.Load(); n != 1 {
		t.Errorf("expected 1 AssignToUser call, got %d", n)
	}
	if gotUserID != wantUserID {
		t.Errorf("AssignToUser userID: want %q, got %q", wantUserID, gotUserID)
	}
	if gotRoleID != wantRoleID {
		t.Errorf("AssignToUser roleID: want %q, got %q", wantRoleID, gotRoleID)
	}

	// Verify INFO log contains user_id but not the email.
	records := decodeLogLines(t, &buf)
	if len(records) != 1 {
		t.Fatalf("expected 1 log line, got %d: %s", len(records), buf.String())
	}
	rec0 := records[0]
	if rec0["level"] != "INFO" {
		t.Errorf("expected level=INFO, got %v", rec0["level"])
	}
	if rec0["msg"] != "superuser: promoted to admin" {
		t.Errorf("expected msg='superuser: promoted to admin', got %v", rec0["msg"])
	}
	if rec0["user_id"] != wantUserID {
		t.Errorf("expected user_id=%q, got %v", wantUserID, rec0["user_id"])
	}
	if _, hasEmail := rec0["email"]; hasEmail {
		t.Error("INFO log must not contain 'email' field")
	}
}

// M7: IsAdmin returns an error — WARN log with error_chain, AssignToUser not called, 200.
func TestSuperUserPromoter_M7_IsAdminError(t *testing.T) {
	// Not parallel: captureDefaultLogger mutates global slog default.
	var buf bytes.Buffer
	captureDefaultLogger(t, &buf)

	var assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		return false, errors.New("db: connection refused")
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner)

	u := &AuthUser{Sub: "user-1", Email: "a@x.com", EmailVerified: true}
	rec := runMiddleware(t, promoter, u)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if n := assignCalls.Load(); n != 0 {
		t.Errorf("expected 0 AssignToUser calls, got %d", n)
	}

	records := decodeLogLines(t, &buf)
	if len(records) != 1 {
		t.Fatalf("expected 1 log line, got %d: %s", len(records), buf.String())
	}
	rec0 := records[0]
	if rec0["level"] != "WARN" {
		t.Errorf("expected level=WARN, got %v", rec0["level"])
	}
	if rec0["msg"] != "superuser: admin check failed" {
		t.Errorf("unexpected msg: %v", rec0["msg"])
	}
	if _, ok := rec0["error_chain"]; !ok {
		t.Error("expected error_chain attribute in WARN log")
	}
}

// M8: AssignToUser returns an error — WARN log with error_chain, 200.
func TestSuperUserPromoter_M8_AssignToUserError(t *testing.T) {
	// Not parallel: captureDefaultLogger mutates global slog default.
	var buf bytes.Buffer
	captureDefaultLogger(t, &buf)

	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		return false, nil
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		return errors.New("db: deadlock detected")
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner)

	u := &AuthUser{Sub: "user-1", Email: "a@x.com", EmailVerified: true}
	rec := runMiddleware(t, promoter, u)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	records := decodeLogLines(t, &buf)
	if len(records) != 1 {
		t.Fatalf("expected 1 log line, got %d: %s", len(records), buf.String())
	}
	rec0 := records[0]
	if rec0["level"] != "WARN" {
		t.Errorf("expected level=WARN, got %v", rec0["level"])
	}
	if rec0["msg"] != "superuser: role assignment failed" {
		t.Errorf("unexpected msg: %v", rec0["msg"])
	}
	if _, ok := rec0["error_chain"]; !ok {
		t.Error("expected error_chain attribute in WARN log")
	}
}

// M9: concurrent first-login — two goroutines both hit the middleware for the
// same user before either has promoted. The stub mirrors ON CONFLICT DO NOTHING
// by returning nil for both calls. Both goroutines must complete with 200.
func TestSuperUserPromoter_M9_ConcurrentFirstLogin(t *testing.T) {
	// Not parallel: captureDefaultLogger mutates global slog default.
	var buf bytes.Buffer
	captureDefaultLogger(t, &buf)

	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return false, nil // both goroutines see "not yet admin"
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil // ON CONFLICT DO NOTHING equivalent
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner)

	const goroutines = 2
	var wg sync.WaitGroup
	codes := make([]int, goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/query", nil)
			u := &AuthUser{Sub: "user-concurrent", Email: "a@x.com", EmailVerified: true}
			ctx := ContextWithUser(req.Context(), u)
			req = req.WithContext(ctx)

			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			next := func(c *echo.Context) error {
				return c.NoContent(http.StatusOK)
			}
			if err := promoter.Middleware()(next)(c); err != nil {
				codes[i] = 500
				return
			}
			codes[i] = rec.Code
		}()
	}
	wg.Wait()

	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("goroutine %d: expected 200, got %d", i, code)
		}
	}
	if n := isAdminCalls.Load(); n != int64(goroutines) {
		t.Errorf("expected %d IsAdmin calls, got %d", goroutines, n)
	}
	if n := assignCalls.Load(); n != int64(goroutines) {
		t.Errorf("expected %d AssignToUser calls, got %d", goroutines, n)
	}
}
