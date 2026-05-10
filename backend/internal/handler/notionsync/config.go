package notionsync

import (
	"os"
	"strings"

	"github.com/rotisserie/eris"
)

// EnvConfig is the wire between os.Getenv and the constructors that build the
// Notion sync handler. NotionToken feeds notion.NewFetcher; HandlerConfig feeds
// notionsync.New.
type EnvConfig struct {
	NotionToken   string
	HandlerConfig Config
}

// varOrder defines the deterministic iteration order for the five required
// NOTION_* env vars. The order is fixed so that OptionalConfigFromEnv returns
// a stable missing slice regardless of map-iteration randomness, and so that
// ConfigFromEnv always reports the first missing var in the same predictable
// position.
var varOrder = []string{
	"NOTION_TOKEN",
	"NOTION_PAGE_IDS",
	"NOTION_TARGET_OWNER_ID",
	"NOTION_TARGET_CARDGROUP_NAME",
	"NOTION_SYNC_TOKEN",
}

// OptionalConfigFromEnv reads the five NOTION_* env vars and reports which (if
// any) are missing or whitespace-only. When missing is empty, the returned
// EnvConfig is fully populated and the handler should be wired up. When
// missing is non-empty, the handler should be skipped and the caller should
// log the missing var names. A non-nil error indicates a malformed value
// (e.g. NOTION_PAGE_IDS contains only commas/whitespace) and should fail
// startup, not be skipped.
func OptionalConfigFromEnv() (cfg EnvConfig, missing []string, err error) {
	vals := make(map[string]string, len(varOrder))
	for _, key := range varOrder {
		v := strings.TrimSpace(os.Getenv(key))
		vals[key] = v
		if v == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return EnvConfig{}, missing, nil
	}

	pageIDs := splitCSV(vals["NOTION_PAGE_IDS"])
	if len(pageIDs) == 0 {
		return EnvConfig{}, nil, eris.New("notionsync: NOTION_PAGE_IDS must contain at least one page id")
	}

	return EnvConfig{
		NotionToken: vals["NOTION_TOKEN"],
		HandlerConfig: Config{
			Token:         vals["NOTION_SYNC_TOKEN"],
			PageIDs:       pageIDs,
			OwnerID:       vals["NOTION_TARGET_OWNER_ID"],
			CardgroupName: vals["NOTION_TARGET_CARDGROUP_NAME"],
		},
	}, nil, nil
}

// ConfigFromEnv reads the five required NOTION_* env vars and returns an
// EnvConfig. All values are required; whitespace-only is treated as missing.
// This is the strict variant: any missing var produces an error. Callers that
// want to skip Notion sync gracefully when env is incomplete should use
// OptionalConfigFromEnv instead.
func ConfigFromEnv() (EnvConfig, error) {
	cfg, missing, err := OptionalConfigFromEnv()
	if err != nil {
		return EnvConfig{}, err
	}
	if len(missing) > 0 {
		// Match legacy single-var error message (existing tests assert this format).
		return EnvConfig{}, eris.Errorf("notionsync: %s env var is required", missing[0])
	}
	return cfg, nil
}

// splitCSV splits raw on commas, trims each part, and drops empty entries.
func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
