package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"backend/graph/resolver"
	"backend/internal/auth"
	"backend/internal/database"
	"backend/internal/repository"
	"backend/internal/usecase"
)

var testDBURL string

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx,
		"postgres:15-alpine",
		tcpostgres.WithDatabase("flamingo_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		// BasicWaitStrategies matches the pattern used by database/repository
		// tests: the default wait is racy against postgres's init-time restart.
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcpostgres run: %v\n", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			fmt.Fprintf(os.Stderr, "terminate container: %v\n", err)
		}
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "conn string: %v\n", err)
		return 1
	}
	if err := bootstrapAuthSchema(ctx, dsn); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
		return 1
	}
	if err := database.Migrate(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		return 1
	}
	testDBURL = dsn
	return m.Run()
}

// bootstrapAuthSchema mimics the Supabase-managed auth.users table just enough
// for FK and trigger references in our migrations to resolve.
func bootstrapAuthSchema(ctx context.Context, dsn string) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `
        CREATE SCHEMA IF NOT EXISTS auth;
        CREATE TABLE IF NOT EXISTS auth.users (
            id uuid PRIMARY KEY,
            email text
        );
    `)
	return err
}

func noopAuthMW(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error { return next(c) }
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(newRouter(&resolver.Resolver{}, noopAuthMW, nil))
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
	t.Setenv("SUPABASE_DB_URL", testDBURL)

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
	t.Setenv("SUPABASE_DB_URL", testDBURL)

	err := run(context.Background(), slog.New(slog.DiscardHandler))
	if err == nil {
		t.Fatal("expected run to fail when SUPABASE_JWKS_URL is empty")
	}
	if !strings.Contains(err.Error(), "SUPABASE_JWKS_URL") && !strings.Contains(err.Error(), "JWKS") {
		t.Fatalf("expected error to mention JWKS or SUPABASE_JWKS_URL, got: %v", err)
	}
}

func TestRun_FailsWhenDBURLMissing(t *testing.T) {
	tsJWKS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys": []}`))
	}))
	defer tsJWKS.Close()

	t.Setenv("SUPABASE_JWKS_URL", tsJWKS.URL)
	t.Setenv("SUPABASE_JWT_AUDIENCE", "authenticated")
	t.Setenv("SUPABASE_JWT_ISSUER", "http://issuer.test")
	t.Setenv("SUPABASE_DB_URL", "")

	err := run(context.Background(), slog.New(slog.DiscardHandler))
	if err == nil {
		t.Fatal("expected error when SUPABASE_DB_URL is empty, got nil")
	}
	if !strings.Contains(err.Error(), "SUPABASE_DB_URL") {
		t.Fatalf("expected error to mention SUPABASE_DB_URL, got: %v", err)
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

// jwtFixture bundles an ECDSA signing key plus a JWKS endpoint that exposes its
// public key, so integration tests can mint Supabase-shaped JWTs that the real
// auth.AuthMiddleware will accept.
type jwtFixture struct {
	priv     *ecdsa.PrivateKey
	kid      string
	jwksURL  string
	audience string
	issuer   string
}

func newJWTFixture(t *testing.T) *jwtFixture {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
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
	return &jwtFixture{
		priv: priv, kid: kid, jwksURL: ts.URL,
		audience: "authenticated", issuer: "http://issuer.test",
	}
}

func (f *jwtFixture) sign(t *testing.T, sub string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": sub,
		"aud": f.audience,
		"iss": f.issuer,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = f.kid
	signed, err := tok.SignedString(f.priv)
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signed
}

// newGraphQLTestServer builds a real router (auth middleware + GraphQL) backed
// by the testcontainer Postgres. It returns the running httptest.Server plus
// the opened DB so callers can insert auth.users rows directly.
func newGraphQLTestServer(t *testing.T, f *jwtFixture) (*httptest.Server, *database.DB) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := auth.Config{JWKSURL: f.jwksURL, Audience: f.audience, Issuer: f.issuer}
	kf, err := auth.NewJWKSKeyfunc(ctx, cfg)
	if err != nil {
		t.Fatalf("jwks keyfunc: %v", err)
	}
	mw, err := auth.AuthMiddleware(kf, cfg)
	if err != nil {
		t.Fatalf("auth middleware: %v", err)
	}

	db, err := database.Open(ctx, database.Config{URL: testDBURL})
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	t.Cleanup(db.Close)

	profileRepo := repository.NewProfileRepository(db.GORM)
	profileUC := usecase.NewProfileUsecase(profileRepo)
	e := newRouter(&resolver.Resolver{Profile: profileUC}, mw, profileRepo)

	ts := httptest.NewServer(e)
	t.Cleanup(ts.Close)
	return ts, db
}

