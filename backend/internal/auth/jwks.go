package auth

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/MicahParks/keyfunc/v3"
)

// Config holds the required configuration for JWT verification.
type Config struct {
	JWKSURL  string
	Audience string
	Issuer   string
}

// ConfigFromEnv reads the three required auth environment variables and returns
// a Config. Returns an error if any variable is unset or empty.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		JWKSURL:  os.Getenv("SUPABASE_JWKS_URL"),
		Audience: os.Getenv("SUPABASE_JWT_AUDIENCE"),
		Issuer:   os.Getenv("SUPABASE_JWT_ISSUER"),
	}
	if cfg.JWKSURL == "" {
		return cfg, errors.New("auth: SUPABASE_JWKS_URL is required")
	}
	if cfg.Audience == "" {
		return cfg, errors.New("auth: SUPABASE_JWT_AUDIENCE is required")
	}
	if cfg.Issuer == "" {
		return cfg, errors.New("auth: SUPABASE_JWT_ISSUER is required")
	}
	return cfg, nil
}

// NewJWKSKeyfunc initializes a keyfunc that fetches JWKS from cfg.JWKSURL and
// refreshes in the background. The refresh goroutine stops when ctx is canceled.
// Returns an error if the initial fetch fails — silent failure is disallowed.
func NewJWKSKeyfunc(ctx context.Context, cfg Config) (keyfunc.Keyfunc, error) {
	failOnFirstFetch := false
	override := keyfunc.Override{
		NoErrorReturnFirstHTTPReq: &failOnFirstFetch,
	}
	kf, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{cfg.JWKSURL}, override)
	if err != nil {
		return nil, fmt.Errorf("auth: JWKS initial fetch failed: %w", err)
	}
	return kf, nil
}
