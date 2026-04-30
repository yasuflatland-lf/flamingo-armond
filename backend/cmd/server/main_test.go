package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gorm.io/gorm"

	"backend/graph/resolver"
	"backend/internal/auth"
	"backend/internal/database"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/handler/ping"
	"backend/internal/repository"
	"backend/internal/telemetry"
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

// bootstrapAuthSchema mimics the Supabase-managed auth schema and roles just
// enough for FK, trigger, and RLS policy references in migrations to resolve.
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
        DO $$
        BEGIN
            IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
                CREATE ROLE authenticated LOGIN PASSWORD 'test';
            END IF;
        END
        $$;
        CREATE SCHEMA IF NOT EXISTS auth;
        CREATE TABLE IF NOT EXISTS auth.users (
            id uuid PRIMARY KEY,
            email text
        );
        CREATE OR REPLACE FUNCTION auth.uid()
        RETURNS uuid
        LANGUAGE sql
        STABLE
        AS $$
            SELECT COALESCE(
                NULLIF(current_setting('request.jwt.claim.sub', true), ''),
                NULLIF(current_setting('request.jwt.claims', true), '')::jsonb ->> 'sub'
            )::uuid
        $$;
        GRANT USAGE ON SCHEMA auth TO authenticated;
        GRANT EXECUTE ON FUNCTION auth.uid() TO authenticated;
        GRANT USAGE ON SCHEMA public TO authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public
            GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public
            GRANT USAGE, SELECT ON SEQUENCES TO authenticated;
    `)
	return err
}

func noopAuthMW(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error { return next(c) }
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(newRouter(&resolver.Resolver{}, noopAuthMW, nil, nil, nil, nil, ping.New(nil, "test-token")))
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
	t.Setenv("PING_TOKEN", "test-token")

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
	t.Setenv("PING_TOKEN", "test-token")

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
	t.Setenv("PING_TOKEN", "test-token")

	err := run(context.Background(), slog.New(slog.DiscardHandler))
	if err == nil {
		t.Fatal("expected error when SUPABASE_DB_URL is empty, got nil")
	}
	if !strings.Contains(err.Error(), "SUPABASE_DB_URL") {
		t.Fatalf("expected error to mention SUPABASE_DB_URL, got: %v", err)
	}
}

func TestRun_FailsWhenPINGTokenMissing(t *testing.T) {
	tsJWKS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys": []}`))
	}))
	defer tsJWKS.Close()

	t.Setenv("SUPABASE_JWKS_URL", tsJWKS.URL)
	t.Setenv("SUPABASE_JWT_AUDIENCE", "authenticated")
	t.Setenv("SUPABASE_JWT_ISSUER", "http://issuer.test")
	t.Setenv("SUPABASE_DB_URL", testDBURL)
	t.Setenv("PING_TOKEN", "")

	err := run(context.Background(), slog.New(slog.DiscardHandler))
	if err == nil {
		t.Fatal("expected error when PING_TOKEN is empty, got nil")
	}
	if !strings.Contains(err.Error(), "PING_TOKEN") {
		t.Fatalf("expected error to mention PING_TOKEN, got: %v", err)
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
	return newGraphQLTestServerWithUserRepo(t, f, nil)
}

// newGraphQLTestServerWithUserRepo builds the same chain as newGraphQLTestServer
// but lets the caller swap the User repository (for instrumented test doubles).
// A nil userRepo means "use the default GORM-backed repository".
func newGraphQLTestServerWithUserRepo(t *testing.T, f *jwtFixture, userRepo repository.UserRepository) (*httptest.Server, *database.DB) {
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

	if userRepo == nil {
		userRepo = repository.NewUserRepository(db.GORM)
	}
	roleRepo := repository.NewRoleRepository(db.GORM)
	cardgroupRepo := repository.NewCardgroupRepository(db.GORM)
	cardRepo := repository.NewCardRepository(db.GORM)
	swipeRecordRepo := repository.NewSwipeRecordRepository(db.GORM)
	userUC := usecase.NewUserUsecase(userRepo)
	cardgroupUC := usecase.NewCardgroupUsecase(cardgroupRepo)
	cardUC := usecase.NewCardUsecase(db.GORM, cardRepo, cardgroupRepo)
	swipeUC := usecase.NewSwipeUsecase(db.GORM, cardRepo, cardgroupRepo, swipeRecordRepo, service.NewFSRSScheduler(), 10)
	pingRecordRepo := repository.NewPingRecordRepository(db.GORM)
	e := newRouter(&resolver.Resolver{User: userUC, CardgroupUC: cardgroupUC, CardUC: cardUC, SwipeUC: swipeUC}, mw, userRepo, roleRepo, cardgroupRepo, cardRepo, ping.New(pingRecordRepo, "test-token"), swipeRecordRepo)

	ts := httptest.NewServer(e)
	t.Cleanup(ts.Close)
	return ts, db
}

