package auth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
)

type testFixture struct {
	priv     *ecdsa.PrivateKey
	kid      string
	jwksURL  string
	audience string
	issuer   string
	mwCtx    context.Context
}

func newFixture(t *testing.T) *testFixture {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	kid := "test-kid"
	xB64 := base64.RawURLEncoding.EncodeToString(priv.PublicKey.X.Bytes())
	yB64 := base64.RawURLEncoding.EncodeToString(priv.PublicKey.Y.Bytes())
	jwks := map[string]any{"keys": []map[string]any{{
		"kty": "EC", "crv": "P-256", "alg": "ES256",
		"kid": kid, "x": xB64, "y": yB64, "use": "sig",
	}}}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(ts.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &testFixture{
		priv: priv, kid: kid, jwksURL: ts.URL,
		audience: "authenticated", issuer: "http://issuer.test", mwCtx: ctx,
	}
}

// signJWT signs claims with the given method. If key is nil, the fixture's ES256 private key is used.
// Pass jwt.UnsafeAllowNoneSignatureType for SigningMethodNone, or []byte for HS*.
func (f *testFixture) signJWT(t *testing.T, claims jwt.MapClaims, alg jwt.SigningMethod, key any, kid string) string {
	t.Helper()
	tok := jwt.NewWithClaims(alg, claims)
	if kid == "" {
		kid = f.kid
	}
	tok.Header["kid"] = kid
	if key == nil {
		key = f.priv
	}
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func (f *testFixture) newEcho(t *testing.T) *echo.Echo {
	t.Helper()
	kf, err := NewJWKSKeyfunc(f.mwCtx, Config{JWKSURL: f.jwksURL, Audience: f.audience, Issuer: f.issuer})
	if err != nil {
		t.Fatal(err)
	}
	mw, err := AuthMiddleware(kf, Config{JWKSURL: f.jwksURL, Audience: f.audience, Issuer: f.issuer}, nil)
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	q := e.Group("/query", mw)
	q.POST("", func(c *echo.Context) error {
		u := UserFrom(c.Request().Context())
		if u == nil {
			return c.String(http.StatusOK, "anon")
		}
		return c.String(http.StatusOK, "authed:"+u.Sub)
	})
	return e
}

func send(e *echo.Echo, authz string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/query", nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func assert401(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != `Bearer realm="api"` {
		t.Fatalf("expected WWW-Authenticate Bearer realm=\"api\", got %q", got)
	}
}

func assert200(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != want {
		t.Fatalf("expected body %q, got %q", want, got)
	}
}

func validClaims(f *testFixture, sub string) jwt.MapClaims {
	return jwt.MapClaims{
		"sub": sub, "aud": f.audience, "iss": f.issuer,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
}

func TestMiddleware_AnonymousPassthrough(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	assert200(t, send(f.newEcho(t), ""), "anon")
}

func TestMiddleware_ValidJWT(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	tok := f.signJWT(t, jwt.MapClaims{
		"sub": "uuid-42", "email": "u@example.test", "role": "authenticated",
		"aud": f.audience, "iss": f.issuer,
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodES256, nil, "")
	assert200(t, send(f.newEcho(t), "Bearer "+tok), "authed:uuid-42")
}

func TestMiddleware_Expired(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	tok := f.signJWT(t, jwt.MapClaims{
		"sub": "x", "aud": f.audience, "iss": f.issuer,
		"exp": time.Now().Add(-time.Hour).Unix(),
	}, jwt.SigningMethodES256, nil, "")
	assert401(t, send(f.newEcho(t), "Bearer "+tok))
}

func TestMiddleware_Tampered(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tok := f.signJWT(t, validClaims(f, "x"), jwt.SigningMethodES256, other, "")
	assert401(t, send(f.newEcho(t), "Bearer "+tok))
}

func TestMiddleware_BadScheme(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	assert401(t, send(f.newEcho(t), "foo.bar.baz"))
}

func TestMiddleware_EmptyBearer(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	assert401(t, send(f.newEcho(t), "Bearer "))
}

func TestMiddleware_WrongAudience(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	tok := f.signJWT(t, jwt.MapClaims{
		"sub": "x", "aud": "wrong", "iss": f.issuer,
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodES256, nil, "")
	assert401(t, send(f.newEcho(t), "Bearer "+tok))
}

func TestMiddleware_WrongIssuer(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	tok := f.signJWT(t, jwt.MapClaims{
		"sub": "x", "aud": f.audience, "iss": "wrong",
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodES256, nil, "")
	assert401(t, send(f.newEcho(t), "Bearer "+tok))
}

func TestMiddleware_AlgHS256_Rejected(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	tok := f.signJWT(t, validClaims(f, "x"), jwt.SigningMethodHS256, []byte("secret"), "")
	assert401(t, send(f.newEcho(t), "Bearer "+tok))
}

func TestMiddleware_AlgNone_Rejected(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	// jwt/v5 requires the UnsafeAllowNoneSignatureType sentinel as the key for SigningMethodNone.
	tok := f.signJWT(t, validClaims(f, "x"), jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, "")
	assert401(t, send(f.newEcho(t), "Bearer "+tok))
}

func TestMiddleware_MissingExp(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	tok := f.signJWT(t, jwt.MapClaims{
		"sub": "x", "aud": f.audience, "iss": f.issuer,
	}, jwt.SigningMethodES256, nil, "")
	assert401(t, send(f.newEcho(t), "Bearer "+tok))
}

func TestMiddleware_ClockSkewWithinLeeway(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	tok := f.signJWT(t, jwt.MapClaims{
		"sub": "uuid-skew", "aud": f.audience, "iss": f.issuer,
		"exp": time.Now().Add(-10 * time.Second).Unix(),
	}, jwt.SigningMethodES256, nil, "")
	assert200(t, send(f.newEcho(t), "Bearer "+tok), "authed:uuid-skew")
}

func TestMiddleware_LowercaseBearer(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	tok := f.signJWT(t, validClaims(f, "uuid-lc"), jwt.SigningMethodES256, nil, "")
	assert200(t, send(f.newEcho(t), "bearer "+tok), "authed:uuid-lc")
}

func TestMiddleware_WrongKid(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	tok := f.signJWT(t, validClaims(f, "x"), jwt.SigningMethodES256, nil, "nonexistent-kid")
	assert401(t, send(f.newEcho(t), "Bearer "+tok))
}

func TestAuthMiddleware_RejectsEmptyConfig(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	kf, err := NewJWKSKeyfunc(f.mwCtx, Config{
		JWKSURL: f.jwksURL, Audience: f.audience, Issuer: f.issuer,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := AuthMiddleware(kf, Config{}, nil); err == nil {
		t.Fatal("expected error from empty Config")
	}
}

func TestRejectAttachesErrorChainAttribute(t *testing.T) {
	// Not parallel: mutates the global slog default.
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	buf := &bytes.Buffer{}
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	// Send a malformed Authorization header (non-Bearer scheme) so that
	// extractBearer returns an eris error and reject() is called, which emits
	// a structured WARN log containing the error_chain attribute.
	f := newFixture(t)
	rec := send(f.newEcho(t), "Token not-a-bearer-token")

	// Confirm the request was rejected with 401 before inspecting logs.
	assert401(t, rec)

	var logRec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logRec); err != nil {
		t.Fatalf("decode log record: %v (raw: %s)", err, buf.String())
	}

	if logRec["level"] != "WARN" {
		t.Errorf("expected level=WARN, got %v", logRec["level"])
	}
	if logRec["msg"] != "auth: token rejected" {
		t.Errorf("expected msg='auth: token rejected', got %v", logRec["msg"])
	}

	chain, ok := logRec["error_chain"]
	if !ok {
		t.Fatalf("error_chain attribute missing: %v", logRec)
	}
	if _, ok := chain.(map[string]any); !ok {
		t.Fatalf("expected error_chain to be a JSON object, got %T", chain)
	}
}

// TestMiddleware_EmailVerifiedPropagated verifies that the email_verified JWT
// claim is correctly propagated into AuthUser.EmailVerified for the three
// possible cases: present and true, present and false, and absent (zero value).
func TestMiddleware_EmailVerifiedPropagated(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		claimsEmailVerif  any // the value to set in JWT claims; nil means omit the field
		wantEmailVerified bool
	}{
		{
			name:              "present and true",
			claimsEmailVerif:  true,
			wantEmailVerified: true,
		},
		{
			name:              "present and false",
			claimsEmailVerif:  false,
			wantEmailVerified: false,
		},
		{
			name:              "absent (zero value)",
			claimsEmailVerif:  nil,
			wantEmailVerified: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)

			// Build an Echo instance whose handler captures AuthUser.EmailVerified
			// and encodes it into the response body so the test can assert on it.
			kf, err := NewJWKSKeyfunc(f.mwCtx, Config{
				JWKSURL:  f.jwksURL,
				Audience: f.audience,
				Issuer:   f.issuer,
			})
			if err != nil {
				t.Fatal(err)
			}
			mw, err := AuthMiddleware(kf, Config{
				JWKSURL:  f.jwksURL,
				Audience: f.audience,
				Issuer:   f.issuer,
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			e := echo.New()
			q := e.Group("/query", mw)
			q.POST("", func(c *echo.Context) error {
				u := UserFrom(c.Request().Context())
				if u == nil {
					return c.String(http.StatusOK, "anon")
				}
				return c.String(http.StatusOK, fmt.Sprintf("verified:%v", u.EmailVerified))
			})

			claims := jwt.MapClaims{
				"sub": "uuid-ev-test",
				"aud": f.audience,
				"iss": f.issuer,
				"exp": time.Now().Add(time.Hour).Unix(),
			}
			if tc.claimsEmailVerif != nil {
				claims["email_verified"] = tc.claimsEmailVerif
			}
			tok := f.signJWT(t, claims, jwt.SigningMethodES256, nil, "")

			wantBody := fmt.Sprintf("verified:%v", tc.wantEmailVerified)
			assert200(t, send(e, "Bearer "+tok), wantBody)
		})
	}
}

// ---------------------------------------------------------------------------
// Stubs for TouchLastActive middleware tests
// ---------------------------------------------------------------------------

// panicUserRepo is a panic-base stub for repository.UserRepository. Every
// method panics so tests that embed it only implement what they exercise.
type panicUserRepo struct{}

func (panicUserRepo) FindByID(_ context.Context, _ string) (*domain.User, error) {
	panic("panicUserRepo: FindByID not expected")
}
func (panicUserRepo) FindByIDs(_ context.Context, _ []string) (map[string]*domain.User, error) {
	panic("panicUserRepo: FindByIDs not expected")
}
func (panicUserRepo) Update(_ context.Context, _ string, _ repository.UserUpdate) (*domain.User, error) {
	panic("panicUserRepo: Update not expected")
}
func (panicUserRepo) SetLastViewedCardgroup(_ context.Context, _, _ string) error {
	panic("panicUserRepo: SetLastViewedCardgroup not expected")
}
func (panicUserRepo) ListPage(
	_ context.Context, _, _ *string, _, _ int, _ *string, _ *string,
) ([]*domain.User, int64, error) {
	panic("panicUserRepo: ListPage not expected")
}
func (panicUserRepo) TouchLastActive(_ context.Context, _ string) error {
	panic("panicUserRepo: TouchLastActive not expected")
}

// blockingUserRepo wraps panicUserRepo and provides a TouchLastActive that
// blocks on a channel until released by the test. After release it records
// the user ID it was called with.
type blockingUserRepo struct {
	panicUserRepo
	release chan struct{}
	called  chan string // receives the userID when the goroutine is unblocked
}

func newBlockingUserRepo() *blockingUserRepo {
	return &blockingUserRepo{
		release: make(chan struct{}),
		called:  make(chan string, 1),
	}
}

func (r *blockingUserRepo) TouchLastActive(_ context.Context, userID string) error {
	<-r.release // block until the test signals
	r.called <- userID
	return nil
}

// errorUserRepo wraps panicUserRepo and returns a pre-canned error from
// TouchLastActive.
type errorUserRepo struct {
	panicUserRepo
	err error
}

func (r *errorUserRepo) TouchLastActive(_ context.Context, _ string) error {
	return r.err
}

// chanLogHandler is a thread-safe slog.Handler that sends each log record to
// a channel. Tests can receive from the channel to wait for the goroutine's
// log write without racing on a shared bytes.Buffer.
type chanLogHandler struct {
	ch      chan map[string]any
	wrapped slog.Handler // underlying handler for actual formatting
}

func newChanLogHandler() (*chanLogHandler, chan map[string]any) {
	ch := make(chan map[string]any, 16)
	h := &chanLogHandler{
		ch:      ch,
		wrapped: slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug}),
	}
	return h, ch
}

func (h *chanLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.wrapped.Enabled(ctx, level)
}

func (h *chanLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &chanLogHandler{ch: h.ch, wrapped: h.wrapped.WithAttrs(attrs)}
}

func (h *chanLogHandler) WithGroup(name string) slog.Handler {
	return &chanLogHandler{ch: h.ch, wrapped: h.wrapped.WithGroup(name)}
}

func (h *chanLogHandler) Handle(ctx context.Context, r slog.Record) error {
	rec := map[string]any{
		"level": r.Level.String(),
		"msg":   r.Message,
	}
	r.Attrs(func(a slog.Attr) bool {
		// Resolve any nested values.
		v := a.Value.Resolve()
		switch v.Kind() {
		case slog.KindAny:
			rec[a.Key] = v.Any()
		case slog.KindString:
			rec[a.Key] = v.String()
		case slog.KindBool:
			rec[a.Key] = v.Bool()
		case slog.KindInt64:
			rec[a.Key] = v.Int64()
		case slog.KindFloat64:
			rec[a.Key] = v.Float64()
		default:
			rec[a.Key] = v.String()
		}
		return true
	})
	h.ch <- rec
	return nil
}

// ---------------------------------------------------------------------------
// TouchLastActive behaviour tests
// ---------------------------------------------------------------------------

// buildEchoWithRepo constructs an Echo instance whose /query route is guarded
// by AuthMiddleware wired with the given userRepo stub.
func buildEchoWithRepo(t *testing.T, f *testFixture, userRepo repository.UserRepository) *echo.Echo {
	t.Helper()
	kf, err := NewJWKSKeyfunc(f.mwCtx, Config{
		JWKSURL:  f.jwksURL,
		Audience: f.audience,
		Issuer:   f.issuer,
	})
	if err != nil {
		t.Fatal(err)
	}
	mw, err := AuthMiddleware(kf, Config{
		JWKSURL:  f.jwksURL,
		Audience: f.audience,
		Issuer:   f.issuer,
	}, userRepo)
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	q := e.Group("/query", mw)
	q.POST("", func(c *echo.Context) error {
		u := UserFrom(c.Request().Context())
		if u == nil {
			return c.String(http.StatusOK, "anon")
		}
		return c.String(http.StatusOK, "authed:"+u.Sub)
	})
	return e
}

// TestAuthMiddleware_TouchLastActive_FiresAsync_NoBlock verifies that the
// fire-and-forget goroutine does not block the HTTP response. The stub's
// TouchLastActive blocks on a channel; the test asserts the HTTP response
// arrives before the channel is signalled, then confirms the stub was called
// with the correct user ID once the channel is released.
func TestAuthMiddleware_TouchLastActive_FiresAsync_NoBlock(t *testing.T) {
	// Not parallel: uses a blocking channel; parallel execution with shared
	// global slog state could interfere with adjacent tests that capture logs.
	f := newFixture(t)

	stub := newBlockingUserRepo()
	e := buildEchoWithRepo(t, f, stub)

	const wantSub = "uuid-async"
	tok := f.signJWT(t, jwt.MapClaims{
		"sub": wantSub, "aud": f.audience, "iss": f.issuer,
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodES256, nil, "")

	// The HTTP call must complete while TouchLastActive is still blocked.
	rec := send(e, "Bearer "+tok)
	assert200(t, rec, "authed:"+wantSub)

	// Confirm the goroutine has not yet sent (i.e. it is still blocking).
	select {
	case id := <-stub.called:
		t.Fatalf("TouchLastActive completed before channel was released: userID=%q", id)
	default:
		// Good: goroutine is still blocked.
	}

	// Release the goroutine and wait for confirmation.
	close(stub.release)
	select {
	case gotID := <-stub.called:
		if gotID != wantSub {
			t.Fatalf("TouchLastActive called with userID=%q, want %q", gotID, wantSub)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("TouchLastActive goroutine did not complete within 3 seconds after release")
	}
}

// TestAuthMiddleware_TouchLastActive_OnError_LogsWarn_NoPII verifies that
// when the userRepo.TouchLastActive returns an error, the middleware:
//   - emits a WARN-level log line,
//   - attaches a rich error_chain (root.stack present),
//   - includes user_id in the log,
//   - does NOT include email, display_name, or bio (PII protection).
//
// The request still returns 200 — the failed write must not block the handler.
// The test uses a channel-backed slog handler so reading log records never
// races against the goroutine that writes them.
func TestAuthMiddleware_TouchLastActive_OnError_LogsWarn_NoPII(t *testing.T) {
	// Not parallel: mutates the global slog default.
	handler, logCh := newChanLogHandler()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(handler))

	f := newFixture(t)

	const wantSub = "uuid-warn-test"
	stub := &errorUserRepo{err: eris.New("db: connection refused")}
	e := buildEchoWithRepo(t, f, stub)

	tok := f.signJWT(t, jwt.MapClaims{
		"sub": wantSub, "email": "secret@example.com",
		"aud": f.audience, "iss": f.issuer,
		"exp": time.Now().Add(time.Hour).Unix(),
	}, jwt.SigningMethodES256, nil, "")

	rec := send(e, "Bearer "+tok)
	assert200(t, rec, "authed:"+wantSub)

	// Drain channel records until the expected log line arrives or timeout.
	var warnRec map[string]any
	deadline := time.After(3 * time.Second)
drain:
	for {
		select {
		case r := <-logCh:
			if r["msg"] == "auth: last_active update failed" {
				warnRec = r
				break drain
			}
		case <-deadline:
			t.Fatalf("timed out waiting for WARN log 'auth: last_active update failed'")
		}
	}

	if warnRec["level"] != "WARN" {
		t.Errorf("expected level=WARN, got %v", warnRec["level"])
	}
	if warnRec["user_id"] != wantSub {
		t.Errorf("expected user_id=%q in WARN log, got %v", wantSub, warnRec["user_id"])
	}

	// PII must not appear.
	for _, field := range []string{"email", "display_name", "bio"} {
		if _, has := warnRec[field]; has {
			t.Errorf("WARN log must not contain %q field (PII protection)", field)
		}
	}

	// error_chain must be the rich root.stack shape.
	// Note: error_chain is captured directly from slog.Attr (no JSON round-trip),
	// so eris.ToJSON's []string root.stack stays as []string — not []any.
	chain, ok := warnRec["error_chain"].(map[string]any)
	if !ok {
		t.Fatalf("error_chain is not a JSON object: %T", warnRec["error_chain"])
	}
	root, hasRoot := chain["root"].(map[string]any)
	if !hasRoot {
		t.Error("error_chain must have root entry (got external-only shape; stub may be using stdlib errors)")
	}
	if root != nil {
		// eris.ToJSON emits root.stack as []string (Go native type, no JSON round-trip).
		if stack, _ := root["stack"].([]string); len(stack) == 0 {
			t.Error("error_chain.root.stack must contain at least one frame")
		}
	}
}
