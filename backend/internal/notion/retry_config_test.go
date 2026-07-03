package notion

import (
	"strings"
	"testing"
	"time"
)

func TestRetryConfigFromEnv(t *testing.T) {
	tests := []struct {
		name           string
		maxAttempts    string // value for NOTION_MAX_ATTEMPTS; "" means unset
		maxElapsed     string // value for NOTION_MAX_ELAPSED;  "" means unset
		wantAttempts   int
		wantElapsed    time.Duration
		wantErrContain string // non-empty → expect an error whose message contains this
	}{
		{
			name:         "defaults when no env set",
			wantAttempts: 5,
			wantElapsed:  20 * time.Second,
		},
		{
			name:         "valid override of both vars",
			maxAttempts:  "10",
			maxElapsed:   "5m",
			wantAttempts: 10,
			wantElapsed:  5 * time.Minute,
		},
		// --- NOTION_MAX_ATTEMPTS invalid values ---
		{
			name:           "NOTION_MAX_ATTEMPTS: non-numeric",
			maxAttempts:    "abc",
			wantErrContain: "NOTION_MAX_ATTEMPTS",
		},
		{
			name:           "NOTION_MAX_ATTEMPTS: zero",
			maxAttempts:    "0",
			wantErrContain: "NOTION_MAX_ATTEMPTS",
		},
		{
			name:           "NOTION_MAX_ATTEMPTS: negative",
			maxAttempts:    "-1",
			wantErrContain: "NOTION_MAX_ATTEMPTS",
		},
		// --- NOTION_MAX_ELAPSED invalid values ---
		{
			name:           "NOTION_MAX_ELAPSED: non-duration",
			maxElapsed:     "abc",
			wantErrContain: "NOTION_MAX_ELAPSED",
		},
		{
			name:           "NOTION_MAX_ELAPSED: zero string",
			maxElapsed:     "0",
			wantErrContain: "NOTION_MAX_ELAPSED",
		},
		{
			name:           "NOTION_MAX_ELAPSED: negative",
			maxElapsed:     "-1s",
			wantErrContain: "NOTION_MAX_ELAPSED",
		},
		// --- whitespace-only values treated as unset ---
		{
			name:         "whitespace-only NOTION_MAX_ATTEMPTS treated as unset",
			maxAttempts:  "   ",
			wantAttempts: 5,
			wantElapsed:  20 * time.Second,
		},
		{
			name:         "whitespace-only NOTION_MAX_ELAPSED treated as unset",
			maxElapsed:   "   ",
			wantAttempts: 5,
			wantElapsed:  20 * time.Second,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// t.Setenv requires sequential subtests — no t.Parallel() here.

			// Set both vars unconditionally (to "" for the "unset" cases) so an
			// ambient NOTION_MAX_* — e.g. exported from a local .env by mise —
			// cannot leak into cases that assert the built-in defaults. An empty
			// or whitespace-only value is treated as unset by RetryConfigFromEnv
			// (TrimSpace == ""), and t.Setenv still restores the prior value on
			// cleanup.
			t.Setenv("NOTION_MAX_ATTEMPTS", tc.maxAttempts)
			t.Setenv("NOTION_MAX_ELAPSED", tc.maxElapsed)

			cfg, err := RetryConfigFromEnv()

			if tc.wantErrContain != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrContain)
				}
				if !strings.Contains(err.Error(), tc.wantErrContain) {
					t.Fatalf("expected error containing %q, got: %v", tc.wantErrContain, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.MaxAttempts != tc.wantAttempts {
				t.Errorf("MaxAttempts: want %d, got %d", tc.wantAttempts, cfg.MaxAttempts)
			}
			if cfg.MaxElapsed != tc.wantElapsed {
				t.Errorf("MaxElapsed: want %v, got %v", tc.wantElapsed, cfg.MaxElapsed)
			}
			// Caller-supplied fields must remain zero.
			if cfg.Transport != nil {
				t.Errorf("Transport: want nil, got %v", cfg.Transport)
			}
			if cfg.Logger != nil {
				t.Errorf("Logger: want nil, got non-nil")
			}
			if cfg.Sleep != nil {
				t.Errorf("Sleep: want nil, got non-nil")
			}
		})
	}
}