// countingUserRepo wraps a real UserRepository and counts calls to each method.
// It is used by TestGraphQL_Cardgroup_OwnerLoader_NoNPlus1 to assert that the
// DataLoader batches all owner look-ups into a single FindByIDs call rather
// than issuing one FindByID per cardgroup.
type countingUserRepo struct {
	inner        repository.UserRepository
	findByID     atomic.Int32
	findByIDs    atomic.Int32
	mu           sync.Mutex
	receivedKeys [][]string
}

func (c *countingUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	c.findByID.Add(1)
	return c.inner.FindByID(ctx, id)
}

func (c *countingUserRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.User, error) {
	c.findByIDs.Add(1)
	c.mu.Lock()
	c.receivedKeys = append(c.receivedKeys, append([]string(nil), ids...))
	c.mu.Unlock()
	return c.inner.FindByIDs(ctx, ids)
}

func (c *countingUserRepo) Update(ctx context.Context, id string, patch repository.UserUpdate) (*domain.User, error) {
	return c.inner.Update(ctx, id, patch)
}

// insertAuthUser inserts a row into auth.users so the handle_new_user trigger
// creates the matching public.users row. Returns the generated user id.
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
		`SELECT count(*) FROM public.users WHERE id = $1`, id).Scan(&count); err != nil {
		t.Fatalf("verify user row: %v", err)
	}
	if count == 0 {
		t.Fatalf("handle_new_user trigger did not create user for %s", id)
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

// installInMemoryTracer wires a fresh in-memory exporter as the global
// TracerProvider so individual tests can inspect emitted spans.
// The returned flush function must be called before reading spans.
func installInMemoryTracer(t *testing.T) (*tracetest.InMemoryExporter, func()) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	shutdown, err := telemetry.InitWithExporter(
		context.Background(),
		slog.New(slog.DiscardHandler),
		exp,
	)
	if err != nil {
		t.Fatalf("InitWithExporter: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	flush := func() {
		tp, ok := otel.GetTracerProvider().(interface {
			ForceFlush(context.Context) error
		})
		if ok {
			_ = tp.ForceFlush(context.Background())
		}
	}
	return exp, flush
}

func TestGraphQL_Me_EmitsSpans(t *testing.T) {
	exp, flush := installInMemoryTracer(t)

	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)

	userID := insertAuthUser(t, context.Background())
	tok := f.sign(t, userID)

	resp := postGraphQL(t, ts.URL+"/query",
		`{"query":"query Me { me { id displayName bio avatarUrl } }"}`, tok)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	flush()

	spans := exp.GetSpans()
	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.Name)
	}
	t.Logf("emitted span names: %v", names)
	if len(spans) < 2 {
		t.Fatalf("expected >=2 spans (operation + resolver), got %d: names=%v", len(spans), names)
	}
	// otelgqlgen v0.17 emits the operation span as the operation name (e.g. "Me").
	// Accept either the bare operation name or the "query Me" form so the test
	// survives a future library bump.
	var sawOperation bool
	for _, n := range names {
		if n == "Me" || n == "query Me" || strings.Contains(n, "Me") {
			sawOperation = true
			break
		}
	}
	if !sawOperation {
		t.Errorf("expected an operation span referencing 'Me', got names=%v", names)
	}

	sawField := false
	for _, name := range names {
		if strings.HasPrefix(name, "User/") {
			sawField = true
			break
		}
	}
	if !sawField {
		t.Errorf("expected at least one User/<field> field-level span, got: %v", names)
	}
}

