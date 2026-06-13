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

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"github.com/rotisserie/eris"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gorm.io/gorm"

	"backend/graph/generated"
	"backend/graph/model"
	"backend/graph/resolver"
	"backend/internal/auth"
	"backend/internal/database"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/gqlerr"
	"backend/internal/handler/ping"
	"backend/internal/logging"
	"backend/internal/repository"
	"backend/internal/telemetry"
	"backend/internal/usecase"
)

var testDBURL string

// stubAdminChecker satisfies usecase.AdminChecker for wiring/smoke tests that
// do not exercise the cardgroup-limit guard. Passing isAdmin: true short-circuits
// the limit check and preserves prior test behavior.
type stubAdminChecker struct {
	isAdmin bool
	err     error
}

func (s stubAdminChecker) IsAdmin(_ context.Context, _ string) (bool, error) {
	return s.isAdmin, s.err
}

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
	ts := httptest.NewServer(newRouter(resolver.NewResolver(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil), noopAuthMW, auth.NewSuperUserPromoter(nil, "", nil, nil), nil, nil, nil, nil, nil, nil, nil, ping.New(nil, "test-token"), nil, nil))
	t.Cleanup(ts.Close)
	return ts
}

func setNotionSyncEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NOTION_TOKEN", "notion-test-token")
	t.Setenv("NOTION_PAGE_IDS", "page-1,page-2")
	t.Setenv("NOTION_MASTER_CARDGROUP_NAME", "English")
	t.Setenv("NOTION_SYNC_TOKEN", "sync-test-token")
}

// unsetNotionSyncEnv blanks all four NOTION_* vars so that run() treats
// notion-sync as disabled and skips registering /internal/notion-sync.
func unsetNotionSyncEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NOTION_TOKEN", "")
	t.Setenv("NOTION_PAGE_IDS", "")
	t.Setenv("NOTION_MASTER_CARDGROUP_NAME", "")
	t.Setenv("NOTION_SYNC_TOKEN", "")
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

func TestHealthEndpoint_Head(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)

	res, err := http.Head(ts.URL + "/health")
	if err != nil {
		t.Fatalf("HEAD %s/health: %v", ts.URL, err)
	}
	defer res.Body.Close()

	// HEAD must succeed (not 405) so external uptime monitors that only issue
	// HEAD requests classify the service as up while still resetting Render's
	// free-tier idle timer.
	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	// Per HTTP semantics net/http strips the body from a HEAD response.
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("HEAD response body = %q, want empty", body)
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
	setNotionSyncEnv(t)

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

