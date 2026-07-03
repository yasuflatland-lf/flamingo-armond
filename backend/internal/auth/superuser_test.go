package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/rotisserie/eris"
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
	t.Parallel()
	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return false, nil
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(map[string]struct{}{}, "role-id", checker, assigner, nil)

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
	t.Parallel()
	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return false, nil
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner, nil)

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
	t.Parallel()
	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return false, nil
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("other@x.com"), "role-id", checker, assigner, nil)

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

// M4: email matches but EmailVerified=false — unverified emails are not promoted, no DB calls.
func TestSuperUserPromoter_M4_EmailVerifiedFalse(t *testing.T) {
	t.Parallel()
	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return false, nil
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner, nil)

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
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner, nil)

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
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), wantRoleID, checker, assigner, nil)

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

	const wantUserID = "user-1"
	var assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		return false, eris.New("db: connection refused")
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner, nil)

	u := &AuthUser{Sub: wantUserID, Email: "a@x.com", EmailVerified: true}
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
	if rec0["user_id"] != wantUserID {
		t.Errorf("expected user_id=%q in WARN log, got %v", wantUserID, rec0["user_id"])
	}
	if _, hasEmail := rec0["email"]; hasEmail {
		t.Error("WARN log must not contain 'email' field (PII protection)")
	}
	chain, ok := rec0["error_chain"].(map[string]any)
	if !ok {
		t.Fatalf("error_chain is not a JSON object: %T", rec0["error_chain"])
	}
	root, hasRoot := chain["root"].(map[string]any)
	if !hasRoot {
		t.Error("error_chain must have root entry (got external-only shape; stub may be using stdlib errors)")
	}
	if root != nil {
		if stack, _ := root["stack"].([]any); len(stack) == 0 {
			t.Error("error_chain.root.stack must contain at least one frame")
		}
	}
}

// TestSuperUserPromoter_InjectedLogger_WarnsOnAdminCheckFailure verifies the
// logger-DI path: a logger passed to NewSuperUserPromoter (writing to a local
// bytes.Buffer) is the one the middleware uses, NOT the global slog.Default().
// The admin-check-failure path must emit its WARN line through the injected
// logger. This is the DI replacement for the captureDefaultLogger global-mutation
// pattern the M5–M9 tests use; it runs in parallel because it touches no global
// state.
func TestSuperUserPromoter_InjectedLogger_WarnsOnAdminCheckFailure(t *testing.T) {
	t.Parallel()

	const wantUserID = "user-injected-1"
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		return false, eris.New("db: connection refused")
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		t.Fatal("AssignToUser must not be called when the admin check fails")
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner, logger)

	u := &AuthUser{Sub: wantUserID, Email: "a@x.com", EmailVerified: true}
	rec := runMiddleware(t, promoter, u)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	records := decodeLogLines(t, &buf)
	if len(records) != 1 {
		t.Fatalf("expected 1 log line on the injected logger, got %d: %s", len(records), buf.String())
	}
	rec0 := records[0]
	if rec0["level"] != "WARN" {
		t.Errorf("expected level=WARN, got %v", rec0["level"])
	}
	if rec0["msg"] != "superuser: admin check failed" {
		t.Errorf("unexpected msg: %v", rec0["msg"])
	}
	if rec0["user_id"] != wantUserID {
		t.Errorf("expected user_id=%q in WARN log, got %v", wantUserID, rec0["user_id"])
	}
	if _, hasEmail := rec0["email"]; hasEmail {
		t.Error("WARN log must not contain 'email' field (PII protection)")
	}
	if _, hasChain := rec0["error_chain"]; !hasChain {
		t.Error("WARN log must carry the error_chain attribute")
	}
}

// M8: AssignToUser returns an error — WARN log with error_chain, 200.
func TestSuperUserPromoter_M8_AssignToUserError(t *testing.T) {
	// Not parallel: captureDefaultLogger mutates global slog default.
	var buf bytes.Buffer
	captureDefaultLogger(t, &buf)

	const wantUserID = "user-1"
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		return false, nil
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		return eris.New("db: deadlock detected")
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner, nil)

	u := &AuthUser{Sub: wantUserID, Email: "a@x.com", EmailVerified: true}
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
	if rec0["user_id"] != wantUserID {
		t.Errorf("expected user_id=%q in WARN log, got %v", wantUserID, rec0["user_id"])
	}
	if _, hasEmail := rec0["email"]; hasEmail {
		t.Error("WARN log must not contain 'email' field (PII protection)")
	}
	chain, ok := rec0["error_chain"].(map[string]any)
	if !ok {
		t.Fatalf("error_chain is not a JSON object: %T", rec0["error_chain"])
	}
	root, hasRoot := chain["root"].(map[string]any)
	if !hasRoot {
		t.Error("error_chain must have root entry (got external-only shape; stub may be using stdlib errors)")
	}
	if root != nil {
		if stack, _ := root["stack"].([]any); len(stack) == 0 {
			t.Error("error_chain.root.stack must contain at least one frame")
		}
	}
}

