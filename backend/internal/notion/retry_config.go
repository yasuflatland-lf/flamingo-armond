package notion

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rotisserie/eris"
)

// RetryConfigFromEnv reads NOTION_MAX_ATTEMPTS and NOTION_MAX_ELAPSED from the
// environment and returns a RetryConfig with those two fields populated.
// Defaults: MaxAttempts=5, MaxElapsed=defaultMaxElapsed (20s, below the server's
// 30s WriteTimeout). Other RetryConfig fields (Transport, Logger, Sleep) are
// left zero for the caller to fill.
func RetryConfigFromEnv() (RetryConfig, error) {
	cfg := RetryConfig{
		MaxAttempts: 5,
		MaxElapsed:  defaultMaxElapsed,
	}
	if raw := strings.TrimSpace(os.Getenv("NOTION_MAX_ATTEMPTS")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return RetryConfig{}, eris.New("notion: NOTION_MAX_ATTEMPTS must be a positive integer")
		}
		cfg.MaxAttempts = n
	}
	if raw := strings.TrimSpace(os.Getenv("NOTION_MAX_ELAPSED")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			return RetryConfig{}, eris.New("notion: NOTION_MAX_ELAPSED must be a positive duration")
		}
		cfg.MaxElapsed = d
	}
	return cfg, nil
}