// TestRun_NotionSyncDisabledWhenEnvMissing verifies that when all five
// NOTION_* env vars are absent, run() starts successfully and the router does
// not register /internal/notion-sync (the nil-guard in newRouter skips it).
// A POST to that path must return 404.
func TestRun_NotionSyncDisabledWhenEnvMissing(t *testing.T) {
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
	unsetNotionSyncEnv(t)

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

	// /internal/notion-sync must be absent from the router when the handler is nil.
	res, err := http.Post("http://127.0.0.1:"+port+"/internal/notion-sync", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /internal/notion-sync: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d (notion-sync route should be unregistered)", res.StatusCode, http.StatusNotFound)
	}

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
	// Build a JWKS entry for the test JWKS endpoint. The handler below serves
	// these as JSON Web Keys per RFC 7517/7518: an "EC" key on curve "P-256"
	// is described by the base64url-encoded x and y coordinates of its public
	// point (RFC 7518 §6.2.1).
	//
	// Go 1.26 deprecated direct access to ecdsa.PublicKey.X / Y because:
	//   1. *big.Int handles are mutable — callers could overwrite the raw
	//      coordinates and produce an invalid key.
	//   2. *big.Int operations are not constant-time, so reading the
	//      coordinates with .Bytes() leaks timing information on shared
	//      buffers.
	//
	// (*ecdsa.PublicKey).Bytes (added in Go 1.25) returns the SEC1
	// uncompressed encoding:
	//
	//   pub = 0x04 || X (32 bytes, big-endian) || Y (32 bytes, big-endian)
	//
	// For P-256 that is exactly 65 bytes, so pub[1:33] is X and pub[33:] is Y.
	// Unlike (*big.Int).Bytes(), .Bytes() always pads to the curve byte length
	// (32 for P-256), which is what JWKS requires — the previous code path
	// could emit a 31-byte coordinate when X or Y happened to have a leading
	// zero, yielding a JWKS entry that some parsers reject.
	pub, err := priv.PublicKey.Bytes()
	if err != nil {
		t.Fatalf("public key bytes: %v", err)
	}
	xB64 := base64.RawURLEncoding.EncodeToString(pub[1:33])
	yB64 := base64.RawURLEncoding.EncodeToString(pub[33:])
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
	userRoleRepo := repository.NewUserRoleRepository(db.GORM)
	cardgroupRepo := repository.NewCardgroupRepository(db.GORM)
	cardRepo := repository.NewCardRepository(db.GORM)
	userCardFSRSRepo := repository.NewUserCardFSRSRepository(db.GORM)
	swipeRecordRepo := repository.NewSwipeRecordRepository(db.GORM)
	logger := slog.New(slog.DiscardHandler)
	userUC := usecase.NewUserUsecase(userRepo, userRoleRepo, nil, nil, logger)
	cardgroupUC := usecase.NewCardgroupUsecase(cardgroupRepo, stubAdminChecker{isAdmin: true}, logger)
	cardUC := usecase.NewCardUsecase(db.GORM, cardRepo, cardgroupRepo, userCardFSRSRepo, nil, logger)
	swipeUC := usecase.NewSwipeUsecase(db.GORM, cardRepo, cardgroupRepo, swipeRecordRepo, service.NewFSRSScheduler(), userCardFSRSRepo, logger)
	pingRecordRepo := repository.NewPingRecordRepository(db.GORM)
	userPreferenceRepo := repository.NewUserPreferenceRepository(db.GORM)
	e := newRouter(resolver.NewResolver(userUC, cardgroupUC, cardUC, swipeUC, nil, nil, nil, nil, nil, nil, nil, nil, nil), mw, auth.NewSuperUserPromoter(nil, "", nil, nil), userRepo, roleRepo, userRoleRepo, cardgroupRepo, cardRepo, userPreferenceRepo, userCardFSRSRepo, ping.New(pingRecordRepo, "test-token"), nil, swipeRecordRepo)

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
func (c *countingUserRepo) UpdateTx(ctx context.Context, tx *gorm.DB, id string, patch repository.UserUpdate) error {
	return c.inner.UpdateTx(ctx, tx, id, patch)
}
func (c *countingUserRepo) UpdateTxVersioned(ctx context.Context, tx *gorm.DB, id string, patch repository.UserUpdate, expectedVersion int64) error {
	return c.inner.UpdateTxVersioned(ctx, tx, id, patch, expectedVersion)
}

// ListPage forwards to the inner repository so any future test that exercises
// the cursor-paginated user list keeps working.
func (c *countingUserRepo) ListPage(
	ctx context.Context,
	after, before *string,
	first, last int,
	search *string,
) ([]*domain.User, int64, error) {
	return c.inner.ListPage(ctx, after, before, first, last, search)
}

func (c *countingUserRepo) DeleteAuthUser(ctx context.Context, id string) error {
	return c.inner.DeleteAuthUser(ctx, id)
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

	mutation := `{"query":"mutation { updateProfile(input: { displayName: \"Alice\", bio: \"hi\" }) { __typename ... on UpdateProfileSuccess { user { id displayName bio } } ... on InputValidationError { field message } } }"}`
	resp := postGraphQL(t, ts.URL+"/query", mutation, tok)

	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("unexpected errors on mutation: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	upd, _ := data["updateProfile"].(map[string]any)
	if upd["__typename"] != "UpdateProfileSuccess" {
		t.Fatalf("expected UpdateProfileSuccess, got %v; resp=%v", upd["__typename"], resp)
	}
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
	ts := httptest.NewServer(newGraphQLServer(resolver.NewResolver(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)))
	t.Cleanup(ts.Close)
	return ts
}

const introspectionQuery = `{"query":"{ __schema { queryType { name } } }"}`

// assertIntrospectionDisabled posts an introspection query and asserts the
// server rejects it with an "introspection disabled" validation error.
func assertIntrospectionDisabled(t *testing.T) {
	t.Helper()
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

// assertIntrospectionEnabled posts an introspection query and asserts the
// server returns a populated __schema with no errors.
func assertIntrospectionEnabled(t *testing.T) {
	t.Helper()
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

// Introspection is fail-safe (opt-in): enabled only when GRAPHQL_INTROSPECTION
// is set to exactly "on". An unset or any other value keeps it disabled.

func TestIntrospection_UnsetDisabled(t *testing.T) {
	t.Setenv("GRAPHQL_INTROSPECTION", "")
	assertIntrospectionDisabled(t)
}

func TestIntrospection_OffDisabled(t *testing.T) {
	t.Setenv("GRAPHQL_INTROSPECTION", "off")
	assertIntrospectionDisabled(t)
}

func TestIntrospection_OnEnabled(t *testing.T) {
	t.Setenv("GRAPHQL_INTROSPECTION", "on")
	assertIntrospectionEnabled(t)
}

// getStatus issues a GET and returns only the HTTP status code. Used for
// routes whose body is not JSON (the Playground UI serves HTML).
func getStatus(t *testing.T, url string) int {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer res.Body.Close()
	return res.StatusCode
}

// TestPlayground_GatedOff asserts that GRAPHQL_INTROSPECTION=off leaves the
// /playground route unregistered (404). The Playground UI is useless without
// introspection, so production (where introspection is off) does not serve it.
func TestPlayground_GatedOff(t *testing.T) {
	t.Setenv("GRAPHQL_INTROSPECTION", "off")
	ts := newTestServer(t)

	if got := getStatus(t, ts.URL+"/playground"); got != http.StatusNotFound {
		t.Fatalf("GET /playground status = %d, want %d (route should be gated off)", got, http.StatusNotFound)
	}
}

// TestPlayground_DefaultOn asserts that the /playground route is registered
// (returns 200) for any value other than "off" — the route gate is `!= "off"`,
// which is intentionally looser than the introspection gate (`== "on"`).
func TestPlayground_DefaultOn(t *testing.T) {
	t.Setenv("GRAPHQL_INTROSPECTION", "")
	ts := newTestServer(t)

	if got := getStatus(t, ts.URL+"/playground"); got != http.StatusOK {
		t.Fatalf("GET /playground status = %d, want %d (route should be registered)", got, http.StatusOK)
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
// Uses the CreateCardgroupResult union selection set; validates __typename before extracting the id.
func createTestCardgroup(t *testing.T, srvURL, bearer, name string) string {
	t.Helper()
	gqlQuery := fmt.Sprintf(`mutation { createCardgroup(input: {name: %s}) { __typename ... on CreateCardgroupSuccess { cardgroup { id name ownerId } } ... on InputValidationError { field message } } }`, gqlStringLit(name))
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
	if payload == nil {
		t.Fatalf("createCardgroup: expected payload, got nil; resp=%v", resp)
	}
	if payload["__typename"] != "CreateCardgroupSuccess" {
		t.Fatalf("createCardgroup: expected CreateCardgroupSuccess, got %v; resp=%v", payload["__typename"], resp)
	}
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
	// createCard now returns the CreateCardResult union: CreateCardSuccess on
	// the happy path, CardDuplicateFrontError when (cardgroup_id, front)
	// collides. The selection set discriminates via __typename so this helper
	// can assert success and surface the duplicate-front payload as a test failure.
	gqlQuery := fmt.Sprintf(`mutation {
		createCard(input: {cardgroupId: %s, front: %s, back: %s}) {
			__typename
			... on CreateCardSuccess {
				card { id front back cardgroupId }
			}
			... on CardDuplicateFrontError {
				message
				existingCardId
				existingBack
			}
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
	if typename, _ := payload["__typename"].(string); typename != "CreateCardSuccess" {
		t.Fatalf("createCard: expected CreateCardSuccess, got %q; resp=%v", typename, resp)
	}
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

func TestGraphQL_CreateCardgroup_Then_MyCardgroupsConnection(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	cgID := createTestCardgroup(t, ts.URL, tok, "Vocab 1")

	resp := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroupsConnection(first: 100) { edges { node { id name ownerId } } totalCount } }"}`, tok)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("myCardgroupsConnection errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	conn, _ := data["myCardgroupsConnection"].(map[string]any)
	edges, _ := conn["edges"].([]any)
	if len(edges) != 1 {
		t.Fatalf("expected 1 cardgroup, got %d; resp=%v", len(edges), resp)
	}
	cg, _ := edges[0].(map[string]any)["node"].(map[string]any)
	if cg["id"] != cgID {
		t.Fatalf("expected id=%q, got %v", cgID, cg["id"])
	}
	if cg["name"] != "Vocab 1" {
		t.Fatalf("expected name=Vocab 1, got %v", cg["name"])
	}
	if cg["ownerId"] != sub {
		t.Fatalf("expected ownerId=%q, got %v", sub, cg["ownerId"])
	}
	if got := conn["totalCount"].(float64); got != 1 {
		t.Fatalf("expected totalCount=1, got %v", got)
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
	body := fmt.Sprintf(`{"query":"mutation { createCard(input: {cardgroupId: \"%s\", front: \"x\", back: \"y\"}) { __typename ... on CreateCardSuccess { card { id } } } }"}`, cgID)
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

	body := fmt.Sprintf(`{"query":"mutation { createCard(input: {cardgroupId: \"%s\", front: \"\", back: \"y\"}) { __typename ... on CreateCardSuccess { card { id } } } }"}`, cgID)
	resp := postGraphQL(t, ts.URL+"/query", body, tok)

	if code := gqlErrCode(resp); code != "BAD_USER_INPUT" {
		t.Fatalf("expected BAD_USER_INPUT, got %q; resp=%v", code, resp)
	}
	if field := gqlErrField(resp); field != "front" {
		t.Fatalf("expected extensions.field=front, got %q; resp=%v", field, resp)
	}
}

func TestGraphQL_MyCardgroupsConnection_DoesNotLeakOtherUsers(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()

	subA := insertAuthUser(t, ctx)
	tokA := f.sign(t, subA)
	createTestCardgroup(t, ts.URL, tokA, "A's group")

	subB := insertAuthUser(t, ctx)
	tokB := f.sign(t, subB)

	// B sees zero cardgroups before creating any.
	resp := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroupsConnection(first: 100) { edges { node { id } } totalCount } }"}`, tokB)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("myCardgroupsConnection (B, empty) errors: %v", errs)
	}
	connB, _ := resp["data"].(map[string]any)["myCardgroupsConnection"].(map[string]any)
	listB, _ := connB["edges"].([]any)
	if len(listB) != 0 {
		t.Fatalf("expected B to see 0 cardgroups, got %d", len(listB))
	}
	if got := connB["totalCount"].(float64); got != 0 {
		t.Fatalf("expected B totalCount=0 before creating any cardgroup, got %v", got)
	}

	// B creates one; now B sees exactly one and A still sees exactly one.
	createTestCardgroup(t, ts.URL, tokB, "B's group")

	respB2 := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroupsConnection(first: 100) { edges { node { id } } totalCount } }"}`, tokB)
	connB2, _ := respB2["data"].(map[string]any)["myCardgroupsConnection"].(map[string]any)
	listB2, _ := connB2["edges"].([]any)
	if len(listB2) != 1 {
		t.Fatalf("expected B to see 1 cardgroup, got %d", len(listB2))
	}
	if got := connB2["totalCount"].(float64); got != 1 {
		t.Fatalf("expected B totalCount=1 after creating one cardgroup, got %v", got)
	}

	respA2 := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroupsConnection(first: 100) { edges { node { id } } totalCount } }"}`, tokA)
	connA2, _ := respA2["data"].(map[string]any)["myCardgroupsConnection"].(map[string]any)
	listA2, _ := connA2["edges"].([]any)
	if len(listA2) != 1 {
		t.Fatalf("expected A to still see 1 cardgroup, got %d", len(listA2))
	}
	if got := connA2["totalCount"].(float64); got != 1 {
		t.Fatalf("expected A totalCount=1 (unaffected by B's create), got %v", got)
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
	body := fmt.Sprintf(`{"query":"mutation { updateCardgroup(id: \"%s\", input: {name: \"stolen\"}) { __typename } }"}`, cgID)
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
	listResp := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroupsConnection(first: 100) { edges { node { id } } totalCount } }"}`, tokA)
	if errs, ok := listResp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("myCardgroupsConnection re-fetch errors: %v", errs)
	}
	connList, _ := listResp["data"].(map[string]any)["myCardgroupsConnection"].(map[string]any)
	list, _ := connList["edges"].([]any)
	found := false
	for _, item := range list {
		edge, _ := item.(map[string]any)
		cg, _ := edge["node"].(map[string]any)
		if cg["id"] == cgID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("cardgroup %q was deleted by a non-owner; still expected in myCardgroupsConnection", cgID)
	}
	if got := connList["totalCount"].(float64); got != 1 {
		t.Fatalf("expected totalCount=1 after failed non-owner delete, got %v", got)
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

	resp := postGraphQL(t, ts.URL+"/query", `{"query":"{ myCardgroupsConnection(first: 100) { edges { node { id name owner { id displayName } } } totalCount } }"}`, tok)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("myCardgroupsConnection with owner errors: %v", errs)
	}
	conn, _ := resp["data"].(map[string]any)["myCardgroupsConnection"].(map[string]any)
	list, _ := conn["edges"].([]any)
	if len(list) == 0 {
		t.Fatalf("expected at least one cardgroup; resp=%v", resp)
	}
	if got := conn["totalCount"].(float64); got != 1 {
		t.Fatalf("expected totalCount=1, got %v", got)
	}
	for i, item := range list {
		cg, _ := item.(map[string]any)["node"].(map[string]any)
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
		`{"query":"query BatchOwner { myCardgroupsConnection(first: 100) { edges { node { id owner { id displayName } } } totalCount } }"}`, tok)
	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("myCardgroupsConnection batch errors: %v", errs)
	}
	conn, _ := resp["data"].(map[string]any)["myCardgroupsConnection"].(map[string]any)
	list, _ := conn["edges"].([]any)
	if len(list) != n {
		t.Fatalf("expected %d cardgroups, got %d", n, len(list))
	}
	if got := conn["totalCount"].(float64); got != float64(n) {
		t.Fatalf("expected totalCount=%d, got %v", n, got)
	}
	for i, item := range list {
		cg, _ := item.(map[string]any)["node"].(map[string]any)
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

// TestGraphQL_CreateCardgroup_NameTooShort verifies that an empty name is
// surfaced as the InputValidationError union variant — returned as data, not
// as a GraphQL protocol error. Validation failures travel through the data
// path per the outcome-union design in .claude/rules/error-wrapping.md.
func TestGraphQL_CreateCardgroup_NameTooShort(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	resp := postGraphQL(t, ts.URL+"/query", `{"query":"mutation { createCardgroup(input: {name: \"\"}) { __typename ... on InputValidationError { field message } } }"}`, tok)

	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("unexpected GraphQL errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["createCardgroup"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.createCardgroup, got nil; resp=%v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected __typename=InputValidationError, got %v; resp=%v", payload["__typename"], resp)
	}
	if payload["field"] != "name" {
		t.Fatalf("expected field=name, got %v; resp=%v", payload["field"], resp)
	}
}

// TestGraphQL_CreateCardgroup_NameTooLong verifies that a name exceeding the
// length limit is surfaced as the InputValidationError union variant — returned
// as data, not as a GraphQL error.
func TestGraphQL_CreateCardgroup_NameTooLong(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	longName := strings.Repeat("x", 101)
	body := fmt.Sprintf(`{"query":"mutation { createCardgroup(input: {name: \"%s\"}) { __typename ... on InputValidationError { field message } } }"}`, longName)
	resp := postGraphQL(t, ts.URL+"/query", body, tok)

	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("unexpected GraphQL errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["createCardgroup"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.createCardgroup, got nil; resp=%v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected __typename=InputValidationError, got %v; resp=%v", payload["__typename"], resp)
	}
	if payload["field"] != "name" {
		t.Fatalf("expected field=name, got %v; resp=%v", payload["field"], resp)
	}
}

func TestGraphQL_CreateCardgroup_Anonymous_Unauthenticated(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newGraphQLTestServer(t, f)

	resp := postGraphQL(t, ts.URL+"/query", `{"query":"mutation { createCardgroup(input: {name: \"Anon\"}) { __typename ... on CreateCardgroupSuccess { cardgroup { id } } ... on InputValidationError { field message } } }"}`, "")

	if code := gqlErrCode(resp); code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q; resp=%v", code, resp)
	}
}

// newLastViewedGraphQLTestServer builds a real router backed by testcontainer
// Postgres and wires the LastViewedCardgroupUsecase, which newGraphQLTestServer
// leaves nil. All other usecase dependencies are also wired so that helper
// functions like createTestCardgroup work correctly.
func newLastViewedGraphQLTestServer(t *testing.T, f *jwtFixture) (*httptest.Server, *database.DB) {
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

	userRepo := repository.NewUserRepository(db.GORM)
	roleRepo := repository.NewRoleRepository(db.GORM)
	userRoleRepo := repository.NewUserRoleRepository(db.GORM)
	cardgroupRepo := repository.NewCardgroupRepository(db.GORM)
	cardRepo := repository.NewCardRepository(db.GORM)
	userCardFSRSRepo := repository.NewUserCardFSRSRepository(db.GORM)
	swipeRecordRepo := repository.NewSwipeRecordRepository(db.GORM)
	userPreferenceRepo := repository.NewUserPreferenceRepository(db.GORM)
	logger := slog.New(slog.DiscardHandler)
	userUC := usecase.NewUserUsecase(userRepo, userRoleRepo, nil, nil, logger)
	cardgroupUC := usecase.NewCardgroupUsecase(cardgroupRepo, stubAdminChecker{isAdmin: true}, logger)
	cardUC := usecase.NewCardUsecase(db.GORM, cardRepo, cardgroupRepo, userCardFSRSRepo, nil, logger)
	swipeUC := usecase.NewSwipeUsecase(db.GORM, cardRepo, cardgroupRepo, swipeRecordRepo, service.NewFSRSScheduler(), userCardFSRSRepo, logger)
	lastViewedUC := usecase.NewLastViewedCardgroup(userPreferenceRepo, userRepo, logger)
	pingRecordRepo := repository.NewPingRecordRepository(db.GORM)
	e := newRouter(
		resolver.NewResolver(userUC, cardgroupUC, cardUC, swipeUC, nil, nil, nil, nil, lastViewedUC, nil, nil, nil, nil),
		mw,
		auth.NewSuperUserPromoter(nil, "", nil, nil),
		userRepo, roleRepo, userRoleRepo, cardgroupRepo, cardRepo, userPreferenceRepo, userCardFSRSRepo,
		ping.New(pingRecordRepo, "test-token"), nil, swipeRecordRepo,
	)

	ts := httptest.NewServer(e)
	t.Cleanup(ts.Close)
	return ts, db
}

// TestGraphQL_SetLastViewedCardgroup_HappyPath verifies that an authenticated
// user can record a cardgroup as last-viewed and the response carries
// __typename SetLastViewedCardgroupSuccess with the matching user and cardgroup.
func TestGraphQL_SetLastViewedCardgroup_HappyPath(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newLastViewedGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	cgID := createTestCardgroup(t, ts.URL, tok, "Last Viewed Group")

	body := fmt.Sprintf(
		`{"query":"mutation { setLastViewedCardgroup(cardgroupId: \"%s\") { __typename ... on SetLastViewedCardgroupSuccess { user { id lastViewedCardgroup { id } } } ... on InputValidationError { field message } } }"}`,
		cgID,
	)
	resp := postGraphQL(t, ts.URL+"/query", body, tok)

	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("unexpected GraphQL errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["setLastViewedCardgroup"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.setLastViewedCardgroup, got nil; resp=%v", resp)
	}
	if payload["__typename"] != "SetLastViewedCardgroupSuccess" {
		t.Fatalf("expected SetLastViewedCardgroupSuccess, got %v; resp=%v", payload["__typename"], resp)
	}
	user, _ := payload["user"].(map[string]any)
	if user == nil {
		t.Fatalf("expected user in success payload, got nil; resp=%v", resp)
	}
	if user["id"] != sub {
		t.Fatalf("expected user.id=%q, got %v; resp=%v", sub, user["id"], resp)
	}
	lastViewed, _ := user["lastViewedCardgroup"].(map[string]any)
	if lastViewed == nil {
		t.Fatalf("expected user.lastViewedCardgroup, got nil; resp=%v", resp)
	}
	if lastViewed["id"] != cgID {
		t.Fatalf("expected lastViewedCardgroup.id=%q, got %v; resp=%v", cgID, lastViewed["id"], resp)
	}
}