func TestGraphQL_APQ_HashOnly_Roundtrip(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)

	userID := insertAuthUser(t, context.Background())
	tok := f.sign(t, userID)

	const query = `query Me { me { id displayName } }`
	hash := sha256Hex(query)

	// 1st POST: full query + hash to populate the APQ cache.
	firstBody := fmt.Sprintf(
		`{"query":%q,"extensions":{"persistedQuery":{"version":1,"sha256Hash":%q}}}`,
		query, hash,
	)
	resp1 := postGraphQL(t, ts.URL+"/query", firstBody, tok)
	if errs, ok := resp1["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("1st request unexpected errors: %v", errs)
	}
	data1, _ := resp1["data"].(map[string]any)
	if data1["me"] == nil {
		t.Fatalf("1st request: expected data.me, resp=%v", resp1)
	}

	// 2nd POST: hash-only (no `query` field). Should resolve from APQ cache.
	secondBody := fmt.Sprintf(
		`{"extensions":{"persistedQuery":{"version":1,"sha256Hash":%q}}}`,
		hash,
	)
	resp2 := postGraphQL(t, ts.URL+"/query", secondBody, tok)
	if errs, ok := resp2["errors"].([]any); ok && len(errs) > 0 {
		raw, _ := json.Marshal(errs)
		if strings.Contains(string(raw), "PERSISTED_QUERY_NOT_FOUND") {
			t.Fatalf("2nd request returned PERSISTED_QUERY_NOT_FOUND, APQ not working: %s", raw)
		}
		t.Fatalf("2nd request unexpected errors: %v", errs)
	}
	data2, _ := resp2["data"].(map[string]any)
	if data2["me"] == nil {
		t.Fatalf("2nd request: expected data.me, resp=%v", resp2)
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestGraphQL_PropagatesTraceparent verifies that an incoming W3C traceparent
// header is extracted by the otelhttp layer and that all emitted spans share
// the trace ID supplied by the caller.
func TestGraphQL_PropagatesTraceparent(t *testing.T) {
	exp, flush := installInMemoryTracer(t)

	// The /query group uses noopAuthMW so no real JWT is needed here.
	ts := newTestServer(t)

	const (
		traceIDHex  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		spanIDHex   = "bbbbbbbbbbbbbbbb"
		traceparent = "00-" + traceIDHex + "-" + spanIDHex + "-01"
	)

	body := bytes.NewBufferString(`{"query":"{ health }"}`)
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/query", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("traceparent", traceparent)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /query: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d, body = %q", res.StatusCode, raw)
	}

	flush()

	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span, got none")
	}

	wantTraceID, err := hex.DecodeString(traceIDHex)
	if err != nil {
		t.Fatalf("decode traceIDHex: %v", err)
	}
	var wantArr [16]byte
	copy(wantArr[:], wantTraceID)

	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.Name)
		if s.SpanContext.TraceID() != wantArr {
			t.Errorf("span %q: TraceID = %s, want %s",
				s.Name, s.SpanContext.TraceID(), traceIDHex)
		}
	}
	t.Logf("propagation test span names: %v", names)

	var sawHTTP bool
	for _, n := range names {
		if strings.Contains(n, "/query") || strings.Contains(n, "POST") || strings.Contains(n, "graphql.http") {
			sawHTTP = true
			break
		}
	}
	if !sawHTTP {
		t.Errorf("expected an HTTP-layer span (POST /query or graphql.http), got: %v", names)
	}

	var sawOperation bool
	for _, n := range names {
		if strings.Contains(n, "Health") || strings.Contains(n, "health") || strings.Contains(n, "Query") {
			sawOperation = true
			break
		}
	}
	if !sawOperation {
		t.Errorf("expected a gqlgen operation or resolver span, got: %v", names)
	}

	// Verify the parent-child relationship: at least one server-side span must
	// have its Parent pointing at the injected (remote) span. This catches the
	// regression where otelhttp ignores the inbound traceparent and silently
	// creates a new root span — the spans would all share a *new* trace ID
	// instead of rooting back to the caller's spanIDHex.
	var sawInjectedParent bool
	for _, s := range spans {
		if s.Parent.SpanID().String() == spanIDHex &&
			s.Parent.TraceID().String() == traceIDHex {
			sawInjectedParent = true
			t.Logf("span %q correctly links to injected parent %s/%s",
				s.Name, traceIDHex, spanIDHex)
			break
		}
	}
	if !sawInjectedParent {
		t.Errorf("no span had Parent.SpanID=%q / Parent.TraceID=%q; "+
			"otelhttp may not be extracting the inbound traceparent. spans: %v",
			spanIDHex, traceIDHex, names)
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

// createTestCardgroup calls the createCardgroup mutation and returns the new cardgroup id.
// It serialises the full JSON body via json.Marshal so name is always a valid JSON string.
func createTestCardgroup(t *testing.T, srvURL, bearer, name string) string {
	t.Helper()
	gqlQuery := fmt.Sprintf(`mutation { createCardgroup(input: {name: %s}) { cardgroup { id name ownerId } } }`, gqlStringLit(name))
	body, err := json.Marshal(map[string]string{"query": gqlQuery})
	if err != nil {
		t.Fatalf("json.Marshal body: %v", err)
	}
	resp := postGraphQL(t, srvURL+"/query", string(body), bearer)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("createCardgroup errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["createCardgroup"].(map[string]any)
	cg, _ := payload["cardgroup"].(map[string]any)
	if cg == nil {
		t.Fatalf("createCardgroup: expected cardgroup, got nil; resp=%v", resp)
	}
	id, _ := cg["id"].(string)
	if id == "" {
		t.Fatalf("createCardgroup: empty id; resp=%v", resp)
	}
	return id
}

func createTestCard(t *testing.T, srvURL, bearer, cardgroupID, front, back string) string {
	t.Helper()
	gqlQuery := fmt.Sprintf(`mutation {
		createCard(input: {cardgroupId: %s, front: %s, back: %s}) {
			card { id front back cardgroupId state reps lapses stability difficulty }
		}
	}`, gqlStringLit(cardgroupID), gqlStringLit(front), gqlStringLit(back))
	body, err := json.Marshal(map[string]string{"query": gqlQuery})
	if err != nil {
		t.Fatalf("json.Marshal body: %v", err)
	}
	resp := postGraphQL(t, srvURL+"/query", string(body), bearer)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("createCard errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["createCard"].(map[string]any)
	card, _ := payload["card"].(map[string]any)
	if card == nil {
		t.Fatalf("createCard: expected card, got nil; resp=%v", resp)
	}
	id, _ := card["id"].(string)
	if id == "" {
		t.Fatalf("createCard: empty id; resp=%v", resp)
	}
	if card["cardgroupId"] != cardgroupID {
		t.Fatalf("createCard cardgroupId=%v, want %q", card["cardgroupId"], cardgroupID)
	}
	if card["front"] != front || card["back"] != back {
		t.Fatalf("createCard text mismatch: %v", card)
	}
	if card["state"] != float64(0) || card["reps"] != float64(0) || card["lapses"] != float64(0) {
		t.Fatalf("createCard FSRS counters mismatch: %v", card)
	}
	return id
}

// gqlStringLit returns s as a GraphQL string literal (double-quoted, with internal
// double-quotes and backslashes escaped). UUIDs and plain ASCII names are returned as-is.
func gqlStringLit(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// gqlErrCode extracts errors[0].extensions.code from a postGraphQL response.
func gqlErrCode(resp map[string]any) string {
	errs, ok := resp["errors"].([]any)
	if !ok || len(errs) == 0 {
		return ""
	}
	ext, _ := errs[0].(map[string]any)["extensions"].(map[string]any)
	code, _ := ext["code"].(string)
	return code
}

// gqlErrField extracts errors[0].extensions.field from a postGraphQL response.
func gqlErrField(resp map[string]any) string {
	errs, ok := resp["errors"].([]any)
	if !ok || len(errs) == 0 {
		return ""
	}
	ext, _ := errs[0].(map[string]any)["extensions"].(map[string]any)
	field, _ := ext["field"].(string)
	return field
}

// gqlErrMessage extracts errors[0].message from a postGraphQL response.
func gqlErrMessage(resp map[string]any) string {
	errs, ok := resp["errors"].([]any)
	if !ok || len(errs) == 0 {
		return ""
	}
	message, _ := errs[0].(map[string]any)["message"].(string)
	return message
}

func TestGraphQL_CreateCardgroup_Then_MyCardgroups(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	cgID := createTestCardgroup(t, ts.URL, tok, "Vocab 1")

	resp := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroups { id name ownerId } }"}`, tok)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("myCardgroups errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	list, _ := data["myCardgroups"].([]any)
	if len(list) != 1 {
		t.Fatalf("expected 1 cardgroup, got %d; resp=%v", len(list), resp)
	}
	cg, _ := list[0].(map[string]any)
	if cg["id"] != cgID {
		t.Fatalf("expected id=%q, got %v", cgID, cg["id"])
	}
	if cg["name"] != "Vocab 1" {
		t.Fatalf("expected name=Vocab 1, got %v", cg["name"])
	}
	if cg["ownerId"] != sub {
		t.Fatalf("expected ownerId=%q, got %v", sub, cg["ownerId"])
	}
}

func TestGraphQL_CreateCard_Then_CardsByCardgroup(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	cgID := createTestCardgroup(t, ts.URL, tok, "Cards")
	cardID := createTestCard(t, ts.URL, tok, cgID, "front", "back")

	body := fmt.Sprintf(`{"query":"{ cardsByCardgroup(cardgroupId: \"%s\") { id front back cardgroup { id name } } }"}`, cgID)
	resp := postGraphQL(t, ts.URL+"/query", body, tok)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("cardsByCardgroup errors: %v", errs)
	}
	list, _ := resp["data"].(map[string]any)["cardsByCardgroup"].([]any)
	if len(list) != 1 {
		t.Fatalf("expected 1 card, got %d; resp=%v", len(list), resp)
	}
	card, _ := list[0].(map[string]any)
	if card["id"] != cardID {
		t.Fatalf("card id=%v, want %q", card["id"], cardID)
	}
	cg, _ := card["cardgroup"].(map[string]any)
	if cg == nil || cg["id"] != cgID || cg["name"] != "Cards" {
		t.Fatalf("cardgroup resolver returned %v", cg)
	}
}

func TestGraphQL_CreateCard_NonOwner_Unauthenticated(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()

	subA := insertAuthUser(t, ctx)
	tokA := f.sign(t, subA)
	cgID := createTestCardgroup(t, ts.URL, tokA, "A's group")

	subB := insertAuthUser(t, ctx)
	tokB := f.sign(t, subB)
	body := fmt.Sprintf(`{"query":"mutation { createCard(input: {cardgroupId: \"%s\", front: \"x\", back: \"y\"}) { card { id } } }"}`, cgID)
	resp := postGraphQL(t, ts.URL+"/query", body, tokB)

	if code := gqlErrCode(resp); code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q; resp=%v", code, resp)
	}
}

func TestGraphQL_CreateCard_Validation(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)
	cgID := createTestCardgroup(t, ts.URL, tok, "Validation")

	body := fmt.Sprintf(`{"query":"mutation { createCard(input: {cardgroupId: \"%s\", front: \"\", back: \"y\"}) { card { id } } }"}`, cgID)
	resp := postGraphQL(t, ts.URL+"/query", body, tok)

	if code := gqlErrCode(resp); code != "BAD_USER_INPUT" {
		t.Fatalf("expected BAD_USER_INPUT, got %q; resp=%v", code, resp)
	}
	if field := gqlErrField(resp); field != "front" {
		t.Fatalf("expected extensions.field=front, got %q; resp=%v", field, resp)
	}
}

