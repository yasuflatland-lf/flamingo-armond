package auth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
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
	mw, err := AuthMiddleware(kf, Config{JWKSURL: f.jwksURL, Audience: f.audience, Issuer: f.issuer})
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

	if _, err := AuthMiddleware(kf, Config{}); err == nil {
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