// TestGraphQL_SetLastViewedCardgroup_NotFound_InputValidation verifies that
// passing a cardgroup ID that does not exist (or is not owned by the caller)
// is surfaced as the InputValidationError union variant with field==cardgroupId,
// not as a GraphQL protocol error.
func TestGraphQL_SetLastViewedCardgroup_NotFound_InputValidation(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newLastViewedGraphQLTestServer(t, f)
	ctx := context.Background()
	sub := insertAuthUser(t, ctx)
	tok := f.sign(t, sub)

	nonexistentID := "00000000-0000-0000-0000-000000000000"
	body := fmt.Sprintf(
		`{"query":"mutation { setLastViewedCardgroup(cardgroupId: \"%s\") { __typename ... on SetLastViewedCardgroupSuccess { user { id } } ... on InputValidationError { field message } } }"}`,
		nonexistentID,
	)
	resp := postGraphQL(t, ts.URL+"/query", body, tok)

	if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
		t.Fatalf("unexpected GraphQL errors (validation should come as data): %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["setLastViewedCardgroup"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.setLastViewedCardgroup, got nil; resp=%v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected InputValidationError, got %v; resp=%v", payload["__typename"], resp)
	}
	if payload["field"] != "cardgroupId" {
		t.Fatalf("expected field=cardgroupId, got %v; resp=%v", payload["field"], resp)
	}
}