func TestGraphQL_MyCardgroups_DoesNotLeakOtherUsers(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()

	subA := insertAuthUser(t, ctx)
	tokA := f.sign(t, subA)
	createTestCardgroup(t, ts.URL, tokA, "A's group")

	subB := insertAuthUser(t, ctx)
	tokB := f.sign(t, subB)

	// B sees zero cardgroups before creating any.
	resp := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroups { id } }"}`, tokB)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("myCardgroups (B, empty) errors: %v", errs)
	}
	listB, _ := resp["data"].(map[string]any)["myCardgroups"].([]any)
	if len(listB) != 0 {
		t.Fatalf("expected B to see 0 cardgroups, got %d", len(listB))
	}

	// B creates one; now B sees exactly one and A still sees exactly one.
	createTestCardgroup(t, ts.URL, tokB, "B's group")

	respB2 := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroups { id } }"}`, tokB)
	listB2, _ := respB2["data"].(map[string]any)["myCardgroups"].([]any)
	if len(listB2) != 1 {
		t.Fatalf("expected B to see 1 cardgroup, got %d", len(listB2))
	}

	respA2 := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroups { id } }"}`, tokA)
	listA2, _ := respA2["data"].(map[string]any)["myCardgroups"].([]any)
	if len(listA2) != 1 {
		t.Fatalf("expected A to still see 1 cardgroup, got %d", len(listA2))
	}
}

