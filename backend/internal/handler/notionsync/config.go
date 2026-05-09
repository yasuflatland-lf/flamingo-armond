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

// ConfigFromEnv reads the five required NOTION_* env vars and returns an
// EnvConfig. All values are required; whitespace-only is treated as missing.
func ConfigFromEnv() (EnvConfig, error) {
	required := map[string]string{
		"NOTION_TOKEN":                 os.Getenv("NOTION_TOKEN"),
		"NOTION_PAGE_IDS":              os.Getenv("NOTION_PAGE_IDS"),
		"NOTION_TARGET_OWNER_ID":       os.Getenv("NOTION_TARGET_OWNER_ID"),
		"NOTION_TARGET_CARDGROUP_NAME": os.Getenv("NOTION_TARGET_CARDGROUP_NAME"),
		"NOTION_SYNC_TOKEN":            os.Getenv("NOTION_SYNC_TOKEN"),
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return EnvConfig{}, eris.Errorf("notionsync: %s env var is required", key)
		}
	}
	pageIDs := splitCSV(required["NOTION_PAGE_IDS"])
	if len(pageIDs) == 0 {
		return EnvConfig{}, eris.New("notionsync: NOTION_PAGE_IDS must contain at least one page id")
	}
	return EnvConfig{
		NotionToken: strings.TrimSpace(required["NOTION_TOKEN"]),
		HandlerConfig: Config{
			Token:         strings.TrimSpace(required["NOTION_SYNC_TOKEN"]),
			PageIDs:       pageIDs,
			OwnerID:       strings.TrimSpace(required["NOTION_TARGET_OWNER_ID"]),
			CardgroupName: strings.TrimSpace(required["NOTION_TARGET_CARDGROUP_NAME"]),
		},
	}, nil
}

// splitCSV splits raw on commas, trims surrounding whitespace from each part,
// and drops empty parts. Used by ConfigFromEnv for NOTION_PAGE_IDS.
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