// TestGraphQL_SetLastViewedCardgroup_Anonymous_Unauthenticated verifies that an
// anonymous request (no bearer token) is rejected with UNAUTHENTICATED.
func TestGraphQL_SetLastViewedCardgroup_Anonymous_Unauthenticated(t *testing.T) {
	f := newJWTFixture(t)
	ts, _ := newLastViewedGraphQLTestServer(t, f)

	body := `{"query":"mutation { setLastViewedCardgroup(cardgroupId: \"some-id\") { __typename ... on SetLastViewedCardgroupSuccess { user { id } } ... on InputValidationError { field message } } }"}`
	resp := postGraphQL(t, ts.URL+"/query", body, "")

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

	gqlQuery := fmt.Sprintf(`mutation {
		handleSwipe(input: {cardId: %s, cardgroupId: %s, mode: 4}) {
			__typename
			... on HandleSwipeSuccess {
				response {
					performanceMode
					metrics { reviewCount successRate }
				}
			}
			... on InputValidationError { field message }
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
	if payload["__typename"] != "HandleSwipeSuccess" {
		t.Fatalf("expected HandleSwipeSuccess, got %v; resp=%v", payload["__typename"], resp)
	}
	swipeResp, _ := payload["response"].(map[string]any)
	if swipeResp == nil {
		t.Fatalf("expected handleSwipe.response; resp=%v", resp)
	}
	if swipeResp["performanceMode"] != float64(1) {
		t.Fatalf("performanceMode=%v, want 1", swipeResp["performanceMode"])
	}
	metrics, _ := swipeResp["metrics"].(map[string]any)
	if metrics["reviewCount"] != float64(1) {
		t.Fatalf("metrics.reviewCount=%v, want 1", metrics["reviewCount"])
	}
	if metrics["successRate"] != float64(1) {
		t.Fatalf("metrics.successRate=%v, want 1", metrics["successRate"])
	}

	var swipeCount int64
	if err := db.GORM.WithContext(ctx).Table("swipe_records").Where("card_id = ?", firstID).Count(&swipeCount).Error; err != nil {
		t.Fatalf("count swipe_records: %v", err)
	}
	if swipeCount != 1 {
		t.Fatalf("swipe_records count=%d, want 1", swipeCount)
	}
	userCardFSRSRepo := repository.NewUserCardFSRSRepository(db.GORM)
	stateRows, err := userCardFSRSRepo.FindByUserAndCardIDs(ctx, sub, []string{firstID})
	if err != nil {
		t.Fatalf("find swiped user_card_fsrs: %v", err)
	}
	if stateRows[firstID] == nil || stateRows[firstID].State.Reps != 1 {
		t.Fatalf("user_card_fsrs reps=%v, want 1", stateRows[firstID])
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
		body := fmt.Sprintf(`{"query":"mutation { handleSwipe(input: {cardId: \"%s\", cardgroupId: \"%s\", mode: %d}) { __typename ... on InputValidationError { field message } } }"}`, cardID, cgID, mode)
		resp := postGraphQL(t, ts.URL+"/query", body, tok)
		if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
			t.Fatalf("mode=%d: unexpected GraphQL errors: %v", mode, errs)
		}
		payload, _ := resp["data"].(map[string]any)["handleSwipe"].(map[string]any)
		if payload == nil {
			t.Fatalf("mode=%d: expected data.handleSwipe, got nil; resp=%v", mode, resp)
		}
		if payload["__typename"] != "InputValidationError" {
			t.Fatalf("mode=%d: expected InputValidationError, got %v; resp=%v", mode, payload["__typename"], resp)
		}
		if payload["field"] != "mode" {
			t.Fatalf("mode=%d: expected field=mode, got %q; resp=%v", mode, payload["field"], resp)
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
	body := fmt.Sprintf(`{"query":"mutation { handleSwipe(input: {cardId: \"%s\", cardgroupId: \"%s\", mode: 4}) { __typename } }"}`, cardID, cgID)
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
		return fmt.Sprintf(`{"query":"mutation { handleSwipe(input: {cardId: \"%s\", cardgroupId: \"%s\", mode: 4}) { __typename ... on InputValidationError { field message } } }"}`, cardID, ownedCgID)
	}
	mismatchResp := postGraphQL(t, ts.URL+"/query", bodyForCard(otherCardID), tok)
	missingResp := postGraphQL(t, ts.URL+"/query", bodyForCard(missingCardID), tok)

	extractPayload := func(name string, resp map[string]any) map[string]any {
		if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
			t.Fatalf("%s: unexpected GraphQL errors: %v", name, errs)
		}
		payload, _ := resp["data"].(map[string]any)["handleSwipe"].(map[string]any)
		if payload == nil {
			t.Fatalf("%s: expected data.handleSwipe, got nil; resp=%v", name, resp)
		}
		if payload["__typename"] != "InputValidationError" {
			t.Fatalf("%s: expected InputValidationError, got %v; resp=%v", name, payload["__typename"], resp)
		}
		if payload["field"] != "cardId" {
			t.Fatalf("%s: expected field=cardId, got %q; resp=%v", name, payload["field"], resp)
		}
		return payload
	}
	mismatchPayload := extractPayload("mismatch", mismatchResp)
	missingPayload := extractPayload("missing", missingResp)
	if mismatchPayload["message"] != missingPayload["message"] {
		t.Fatalf("mismatched-cardgroup and missing-card errors differ: mismatch=%q missing=%q",
			mismatchPayload["message"], missingPayload["message"])
	}
}

type failingSwipeRepo struct{ err error }

func (f failingSwipeRepo) CreateTx(context.Context, *gorm.DB, *domain.SwipeRecord) error {
	return f.err
}

func (f failingSwipeRepo) ListRecentByUser(context.Context, string, int) ([]*domain.SwipeRecord, error) {
	return nil, f.err
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
	userCardFSRSRepo := repository.NewUserCardFSRSRepository(db.GORM)
	logger := slog.New(slog.DiscardHandler)
	uc := usecase.NewSwipeUsecase(
		db.GORM,
		cardRepo,
		cardgroupRepo,
		failingSwipeRepo{err: errors.New("forced swipe insert failure")},
		service.NewFSRSScheduler(),
		userCardFSRSRepo,
		logger,
	)

	_, err := uc.HandleSwipe(auth.ContextWithUser(ctx, &auth.AuthUser{Sub: sub}), usecase.HandleSwipeInput{
		CardID:      cardID,
		CardgroupID: cgID,
		Mode:        4,
	})
	if err == nil {
		t.Fatal("expected forced error, got nil")
	}
	stateRows, err := userCardFSRSRepo.FindByUserAndCardIDs(ctx, sub, []string{cardID})
	if err != nil {
		t.Fatalf("find user_card_fsrs after: %v", err)
	}
	if len(stateRows) != 0 {
		t.Fatalf("user_card_fsrs row created despite rollback: %+v", stateRows)
	}
	var swipeCount int64
	if err := db.GORM.WithContext(ctx).Table("swipe_records").Where("card_id = ?", cardID).Count(&swipeCount).Error; err != nil {
		t.Fatalf("count swipe_records: %v", err)
	}
	if swipeCount != 0 {
		t.Fatalf("swipe_records count=%d, want 0", swipeCount)
	}
}

// panicRoleRepo is an embed base that satisfies repository.RoleRepository
// (CRUD only, 7 methods) with every method panicking. Concrete stubs embed
// this and override only the methods their test exercises; any unexpected
// call fails loudly.
type panicRoleRepo struct{}

func (panicRoleRepo) FindByID(_ context.Context, _ string) (*domain.Role, error) {
	panic("not used in this test")
}
func (panicRoleRepo) FindByName(_ context.Context, _ domain.RoleName) (*domain.Role, error) {
	panic("not used in this test")
}
func (panicRoleRepo) FindByIDs(_ context.Context, _ []string) (map[string]*domain.Role, error) {
	panic("not used in this test")
}
func (panicRoleRepo) FindByIDsTx(_ context.Context, _ *gorm.DB, _ []string) (map[string]*domain.Role, error) {
	panic("not used in this test")
}
func (panicRoleRepo) Create(_ context.Context, _ string) (*domain.Role, error) {
	panic("not used in this test")
}
func (panicRoleRepo) Update(_ context.Context, _, _ string) (*domain.Role, error) {
	panic("not used in this test")
}
func (panicRoleRepo) Delete(_ context.Context, _ string) error {
	panic("not used in this test")
}
func (panicRoleRepo) ListAll(_ context.Context) ([]*domain.Role, error) {
	panic("not used in this test")
}

// panicUserRoleRepo is an embed base that satisfies repository.UserRoleRepository
// with every method panicking. Concrete stubs embed this and override only the
// methods their test exercises; any unexpected call fails loudly.
type panicUserRoleRepo struct{}

func (panicUserRoleRepo) HasRole(_ context.Context, _ string, _ domain.RoleName) (bool, error) {
	panic("not used in this test")
}
func (panicUserRoleRepo) AssignToUser(_ context.Context, _, _ string) error {
	panic("not used in this test")
}
func (panicUserRoleRepo) RevokeFromUser(_ context.Context, _, _ string) error {
	panic("not used in this test")
}
func (panicUserRoleRepo) SetUserRolesTx(_ context.Context, _ *gorm.DB, _ string, _ []string) error {
	panic("not used in this test")
}
func (panicUserRoleRepo) ListByUser(_ context.Context, _ string) ([]*domain.Role, error) {
	panic("not used in this test")
}
func (panicUserRoleRepo) ListByUserIDs(_ context.Context, _ []string) (map[string][]*domain.Role, error) {
	panic("not used in this test")
}
func (panicUserRoleRepo) CountAdmins(_ context.Context) (int64, error) {
	panic("not used in this test")
}

// failingCountRepo satisfies repository.UserRoleRepository with only CountAdmins
// implemented. All other methods panic via the embedded panicUserRoleRepo.
type failingCountRepo struct {
	panicUserRoleRepo
	err error
}

func (f failingCountRepo) CountAdmins(_ context.Context) (int64, error) {
	return 0, f.err
}

// findByNameRepo satisfies repository.RoleRepository with FindByName returning
// a configurable (role, error) pair. CountAdmins panics via the embedded
// panicRoleRepo — the non-empty-emails branch in bootstrapSuperUserPromoter
// never calls CountAdmins, only FindByName.
type findByNameRepo struct {
	panicRoleRepo
	role *domain.Role
	err  error
}

func (f findByNameRepo) FindByName(_ context.Context, _ domain.RoleName) (*domain.Role, error) {
	return f.role, f.err
}

// existingAdminRepo satisfies repository.UserRoleRepository with CountAdmins
// returning a fixed count. Used by deterministic tests for the Branch E path
// (admin role-holders already exist → no WARN emitted).
type existingAdminRepo struct {
	panicUserRoleRepo
	count int64
}

func (e existingAdminRepo) CountAdmins(_ context.Context) (int64, error) {
	return e.count, nil
}

// decodeLogRecords parses newline-delimited JSON log output from a bytes.Buffer
// and returns the decoded records. Fatals on malformed JSON.
func decodeLogRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
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

// TestBootstrapSuperUserPromoter_WarnsWhenNoEscapeHatch covers the WARN log
// path: SUPER_USER_EMAILS empty AND zero admin role-holders in the DB must
// emit a single WARN with admin_count=0.
func TestBootstrapSuperUserPromoter_WarnsWhenNoEscapeHatch(t *testing.T) {
	// Do not run in parallel — this test opens a DB connection and logs to a
	// local buffer; parallel would risk DB-state interference from other tests
	// that insert user_roles rows.

	ctx := t.Context()

	db, err := database.Open(ctx, database.Config{URL: testDBURL})
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	defer db.Close()

	roleRepo := repository.NewRoleRepository(db.GORM)
	userRoleRepo := repository.NewUserRoleRepository(db.GORM)

	// Confirm there are no admin role-holders in this test DB state so the
	// WARN branch fires. Other tests may insert user_roles rows but none grant
	// the admin role as part of their setup.
	adminCount, err := userRoleRepo.CountAdmins(ctx)
	if err != nil {
		t.Fatalf("CountAdmins: %v", err)
	}
	if adminCount != 0 {
		t.Skipf("pre-condition: %d admin role-holder(s) already exist; WARN branch would not fire — skipping", adminCount)
	}

	// Capture WARN-level log output via an inline JSON logger.
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	promoter, err := bootstrapSuperUserPromoter(ctx, logger, nil, roleRepo, userRoleRepo, "")
	if err != nil {
		t.Fatalf("bootstrapSuperUserPromoter returned unexpected error: %v", err)
	}
	if promoter == nil {
		t.Fatal("bootstrapSuperUserPromoter returned nil promoter")
	}

	records := decodeLogRecords(t, &buf)

	// Locate the expected WARN record.
	const wantMsg = "super-user bootstrap: no admin configured and no admin role-holder exists"
	var found map[string]any
	for _, rec := range records {
		if rec["msg"] == wantMsg {
			found = rec
			break
		}
	}
	if found == nil {
		t.Fatalf("expected WARN log line %q not found in output: %s", wantMsg, buf.String())
	}

	// Assert level is WARN.
	if got, _ := found["level"].(string); got != "WARN" {
		t.Errorf("want level=WARN, got %q", got)
	}

	// Assert admin_count is 0. slog.NewJSONHandler encodes numbers as JSON
	// numbers; json.Unmarshal into map[string]any decodes them as float64.
	if got, _ := found["admin_count"].(float64); got != 0 {
		t.Errorf("want admin_count=0, got %v", got)
	}
}

// TestBootstrapSuperUserPromoter_WarnOnCountError verifies that a DB failure
// in CountAdmins is non-fatal: bootstrapSuperUserPromoter returns a
// pass-through promoter and emits a structured WARN with error_chain.root.stack.
func TestBootstrapSuperUserPromoter_WarnOnCountError(t *testing.T) {
	ctx := t.Context()

	// Use a stub that returns an eris error so the structural error_chain
	// assertion (root.stack non-empty) can be verified. Using eris.New here
	// matches the convention in docs/backend/error-wrapping/test-error-chain-shape-not-presence.md
	// — stubs must use eris.New so the assertion exercises the same code path
	// production hits.
	stub := failingCountRepo{err: eris.New("repository: simulated DB failure")}

	// Capture log output at WARN+ level.
	var buf bytes.Buffer
	logger := slog.New(logging.NewContextHandler(
		slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}),
		func(_ context.Context) string { return "" },
	))

	promoter, err := bootstrapSuperUserPromoter(ctx, logger, nil, panicRoleRepo{}, stub, "")
	if err != nil {
		t.Fatalf("bootstrapSuperUserPromoter returned unexpected error: %v", err)
	}
	if promoter == nil {
		t.Fatal("bootstrapSuperUserPromoter returned nil promoter")
	}

	records := decodeLogRecords(t, &buf)

	// 1. The "admin count check failed" WARN must be present.
	const wantCountErrMsg = "super-user bootstrap: admin count check failed"
	var countErrRec map[string]any
	for _, rec := range records {
		if rec["msg"] == wantCountErrMsg {
			countErrRec = rec
			break
		}
	}
	if countErrRec == nil {
		t.Fatalf("expected WARN log line %q not found in output: %s", wantCountErrMsg, buf.String())
	}

	// 2. Assert level is WARN.
	if got, _ := countErrRec["level"].(string); got != "WARN" {
		t.Errorf("want level=WARN, got %q", got)
	}

	// 3. Assert structural error_chain shape (root.stack non-empty).
	// See docs/backend/error-wrapping/test-error-chain-shape-not-presence.md.
	chain, ok := countErrRec["error_chain"].(map[string]any)
	if !ok {
		t.Fatalf("error_chain is not a JSON object: %T", countErrRec["error_chain"])
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

	// 4. The "no admin configured" WARN must NOT appear — the count failed,
	//    so we never learned whether adminCount == 0.
	const wantNoEscapeMsg = "super-user bootstrap: no admin configured and no admin role-holder exists"
	for _, rec := range records {
		if rec["msg"] == wantNoEscapeMsg {
			t.Errorf("unexpected log line %q: should only appear when count succeeds with 0", wantNoEscapeMsg)
		}
	}
}

// userRoleStub satisfies repository.UserRoleRepository. HasRole always returns
// (false, nil) — sufficient for constructing auth.NewService in tests that only
// need to verify promoter construction, not per-request role checks.
type userRoleStub struct{ panicUserRoleRepo }

func (userRoleStub) HasRole(_ context.Context, _ string, _ domain.RoleName) (bool, error) {
	return false, nil
}

// TestBootstrapSuperUserPromoter_FindByNameError covers Branch A: when
// SUPER_USER_EMAILS is non-empty but the admin role lookup (FindByName) fails,
// bootstrapSuperUserPromoter must propagate the error and return nil.
func TestBootstrapSuperUserPromoter_FindByNameError(t *testing.T) {
	ctx := t.Context()

	roleErr := eris.New("repository: simulated FindByName failure")
	stub := findByNameRepo{err: roleErr}

	authSvc := auth.NewService(userRoleStub{})

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	promoter, err := bootstrapSuperUserPromoter(ctx, logger, authSvc, stub, panicUserRoleRepo{}, "you@example.com")

	if err == nil {
		t.Fatal("expected non-nil error when FindByName fails, got nil")
	}
	if !errors.Is(err, roleErr) {
		t.Errorf("expected wrapped sentinel errors.Is(err, roleErr) to be true; err=%v", err)
	}
	if promoter != nil {
		t.Errorf("expected nil promoter on error, got %v", promoter)
	}

	// No log output is expected for this fatal-error path.
	records := decodeLogRecords(t, &buf)
	for _, rec := range records {
		if rec["level"] == "WARN" {
			t.Errorf("unexpected WARN log on FindByName-error path: %v", rec)
		}
	}
}

// TestBootstrapSuperUserPromoter_NonEmptyEmailsHappyPath covers Branch B:
// when SUPER_USER_EMAILS is non-empty and FindByName succeeds, the function
// must return a non-nil active promoter and emit a single INFO log with
// msg="super-user bootstrap enabled" and email_count matching the parsed set.
func TestBootstrapSuperUserPromoter_NonEmptyEmailsHappyPath(t *testing.T) {
	ctx := t.Context()

	adminRole := &domain.Role{ID: "role-uuid-123", Name: "admin"}
	stub := findByNameRepo{role: adminRole}

	authSvc := auth.NewService(userRoleStub{})

	// Capture all log output so we can assert INFO and absence of WARN.
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	const emailsEnv = "alice@example.com,bob@example.com"
	promoter, err := bootstrapSuperUserPromoter(ctx, logger, authSvc, stub, panicUserRoleRepo{}, emailsEnv)

	if err != nil {
		t.Fatalf("bootstrapSuperUserPromoter returned unexpected error: %v", err)
	}
	if promoter == nil {
		t.Fatal("expected non-nil promoter, got nil")
	}

	records := decodeLogRecords(t, &buf)

	// Locate the INFO "super-user bootstrap enabled" record.
	const wantMsg = "super-user bootstrap enabled"
	var infoRec map[string]any
	for _, rec := range records {
		if rec["msg"] == wantMsg {
			infoRec = rec
			break
		}
	}
	if infoRec == nil {
		t.Fatalf("expected INFO log line %q not found in output: %s", wantMsg, buf.String())
	}

	if got, _ := infoRec["level"].(string); got != "INFO" {
		t.Errorf("want level=INFO, got %q", got)
	}

	// ParseSuperUserSet("alice@example.com,bob@example.com") should produce 2 entries.
	wantCount := float64(len(auth.ParseSuperUserSet(emailsEnv)))
	if got, _ := infoRec["email_count"].(float64); got != wantCount {
		t.Errorf("want email_count=%.0f, got %.0f", wantCount, got)
	}

	// No WARN log must appear on the happy path.
	for _, rec := range records {
		if rec["level"] == "WARN" {
			t.Errorf("unexpected WARN log on happy path: %v", rec)
		}
	}
}

// panicQueryResolver is a minimal generated.QueryResolver whose Health method
// panics. All other methods are stubs that return zero values. It is used by
// TestGraphQL_PanicRecovery_ReturnsINTERNAL to drive the SetRecoverFunc path
// without requiring a database or auth middleware.
type panicQueryResolver struct{}

func (panicQueryResolver) Health(_ context.Context) (string, error) {
	panic("deliberate panic in Health resolver for panic-recovery test")
}
func (panicQueryResolver) Me(_ context.Context) (*model.User, error) { return nil, nil }
func (panicQueryResolver) Cardgroup(_ context.Context, _ string) (*model.Cardgroup, error) {
	return nil, nil
}
func (panicQueryResolver) MyCardgroupsConnection(_ context.Context, _ *int, _ *string, _ *int, _ *string, _ *string, _ *model.CardgroupOrderBy, _ *model.SortOrder) (*model.CardgroupConnection, error) {
	return nil, nil
}
func (panicQueryResolver) Card(_ context.Context, _ string) (*model.Card, error) { return nil, nil }
func (panicQueryResolver) LearnNextDueCards(_ context.Context, _ string, _ *int) ([]*model.Card, error) {
	return nil, nil
}
func (panicQueryResolver) PracticeTodaysCards(_ context.Context, _ string, _ *int) ([]*model.Card, error) {
	return nil, nil
}
func (panicQueryResolver) CardsByCardgroupConnection(_ context.Context, _ string, _ *int, _ *string, _ *int, _ *string, _ *string, _ *model.CardOrderBy, _ *model.SortOrder) (*model.CardConnection, error) {
	return nil, nil
}
func (panicQueryResolver) ValidateCardImport(_ context.Context, _ model.ValidateCardImportInput) (*model.CardImportValidationResult, error) {
	return nil, nil
}
func (panicQueryResolver) Users(_ context.Context, _ *int, _ *string, _ *int, _ *string, _ *string) (*model.UserConnection, error) {
	return nil, nil
}
func (panicQueryResolver) AdminUser(_ context.Context, _ string) (*model.User, error) {
	return nil, nil
}
func (panicQueryResolver) Roles(_ context.Context) ([]*model.Role, error)        { return nil, nil }
func (panicQueryResolver) Role(_ context.Context, _ string) (*model.Role, error) { return nil, nil }
func (panicQueryResolver) MasterCatalog(_ context.Context, _ *int, _ *string, _ *int, _ *string, _ *string, _ *model.MasterCatalogOrderBy, _ *model.SortOrder) (*model.MasterCatalogConnection, error) {
	return nil, nil
}
func (panicQueryResolver) AdminMasters(_ context.Context, _ *int, _ *string, _ *int, _ *string, _ *string, _ *model.MasterCatalogOrderBy, _ *model.SortOrder) (*model.MasterCatalogConnection, error) {
	panic("not implemented")
}

// panicResolverRoot is a generated.ResolverRoot whose Query resolver panics on
// Health. All other sub-resolvers forward to the real resolver with nil deps
// (which is safe because they are never called in the panic-recovery test).
type panicResolverRoot struct {
	inner *resolver.Resolver
}

func newPanicResolverRoot() *panicResolverRoot {
	return &panicResolverRoot{inner: resolver.NewResolver(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)}
}

func (p *panicResolverRoot) Card() generated.CardResolver           { return p.inner.Card() }
func (p *panicResolverRoot) Cardgroup() generated.CardgroupResolver { return p.inner.Cardgroup() }
func (p *panicResolverRoot) Mutation() generated.MutationResolver   { return p.inner.Mutation() }
func (p *panicResolverRoot) Query() generated.QueryResolver         { return panicQueryResolver{} }
func (p *panicResolverRoot) User() generated.UserResolver           { return p.inner.User() }

// newPanicGraphQLServer builds a gqlgen handler.Server wired with
// panicResolverRoot so that { health } panics. The server uses the shared
// gqlerr.RecoverFunc (same as newGraphQLServer) so recovery behaviour
// stays in sync with production. Used exclusively by
// TestGraphQL_PanicRecovery_ReturnsINTERNAL.
func newPanicGraphQLServer() *handler.Server {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: newPanicResolverRoot()}))
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{
		ResponseHeaders: http.Header{
			"Content-Type": []string{"application/graphql-response+json; charset=utf-8"},
		},
	})
	srv.SetRecoverFunc(gqlerr.RecoverFunc)
	return srv
}

// TestGraphQL_PanicRecovery_ReturnsINTERNAL verifies that a resolver panic is
// caught by SetRecoverFunc and returned to the client as a well-formed GraphQL
// envelope with errors[0].extensions.code == "INTERNAL". The process must not
// crash.
//
// Strategy: newPanicGraphQLServer wires a panicResolverRoot whose Health method
// panics. The gqlgen handler's SetRecoverFunc must catch it and return an
// INTERNAL error. The handler is used directly (no Echo auth layer) to keep the
// fixture minimal — matching the newIntrospectionTestServer pattern.
func TestGraphQL_PanicRecovery_ReturnsINTERNAL(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(newPanicGraphQLServer())
	t.Cleanup(ts.Close)

	raw := postRaw(t, ts.URL, `{"query":"{ health }"}`)

	var payload struct {
		Data   map[string]any `json:"data"`
		Errors []struct {
			Message    string         `json:"message"`
			Extensions map[string]any `json:"extensions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode body: %v (body=%q)", err, raw)
	}
	if len(payload.Errors) == 0 {
		t.Fatalf("expected errors from panic recovery, got none; body=%q", raw)
	}
	code, _ := payload.Errors[0].Extensions["code"].(string)
	if code != "INTERNAL" {
		t.Fatalf("expected extensions.code=INTERNAL, got %q; body=%q", code, raw)
	}
	msg := payload.Errors[0].Message
	if msg == "" {
		t.Fatalf("expected non-empty error message; body=%q", raw)
	}
}