func TestGraphQL_UpdateCardgroup_NonOwner_Unauthenticated(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()

	subA := insertAuthUser(t, ctx)
	tokA := f.sign(t, subA)
	cgID := createTestCardgroup(t, ts.URL, tokA, "Owner's group")

	subB := insertAuthUser(t, ctx)
	tokB := f.sign(t, subB)
	body := fmt.Sprintf(`{"query":"mutation { updateCardgroup(id: \"%s\", input: {name: \"stolen\"}) { cardgroup { id } } }"}`, cgID)
	resp := postGraphQL(t, ts.URL+"/query", body, tokB)

	if code := gqlErrCode(resp); code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q; resp=%v", code, resp)
	}
	data, _ := resp["data"].(map[string]any)
	if data["updateCardgroup"] != nil {
		t.Fatalf("expected data.updateCardgroup to be nil, got %v", data["updateCardgroup"])
	}

	// Verify side-effect: the cardgroup name must be unchanged after the failed update.
	cgBody := fmt.Sprintf(`{"query":"{ cardgroup(id: \"%s\") { id name } }"}`, cgID)
	cgResp := postGraphQL(t, ts.URL+"/query", cgBody, tokA)
	if errs, ok := cgResp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("re-fetch cardgroup errors: %v", errs)
	}
	cgData, _ := cgResp["data"].(map[string]any)
	cg, _ := cgData["cardgroup"].(map[string]any)
	if cg == nil {
		t.Fatalf("expected cardgroup on re-fetch, got nil; resp=%v", cgResp)
	}
	if cg["name"] != "Owner's group" {
		t.Fatalf("cardgroup name was mutated: got %q, want %q", cg["name"], "Owner's group")
	}
}

func TestGraphQL_DeleteCardgroup_NonOwner_Unauthenticated(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()

	subA := insertAuthUser(t, ctx)
	tokA := f.sign(t, subA)
	cgID := createTestCardgroup(t, ts.URL, tokA, "Owner's group")

	subB := insertAuthUser(t, ctx)
	tokB := f.sign(t, subB)
	body := fmt.Sprintf(`{"query":"mutation { deleteCardgroup(id: \"%s\") }"}`, cgID)
	resp := postGraphQL(t, ts.URL+"/query", body, tokB)

	if code := gqlErrCode(resp); code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q; resp=%v", code, resp)
	}

	// Verify side-effect: the cardgroup must still exist in user A's list.
	listResp := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroups { id } }"}`, tokA)
	if errs, ok := listResp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("myCardgroups re-fetch errors: %v", errs)
	}
	list, _ := listResp["data"].(map[string]any)["myCardgroups"].([]any)
	found := false
	for _, item := range list {
		cg, _ := item.(map[string]any)
		if cg["id"] == cgID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("cardgroup %q was deleted by a non-owner; still expected in myCardgroups", cgID)
	}
}

