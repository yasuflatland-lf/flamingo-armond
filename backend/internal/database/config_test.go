package database_test

import (
	"strings"
	"testing"
	"time"

	"backend/internal/database"
)

func TestConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("SUPABASE_DB_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("DB_MAX_CONNS", "")
	t.Setenv("DB_MIN_CONNS", "")
	t.Setenv("DB_MAX_CONN_LIFETIME", "")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "")

	cfg, err := database.ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.URL != "postgres://user:pass@localhost:5432/db" {
		t.Errorf("URL: got %q", cfg.URL)
	}
	if cfg.MaxConns != 10 {
		t.Errorf("MaxConns default: got %d, want 10", cfg.MaxConns)
	}
	if cfg.MinConns != 0 {
		t.Errorf("MinConns default: got %d, want 0", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != 30*time.Minute {
		t.Errorf("MaxConnLifetime default: got %s, want 30m", cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdleTime != 5*time.Minute {
		t.Errorf("MaxConnIdleTime default: got %s, want 5m", cfg.MaxConnIdleTime)
	}
}

func TestConfigFromEnv_Overrides(t *testing.T) {
	t.Setenv("SUPABASE_DB_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("DB_MAX_CONNS", "25")
	t.Setenv("DB_MIN_CONNS", "5")
	t.Setenv("DB_MAX_CONN_LIFETIME", "1h")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "10m")

	cfg, err := database.ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxConns != 25 {
		t.Errorf("MaxConns: got %d, want 25", cfg.MaxConns)
	}
	if cfg.MinConns != 5 {
		t.Errorf("MinConns: got %d, want 5", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != time.Hour {
		t.Errorf("MaxConnLifetime: got %s, want 1h", cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdleTime != 10*time.Minute {
		t.Errorf("MaxConnIdleTime: got %s, want 10m", cfg.MaxConnIdleTime)
	}
}

func TestConfigFromEnv_RejectsInvalid(t *testing.T) {
	base := "postgres://user:pass@localhost:5432/db"

	cases := []struct {
		name    string
		env     string
		value   string
		wantSub string
	}{
		{"DB_MAX_CONNS non-numeric", "DB_MAX_CONNS", "10x", "DB_MAX_CONNS"},
		{"DB_MAX_CONNS zero", "DB_MAX_CONNS", "0", "DB_MAX_CONNS"},
		{"DB_MAX_CONNS negative", "DB_MAX_CONNS", "-1", "DB_MAX_CONNS"},
		{"DB_MIN_CONNS non-numeric", "DB_MIN_CONNS", "abc", "DB_MIN_CONNS"},
		{"DB_MIN_CONNS negative", "DB_MIN_CONNS", "-1", "DB_MIN_CONNS"},
		{"DB_MAX_CONN_LIFETIME invalid", "DB_MAX_CONN_LIFETIME", "thirty-minutes", "DB_MAX_CONN_LIFETIME"},
		{"DB_MAX_CONN_IDLE_TIME invalid", "DB_MAX_CONN_IDLE_TIME", "5x", "DB_MAX_CONN_IDLE_TIME"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SUPABASE_DB_URL", base)
			t.Setenv("DB_MAX_CONNS", "")
			t.Setenv("DB_MIN_CONNS", "")
			t.Setenv("DB_MAX_CONN_LIFETIME", "")
			t.Setenv("DB_MAX_CONN_IDLE_TIME", "")
			t.Setenv(tc.env, tc.value)

			_, err := database.ConfigFromEnv()
			if err == nil {
				t.Fatalf("expected error for %s=%q, got nil", tc.env, tc.value)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestConfigFromEnv_MissingURL(t *testing.T) {
	t.Setenv("SUPABASE_DB_URL", "")
	t.Setenv("DB_MAX_CONNS", "")
	t.Setenv("DB_MIN_CONNS", "")
	t.Setenv("DB_MAX_CONN_LIFETIME", "")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "")

	_, err := database.ConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for missing URL, got nil")
	}
	if !strings.Contains(err.Error(), "SUPABASE_DB_URL") {
		t.Errorf("error %q does not mention SUPABASE_DB_URL", err.Error())
	}
}

func TestValidate_RejectsBadRanges(t *testing.T) {
	cases := []struct {
		name    string
		cfg     database.Config
		wantSub string
	}{
		{
			name:    "MinConns exceeds MaxConns",
			cfg:     database.Config{URL: "postgres://x", MaxConns: 5, MinConns: 10},
			wantSub: "MinConns",
		},
		{
			name:    "negative MaxConns",
			cfg:     database.Config{URL: "postgres://x", MaxConns: -1},
			wantSub: "MaxConns",
		},
		{
			name:    "negative MinConns",
			cfg:     database.Config{URL: "postgres://x", MinConns: -1},
			wantSub: "MinConns",
		},
		{
			name:    "negative MaxConnLifetime",
			cfg:     database.Config{URL: "postgres://x", MaxConnLifetime: -1 * time.Second},
			wantSub: "MaxConnLifetime",
		},
		{
			name:    "negative MaxConnIdleTime",
			cfg:     database.Config{URL: "postgres://x", MaxConnIdleTime: -1 * time.Second},
			wantSub: "MaxConnIdleTime",
		},
		{
			name:    "empty URL",
			cfg:     database.Config{},
			wantSub: "SUPABASE_DB_URL",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.wantSub)
			}
		})
	}
}