// insertAuthUser inserts a row into auth.users so the handle_new_user trigger
// creates the matching public.profiles row. Returns the generated user id.
func insertAuthUser(t *testing.T, ctx context.Context) string {
	t.Helper()
	pool, err := pgxpool.New(ctx, testDBURL)
	if err != nil {
		t.Fatalf("pgxpool new: %v", err)
	}
	defer pool.Close()

	id := uuid.NewString()
	email := "user-" + id[:8] + "@test"
	if _, err := pool.Exec(ctx, `INSERT INTO auth.users(id, email) VALUES ($1, $2)`, id, email); err != nil {
		t.Fatalf("insert auth.users: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM public.profiles WHERE id = $1`, id).Scan(&count); err != nil {
		t.Fatalf("verify profile row: %v", err)
	}
	if count == 0 {
		t.Fatalf("handle_new_user trigger did not create profile for %s", id)
	}
	return id
}

// postGraphQL sends a GraphQL POST and decodes the envelope.
func postGraphQL(t *testing.T, url, body, bearer string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode body (status=%d): %v body=%q", res.StatusCode, err, raw)
	}
	return out
}

func TestGraphQL_Me_Anonymous(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)

	resp := postGraphQL(t, ts.URL+"/query", `{"query":"{ me { id } }"}`, "")

	errs, ok := resp["errors"].([]any)
	if !ok || len(errs) == 0 {
		t.Fatalf("expected errors, got %v", resp)
	}
	ext, _ := errs[0].(map[string]any)["extensions"].(map[string]any)
	code, _ := ext["code"].(string)
	if code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q (resp=%v)", code, resp)
	}
}

func TestGraphQL_Me_Authenticated(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)

	userID := insertAuthUser(t, context.Background())
	tok := f.sign(t, userID)

	resp := postGraphQL(t, ts.URL+"/query",
		`{"query":"{ me { id displayName bio avatarUrl } }"}`, tok)

	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	me, _ := data["me"].(map[string]any)
	if me == nil {
		t.Fatalf("expected data.me, got nil; resp=%v", resp)
	}
	if me["id"] != userID {
		t.Fatalf("expected me.id=%q, got %v", userID, me["id"])
	}
}

func TestGraphQL_UpdateProfile_Authenticated(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)

	userID := insertAuthUser(t, context.Background())
	tok := f.sign(t, userID)

	mutation := `{"query":"mutation { updateProfile(input: { displayName: \"Alice\", bio: \"hi\" }) { user { id displayName bio } } }"}`
	resp := postGraphQL(t, ts.URL+"/query", mutation, tok)

	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("unexpected errors on mutation: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	upd, _ := data["updateProfile"].(map[string]any)
	user, _ := upd["user"].(map[string]any)
	if user == nil {
		t.Fatalf("expected updateProfile.user, got nil; resp=%v", resp)
	}
	if user["id"] != userID {
		t.Fatalf("expected user.id=%q, got %v", userID, user["id"])
	}
	if user["displayName"] != "Alice" {
		t.Fatalf("expected displayName=Alice, got %v", user["displayName"])
	}
	if user["bio"] != "hi" {
		t.Fatalf("expected bio=hi, got %v", user["bio"])
	}

	// Re-query me to verify persistence.
	meResp := postGraphQL(t, ts.URL+"/query",
		`{"query":"{ me { id displayName bio } }"}`, tok)
	if errs, ok := meResp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("unexpected errors on me re-query: %v", errs)
	}
	meData, _ := meResp["data"].(map[string]any)
	me, _ := meData["me"].(map[string]any)
	if me == nil {
		t.Fatalf("expected data.me on re-query, got nil; resp=%v", meResp)
	}
	if me["displayName"] != "Alice" {
		t.Fatalf("expected persisted displayName=Alice, got %v", me["displayName"])
	}
	if me["bio"] != "hi" {
		t.Fatalf("expected persisted bio=hi, got %v", me["bio"])
	}
}

