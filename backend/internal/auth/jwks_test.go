package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigFromEnv_MissingJWKSURL(t *testing.T) {
	t.Setenv("SUPABASE_JWKS_URL", "")
	t.Setenv("SUPABASE_JWT_AUDIENCE", "authenticated")
	t.Setenv("SUPABASE_JWT_ISSUER", "http://127.0.0.1:54321/auth/v1")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected error for missing SUPABASE_JWKS_URL")
	}
}

func TestConfigFromEnv_MissingAudience(t *testing.T) {
	t.Setenv("SUPABASE_JWKS_URL", "http://example/jwks")
	t.Setenv("SUPABASE_JWT_AUDIENCE", "")
	t.Setenv("SUPABASE_JWT_ISSUER", "http://127.0.0.1:54321/auth/v1")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected error for missing SUPABASE_JWT_AUDIENCE")
	}
}

func TestConfigFromEnv_MissingIssuer(t *testing.T) {
	t.Setenv("SUPABASE_JWKS_URL", "http://example/jwks")
	t.Setenv("SUPABASE_JWT_AUDIENCE", "authenticated")
	t.Setenv("SUPABASE_JWT_ISSUER", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected error for missing SUPABASE_JWT_ISSUER")
	}
}

func TestConfigFromEnv_AllSet(t *testing.T) {
	t.Setenv("SUPABASE_JWKS_URL", "http://example/jwks")
	t.Setenv("SUPABASE_JWT_AUDIENCE", "authenticated")
	t.Setenv("SUPABASE_JWT_ISSUER", "http://127.0.0.1:54321/auth/v1")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.JWKSURL == "" || cfg.Audience == "" || cfg.Issuer == "" {
		t.Fatalf("unexpected empty field in %+v", cfg)
	}
}

func TestNewJWKSKeyfunc_FetchesJWKS(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
	}))
	t.Cleanup(ts.Close)

	kf, err := NewJWKSKeyfunc(context.Background(), Config{
		JWKSURL: ts.URL, Audience: "authenticated", Issuer: "http://iss",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if kf == nil {
		t.Fatal("expected non-nil keyfunc")
	}
}

func TestNewJWKSKeyfunc_FetchFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	_, err := NewJWKSKeyfunc(context.Background(), Config{
		JWKSURL: ts.URL, Audience: "authenticated", Issuer: "http://iss",
	})
	if err == nil {
		t.Fatal("expected error for JWKS fetch failure at boot")
	}
}