func TestGraphQL_Cardgroup_NonOwner_Unauthenticated(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()

	subA := insertAuthUser(t, ctx)
	tokA := f.sign(t, subA)
	cgID := createTestCardgroup(t, ts.URL, tokA, "A's private group")

	subB := insertAuthUser(t, ctx)
	tokB := f.sign(t, subB)
	body := fmt.Sprintf(`{"query":"{ cardgroup(id: \"%s\") { id } }"}`, cgID)
	resp := postGraphQL(t, ts.URL+"/query", body, tokB)

	if code := gqlErrCode(resp); code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q; resp=%v", code, resp)
	}
}

func TestGraphQL_Cardgroup_OwnerLoaderResolves(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	createTestCardgroup(t, ts.URL, tok, "Loader Test")

	resp := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroups { id name owner { id displayName } } }"}`, tok)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("myCardgroups with owner errors: %v", errs)
	}
	list, _ := resp["data"].(map[string]any)["myCardgroups"].([]any)
	if len(list) == 0 {
		t.Fatalf("expected at least one cardgroup; resp=%v", resp)
	}
	for i, item := range list {
		cg, _ := item.(map[string]any)
		owner, _ := cg["owner"].(map[string]any)
		if owner == nil {
			t.Fatalf("cardgroup[%d]: owner is nil; cg=%v", i, cg)
		}
		if owner["id"] != sub {
			t.Fatalf("cardgroup[%d]: owner.id=%v, want %q", i, owner["id"], sub)
		}
	}
}

func TestGraphQL_Cardgroup_OwnerLoader_NoNPlus1(t *testing.T) {
	f := newJWTFixture(t)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	// Wrap the real repo in a counter so we can assert batching behaviour.
	realDB, err := database.Open(ctx, database.Config{URL: testDBURL})
	if err != nil {
		t.Fatalf("db open for counter: %v", err)
	}
	t.Cleanup(realDB.Close)
	counter := &countingUserRepo{inner: repository.NewUserRepository(realDB.GORM)}

	ts, _ := newGraphQLTestServerWithUserRepo(t, f, counter)

	const n = 100
	for i := range n {
		createTestCardgroup(t, ts.URL, tok, fmt.Sprintf("Batch Group %d", i+1))
	}

	resp := postGraphQL(t, ts.URL+"/query",
		`{"query":"query BatchOwner { myCardgroups { id owner { id displayName } } }"}`, tok)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("myCardgroups batch errors: %v", errs)
	}
	list, _ := resp["data"].(map[string]any)["myCardgroups"].([]any)
	if len(list) != n {
		t.Fatalf("expected %d cardgroups, got %d", n, len(list))
	}
	for i, item := range list {
		cg, _ := item.(map[string]any)
		owner, _ := cg["owner"].(map[string]any)
		if owner == nil {
			t.Fatalf("cardgroup[%d]: owner is nil", i)
		}
		if owner["id"] != sub {
			t.Fatalf("cardgroup[%d]: owner.id=%v, want %q", i, owner["id"], sub)
		}
	}

	// Assert batching: the DataLoader must call FindByIDs exactly once (all 100
	// cardgroups share the same owner ID, which the loader deduplicates), and
	// must never fall back to the per-item FindByID path.
	if got := counter.findByID.Load(); got != 0 {
		t.Errorf("FindByID called %d times; want 0 (loader must not use per-item path)", got)
	}
	if got := counter.findByIDs.Load(); got != 1 {
		t.Errorf("FindByIDs called %d times; want exactly 1 (single batched call)", got)
	}
	t.Logf("N+1 check: FindByID=%d FindByIDs=%d (keys per call: %v)",
		counter.findByID.Load(), counter.findByIDs.Load(), counter.receivedKeys)
}

func TestGraphQL_CreateCardgroup_NameTooShort(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	resp := postGraphQL(t, ts.URL+"/query", `{"query":"mutation { createCardgroup(input: {name: \"\"}) { cardgroup { id } } }"}`, tok)

	if code := gqlErrCode(resp); code != "BAD_USER_INPUT" {
		t.Fatalf("expected BAD_USER_INPUT, got %q; resp=%v", code, resp)
	}
	if field := gqlErrField(resp); field != "name" {
		t.Fatalf("expected extensions.field=name, got %q; resp=%v", field, resp)
	}
}