// build101ComplexityQuery returns a GraphQL query body whose complexity exceeds
// 100. Each alias of "me { id displayName bio avatarUrl }" costs 5 points
// (1 for me + 4 scalar fields). 21 aliases = 105 points > limit of 100.
func build101ComplexityQuery() string {
	var sb strings.Builder
	sb.WriteString(`{"query":"query {`)
	for i := 1; i <= 21; i++ {
		fmt.Fprintf(&sb, " a%d: me { id displayName bio avatarUrl }", i)
	}
	sb.WriteString(` }"}`)
	return sb.String()
}

// postRaw POSTs body to url and returns the raw response bytes.
func postRaw(t *testing.T, url, body string) []byte {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return raw
}

func TestComplexityLimit_Rejects(t *testing.T) {
	ts := newTestServer(t)

	raw := postRaw(t, ts.URL+"/query", build101ComplexityQuery())

	var payload struct {
		Errors []struct {
			Message    string         `json:"message"`
			Extensions map[string]any `json:"extensions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode body: %v (body=%q)", err, raw)
	}
	if len(payload.Errors) == 0 {
		t.Fatalf("expected complexity errors, got none; body=%q", raw)
	}
	code, _ := payload.Errors[0].Extensions["code"].(string)
	msg := payload.Errors[0].Message
	if code != "COMPLEXITY_LIMIT_EXCEEDED" && !strings.Contains(strings.ToLower(msg), "complexity") {
		t.Fatalf("expected complexity error, got code=%q message=%q", code, msg)
	}
}

func newIntrospectionTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(newGraphQLServer(&resolver.Resolver{}))
	t.Cleanup(ts.Close)
	return ts
}

const introspectionQuery = `{"query":"{ __schema { queryType { name } } }"}`

func TestIntrospection_GatedOff(t *testing.T) {
	t.Setenv("GRAPHQL_INTROSPECTION", "off")
	ts := newIntrospectionTestServer(t)

	raw := postRaw(t, ts.URL, introspectionQuery)

	var payload struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode body: %v (body=%q)", err, raw)
	}
	if len(payload.Errors) == 0 {
		t.Fatalf("expected introspection error, got none; body=%q", raw)
	}
	if !strings.Contains(strings.ToLower(payload.Errors[0].Message), "introspection") {
		t.Fatalf("expected introspection error message, got %q", payload.Errors[0].Message)
	}
}

func TestIntrospection_DefaultOn(t *testing.T) {
	t.Setenv("GRAPHQL_INTROSPECTION", "")
	ts := newIntrospectionTestServer(t)

	raw := postRaw(t, ts.URL, introspectionQuery)

	var payload struct {
		Data   map[string]any `json:"data"`
		Errors []any          `json:"errors"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode body: %v (body=%q)", err, raw)
	}
	if len(payload.Errors) > 0 {
		t.Fatalf("unexpected errors: %v; body=%q", payload.Errors, raw)
	}
	if payload.Data["__schema"] == nil {
		t.Fatalf("expected data.__schema to be non-nil; body=%q", raw)
	}
}

func TestLoader_Middleware_DoesNotBreakQuery(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)

	userID := insertAuthUser(t, context.Background())
	tok := f.sign(t, userID)

	resp := postGraphQL(t, ts.URL+"/query", `{"query":"{ me { id } }"}`, tok)

	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("unexpected errors (loader middleware may have broken request): %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	me, _ := data["me"].(map[string]any)
	if me == nil {
		t.Fatalf("expected data.me, got nil; resp=%v", resp)
	}
	if me["id"] != userID {
		t.Fatalf("expected me.id=%q, got %v", userID, me["id"])
	}
}