// M9: concurrent first-login — two goroutines both hit the middleware for the
// same user before either has promoted. The stub mirrors ON CONFLICT DO NOTHING
// by returning nil for both calls. Both goroutines must complete with 200.
//
// With the process-lifetime confirmed-sub cache, the exact IsAdmin/AssignToUser
// call counts on this path are non-deterministic: if one goroutine records the
// sub before the other reads the cache, the second skips both DB calls. So the
// counts are bounded (at least one call to promote, at most one per goroutine)
// rather than pinned to an exact value. The invariant the cache must preserve is
// that both goroutines complete with 200 and the user ends up promoted.
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
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner, nil)

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
	// The cache makes the counts race-dependent: at least one call promotes the
	// user, and no goroutine issues more than one call. See the cache note above.
	if n := isAdminCalls.Load(); n < 1 || n > int64(goroutines) {
		t.Errorf("expected 1..%d IsAdmin calls, got %d", goroutines, n)
	}
	if n := assignCalls.Load(); n < 1 || n > int64(goroutines) {
		t.Errorf("expected 1..%d AssignToUser calls, got %d", goroutines, n)
	}
}

// TestSuperUserPromoter_CachesConfirmedAdmin verifies that an already-admin
// super-user triggers at most one IsAdmin role query across repeated requests
// within a process: the first request records the sub in the confirmed-sub
// cache and every later request short-circuits before the query.
func TestSuperUserPromoter_CachesConfirmedAdmin(t *testing.T) {
	t.Parallel()
	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return true, nil // already admin
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner, nil)

	u := &AuthUser{Sub: "user-1", Email: "a@x.com", EmailVerified: true}
	const requests = 5
	for i := 0; i < requests; i++ {
		rec := runMiddleware(t, promoter, u)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}
	if n := isAdminCalls.Load(); n != 1 {
		t.Errorf("expected exactly 1 IsAdmin call across %d requests, got %d", requests, n)
	}
	if n := assignCalls.Load(); n != 0 {
		t.Errorf("expected 0 AssignToUser calls, got %d", n)
	}
}

// TestSuperUserPromoter_CachesAfterPromotion verifies a cold cache still promotes
// correctly, and that once a sub has been promoted the confirmed-sub cache halts
// every subsequent IsAdmin query and AssignToUser call for that sub.
func TestSuperUserPromoter_CachesAfterPromotion(t *testing.T) {
	t.Parallel()
	var isAdminCalls, assignCalls atomic.Int64
	checker := stubAdminChecker{fn: func(_ context.Context, _ string) (bool, error) {
		isAdminCalls.Add(1)
		return false, nil // never admin at query time
	}}
	assigner := stubRoleAssigner{fn: func(_ context.Context, _, _ string) error {
		assignCalls.Add(1)
		return nil
	}}
	// Discard the INFO promotion log so the test can run in parallel without
	// mutating the global slog default.
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	promoter := NewSuperUserPromoter(makeSet("a@x.com"), "role-id", checker, assigner, logger)

	u := &AuthUser{Sub: "user-1", Email: "a@x.com", EmailVerified: true}
	const requests = 5
	for i := 0; i < requests; i++ {
		rec := runMiddleware(t, promoter, u)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}
	// Cold cache: the first request runs IsAdmin then AssignToUser. Every later
	// request short-circuits on the cache, so both counters stay at 1.
	if n := isAdminCalls.Load(); n != 1 {
		t.Errorf("expected exactly 1 IsAdmin call across %d requests, got %d", requests, n)
	}
	if n := assignCalls.Load(); n != 1 {
		t.Errorf("expected exactly 1 AssignToUser call across %d requests, got %d", requests, n)
	}
}

// TestSuperUserPromoter_AuthUserWithEmptySub verifies that a non-nil AuthUser
// with an empty Sub field (u.Sub == "") is treated as anonymous and neither
// IsAdmin nor AssignToUser is called, even when the email is in the set.
func TestSuperUserPromoter_AuthUserWithEmptySub(t *testing.T) {
	t.Parallel()
	var checkerCalls atomic.Int64
	var assignerCalls atomic.Int64
	promoter := NewSuperUserPromoter(
		map[string]struct{}{"a@x.com": {}},
		"admin-role-id",
		stubAdminChecker{fn: func(ctx context.Context, _ string) (bool, error) {
			checkerCalls.Add(1)
			return false, nil
		}},
		stubRoleAssigner{fn: func(ctx context.Context, _, _ string) error {
			assignerCalls.Add(1)
			return nil
		}},
		nil,
	)
	u := &AuthUser{Sub: "", Email: "a@x.com", EmailVerified: true}
	rec := runMiddleware(t, promoter, u)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if checkerCalls.Load() != 0 {
		t.Errorf("expected IsAdmin not called, got %d", checkerCalls.Load())
	}
	if assignerCalls.Load() != 0 {
		t.Errorf("expected AssignToUser not called, got %d", assignerCalls.Load())
	}
}

// TestNewSuperUserPromoter_NilMapIsPassthrough verifies that constructing a
// promoter with a nil email map does not panic, and that the resulting
// middleware is a zero-cost pass-through that never calls checker or assigner.
func TestNewSuperUserPromoter_NilMapIsPassthrough(t *testing.T) {
	t.Parallel()
	// No checker/assigner provided; must not be called when map is empty/nil.
	promoter := NewSuperUserPromoter(nil, "", nil, nil, nil)
	u := &AuthUser{Sub: "user-1", Email: "anyone@example.com", EmailVerified: true}
	rec := runMiddleware(t, promoter, u)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}