func TestGraphQL_CreateCardgroup_NameTooLong(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	longName := strings.Repeat("x", 101)
	body := fmt.Sprintf(`{"query":"mutation { createCardgroup(input: {name: \"%s\"}) { cardgroup { id } } }"}`, longName)
	resp := postGraphQL(t, ts.URL+"/query", body, tok)

	if code := gqlErrCode(resp); code != "BAD_USER_INPUT" {
		t.Fatalf("expected BAD_USER_INPUT, got %q; resp=%v", code, resp)
	}
	if field := gqlErrField(resp); field != "name" {
		t.Fatalf("expected extensions.field=name, got %q; resp=%v", field, resp)
	}
}

func TestGraphQL_CreateCardgroup_Anonymous_Unauthenticated(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)

	resp := postGraphQL(t, ts.URL+"/query", `{"query":"mutation { createCardgroup(input: {name: \"Anon\"}) { cardgroup { id } } }"}`, "")

	if code := gqlErrCode(resp); code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q; resp=%v", code, resp)
	}
}

// TestGraphQL_Cardgroup_NotFound_ReturnsNullNoError verifies that querying a
// non-existent cardgroup ID resolves to null data without a GraphQL error,
// matching the resolver contract (usecase returns nil for ErrNotFound).
func TestGraphQL_Cardgroup_NotFound_ReturnsNullNoError(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	randomID := uuid.NewString()
	body := fmt.Sprintf(`{"query":"{ cardgroup(id: \"%s\") { id name } }"}`, randomID)
	resp := postGraphQL(t, ts.URL+"/query", body, tok)

	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("expected no errors for missing cardgroup, got: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatalf("expected data object in response, got nil; resp=%v", resp)
	}
	// The resolver must return null (JSON null), not omit the field.
	cgVal, exists := data["cardgroup"]
	if !exists {
		t.Fatalf("expected data.cardgroup key to be present (as null), resp=%v", resp)
	}
	if cgVal != nil {
		t.Fatalf("expected data.cardgroup == null for non-existent ID, got %v", cgVal)
	}
}

func TestGraphQL_HandleSwipe_HappyPath(t *testing.T) {
	f := newJWTFixture(t)
	ts, db := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	cgID := createTestCardgroup(t, ts.URL, tok, "Swipe")
	firstID := createTestCard(t, ts.URL, tok, cgID, "front 1", "back 1")
	secondID := createTestCard(t, ts.URL, tok, cgID, "front 2", "back 2")

	gqlQuery := fmt.Sprintf(`mutation {
		handleSwipe(input: {cardId: %s, cardgroupId: %s, mode: 4}) {
			performanceMode
			nextCards { id }
		}
	}`, gqlStringLit(firstID), gqlStringLit(cgID))
	body, err := json.Marshal(map[string]string{"query": gqlQuery})
	if err != nil {
		t.Fatalf("json.Marshal body: %v", err)
	}
	resp := postGraphQL(t, ts.URL+"/query", string(body), tok)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("handleSwipe errors: %v", errs)
	}
	payload, _ := resp["data"].(map[string]any)["handleSwipe"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected handleSwipe payload; resp=%v", resp)
	}
	if payload["performanceMode"] != float64(0) {
		t.Fatalf("performanceMode=%v, want 0", payload["performanceMode"])
	}
	nextCards, _ := payload["nextCards"].([]any)
	for _, item := range nextCards {
		card, _ := item.(map[string]any)
		if card["id"] == firstID {
			t.Fatalf("nextCards included swiped card %q: %v", firstID, nextCards)
		}
	}
	if len(nextCards) > 0 {
		card, _ := nextCards[0].(map[string]any)
		if card["id"] != secondID {
			t.Fatalf("expected due sibling card first, got %v", card)
		}
	}

	var swipeCount int64
	if err := db.GORM.WithContext(ctx).Table("swipe_records").Where("card_id = ?", firstID).Count(&swipeCount).Error; err != nil {
		t.Fatalf("count swipe_records: %v", err)
	}
	if swipeCount != 1 {
		t.Fatalf("swipe_records count=%d, want 1", swipeCount)
	}
	cardRepo := repository.NewCardRepository(db.GORM)
	updated, err := cardRepo.FindByID(ctx, firstID)
	if err != nil {
		t.Fatalf("find swiped card: %v", err)
	}
	if updated.FSRS.Reps != 1 {
		t.Fatalf("reps=%d, want 1", updated.FSRS.Reps)
	}
}

func TestGraphQL_HandleSwipe_InvalidModes(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)
	cgID := createTestCardgroup(t, ts.URL, tok, "Invalid Modes")
	cardID := createTestCard(t, ts.URL, tok, cgID, "front", "back")

	for _, mode := range []int{0, 3, 5} {
		body := fmt.Sprintf(`{"query":"mutation { handleSwipe(input: {cardId: \"%s\", cardgroupId: \"%s\", mode: %d}) { performanceMode } }"}`, cardID, cgID, mode)
		resp := postGraphQL(t, ts.URL+"/query", body, tok)
		if code := gqlErrCode(resp); code != "BAD_USER_INPUT" {
			t.Fatalf("mode=%d: expected BAD_USER_INPUT, got %q; resp=%v", mode, code, resp)
		}
		if field := gqlErrField(resp); field != "mode" {
			t.Fatalf("mode=%d: expected extensions.field=mode, got %q; resp=%v", mode, field, resp)
		}
	}
}