// TestGraphQL_ContentType_POST verifies that a GraphQL POST response carries
// the Content-Type header set by the transport.POST ResponseHeaders override.
func TestGraphQL_ContentType_POST(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/query", strings.NewReader(`{"query":"{ health }"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()
	_, _ = io.ReadAll(res.Body)

	ct := res.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/graphql-response+json") {
		t.Errorf("Content-Type = %q, want it to contain %q", ct, "application/graphql-response+json")
	}
}

// TestBootstrapSuperUserPromoter_NoWarnWhenAdminExists covers Branch E:
// when SUPER_USER_EMAILS is empty AND at least one admin role-holder already
// exists, bootstrapSuperUserPromoter must return a pass-through promoter
// without emitting any log lines.
func TestBootstrapSuperUserPromoter_NoWarnWhenAdminExists(t *testing.T) {
	ctx := t.Context()

	// Stub returns count=1 (at least one admin already exists).
	stub := existingAdminRepo{count: 1}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	promoter, err := bootstrapSuperUserPromoter(ctx, logger, nil, panicRoleRepo{}, stub, "")

	if err != nil {
		t.Fatalf("bootstrapSuperUserPromoter returned unexpected error: %v", err)
	}
	if promoter == nil {
		t.Fatal("expected non-nil pass-through promoter, got nil")
	}

	records := decodeLogRecords(t, &buf)
	if len(records) != 0 {
		t.Errorf("expected no log output when admin already exists, got: %s", buf.String())
	}
}

func TestServerConfigFromEnv(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		want    time.Duration
		wantLog string
	}{
		{"empty uses default", "", defaultShutdownTimeout, ""},
		{"valid positive", "5s", 5 * time.Second, ""},
		{"invalid string uses default", "bad", defaultShutdownTimeout, "invalid SHUTDOWN_TIMEOUT"},
		{"zero uses default", "0s", defaultShutdownTimeout, "non-positive SHUTDOWN_TIMEOUT"},
		{"negative uses default", "-1s", defaultShutdownTimeout, "non-positive SHUTDOWN_TIMEOUT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SHUTDOWN_TIMEOUT", tc.env)
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			cfg := serverConfigFromEnv(logger)
			if cfg.shutdownTimeout != tc.want {
				t.Errorf("shutdownTimeout = %v, want %v", cfg.shutdownTimeout, tc.want)
			}
			if tc.wantLog != "" && !strings.Contains(buf.String(), tc.wantLog) {
				t.Errorf("expected log to contain %q, got %q", tc.wantLog, buf.String())
			}
			if tc.wantLog == "" && buf.Len() > 0 {
				t.Errorf("expected no log output, got %q", buf.String())
			}
		})
	}
}
