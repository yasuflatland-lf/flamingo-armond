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

// Validate returns a non-nil error if any required Config field is empty.
func (c Config) Validate() error {
	if c.JWKSURL == "" {
		return errors.New("auth: SUPABASE_JWKS_URL is required")
	}
	if c.Audience == "" {
		return errors.New("auth: SUPABASE_JWT_AUDIENCE is required")
	}
	if c.Issuer == "" {
		return errors.New("auth: SUPABASE_JWT_ISSUER is required")
	}
	return nil
}

// ConfigFromEnv reads the three required auth environment variables and returns
// a Config. Returns an error if any variable is unset or empty.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		JWKSURL:  os.Getenv("SUPABASE_JWKS_URL"),
		Audience: os.Getenv("SUPABASE_JWT_AUDIENCE"),
		Issuer:   os.Getenv("SUPABASE_JWT_ISSUER"),
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// NewJWKSKeyfunc initializes a keyfunc that fetches JWKS from cfg.JWKSURL and
// refreshes in the background. The refresh goroutine stops when ctx is canceled.
// Returns an error if the initial fetch fails — silent failure is disallowed.
func NewJWKSKeyfunc(ctx context.Context, cfg Config) (keyfunc.Keyfunc, error) {
	// keyfunc.NewDefaultCtx (and a zero-value Override) defaults
	// NoErrorReturnFirstHTTPReq=true, which silently swallows the initial fetch
	// failure. Set it explicitly to false so a misconfigured JWKS URL surfaces at boot.
	noSwallowFirstFetchErr := false
	kf, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{cfg.JWKSURL}, keyfunc.Override{
		NoErrorReturnFirstHTTPReq: &noSwallowFirstFetchErr,
	})
	if err != nil {
		return nil, fmt.Errorf("auth: JWKS initial fetch failed: %w", err)
	}
	return kf, nil
}