func TestGraphQL_HandleSwipe_NonOwnerUnauthenticated(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	subA := insertAuthUser(t, ctx)
	tokA := f.sign(t, subA)
	cgID := createTestCardgroup(t, ts.URL, tokA, "Owner")
	cardID := createTestCard(t, ts.URL, tokA, cgID, "front", "back")

	subB := insertAuthUser(t, ctx)
	tokB := f.sign(t, subB)
	body := fmt.Sprintf(`{"query":"mutation { handleSwipe(input: {cardId: \"%s\", cardgroupId: \"%s\", mode: 4}) { performanceMode } }"}`, cardID, cgID)
	resp := postGraphQL(t, ts.URL+"/query", body, tokB)

	if code := gqlErrCode(resp); code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q; resp=%v", code, resp)
	}
}

func TestGraphQL_HandleSwipe_CrossCardgroupMatchesMissingCardError(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	ownedCgID := createTestCardgroup(t, ts.URL, tok, "Owned")
	otherCgID := createTestCardgroup(t, ts.URL, tok, "Other")
	otherCardID := createTestCard(t, ts.URL, tok, otherCgID, "other front", "other back")
	missingCardID := uuid.NewString()

	bodyForCard := func(cardID string) string {
		return fmt.Sprintf(`{"query":"mutation { handleSwipe(input: {cardId: \"%s\", cardgroupId: \"%s\", mode: 4}) { performanceMode } }"}`, cardID, ownedCgID)
	}
	mismatchResp := postGraphQL(t, ts.URL+"/query", bodyForCard(otherCardID), tok)
	missingResp := postGraphQL(t, ts.URL+"/query", bodyForCard(missingCardID), tok)

	for name, resp := range map[string]map[string]any{"mismatch": mismatchResp, "missing": missingResp} {
		if code := gqlErrCode(resp); code != "BAD_USER_INPUT" {
			t.Fatalf("%s: expected BAD_USER_INPUT, got %q; resp=%v", name, code, resp)
		}
		if field := gqlErrField(resp); field != "cardId" {
			t.Fatalf("%s: expected extensions.field=cardId, got %q; resp=%v", name, field, resp)
		}
	}
	if gqlErrMessage(mismatchResp) != gqlErrMessage(missingResp) {
		t.Fatalf("mismatched-cardgroup and missing-card errors differ: mismatch=%q missing=%q",
			gqlErrMessage(mismatchResp), gqlErrMessage(missingResp))
	}
}

type failingSwipeRepo struct{ err error }

func (f failingSwipeRepo) CreateTx(context.Context, *gorm.DB, *domain.SwipeRecord) error {
	return f.err
}

func TestHandleSwipe_RollsBackWhenSwipeRecordInsertFails(t *testing.T) {
	f := newJWTFixture(t)
	ts, db := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)
	cgID := createTestCardgroup(t, ts.URL, tok, "Rollback")
	cardID := createTestCard(t, ts.URL, tok, cgID, "front", "back")

	cardRepo := repository.NewCardRepository(db.GORM)
	cardgroupRepo := repository.NewCardgroupRepository(db.GORM)
	before, err := cardRepo.FindByID(ctx, cardID)
	if err != nil {
		t.Fatalf("find before: %v", err)
	}
	uc := usecase.NewSwipeUsecase(
		db.GORM,
		cardRepo,
		cardgroupRepo,
		failingSwipeRepo{err: errors.New("forced swipe insert failure")},
		service.NewFSRSScheduler(),
		10,
	)

	_, err = uc.HandleSwipe(auth.ContextWithUser(ctx, &auth.AuthUser{Sub: sub}), usecase.HandleSwipeInput{
		CardID:      cardID,
		CardgroupID: cgID,
		Mode:        4,
	})
	if err == nil {
		t.Fatal("expected forced error, got nil")
	}
	after, err := cardRepo.FindByID(ctx, cardID)
	if err != nil {
		t.Fatalf("find after: %v", err)
	}
	if after.FSRS.Reps != before.FSRS.Reps || !after.FSRS.Due.Equal(before.FSRS.Due) {
		t.Fatalf("FSRS state changed despite rollback: before=%+v after=%+v", before.FSRS, after.FSRS)
	}
	var swipeCount int64
	if err := db.GORM.WithContext(ctx).Table("swipe_records").Where("card_id = ?", cardID).Count(&swipeCount).Error; err != nil {
		t.Fatalf("count swipe_records: %v", err)
	}
	if swipeCount != 0 {
		t.Fatalf("swipe_records count=%d, want 0", swipeCount)
	}
}
