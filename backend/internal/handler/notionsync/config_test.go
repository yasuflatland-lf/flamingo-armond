package notionsync

import (
	"reflect"
	"strings"
	"testing"
)

var notionEnvAllVars = map[string]string{
	"NOTION_TOKEN":                 "tok-123",
	"NOTION_PAGE_IDS":              "page-1,page-2,page-3",
	"NOTION_TARGET_OWNER_ID":       "owner-abc",
	"NOTION_TARGET_CARDGROUP_NAME": "My Cards",
	"NOTION_SYNC_TOKEN":            "sync-tok",
}

// setNotionEnv populates the five NOTION_* env vars from notionEnvAllVars,
// applying any overrides supplied by the test. Values are unset via
// t.Setenv(k, "") rather than os.Unsetenv so that t.Cleanup restores the
// prior process state automatically.
func setNotionEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	for k, v := range notionEnvAllVars {
		if ov, ok := overrides[k]; ok {
			t.Setenv(k, ov)
		} else {
			t.Setenv(k, v)
		}
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Run("happy path: all vars set, multi-page CSV", func(t *testing.T) {
		setNotionEnv(t, map[string]string{
			"NOTION_PAGE_IDS": "page-1, page-2 ,page-3",
		})
		cfg, err := ConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.NotionToken != "tok-123" {
			t.Errorf("NotionToken = %q, want %q", cfg.NotionToken, "tok-123")
		}
		if cfg.HandlerConfig.Token != "sync-tok" {
			t.Errorf("HandlerConfig.Token = %q, want %q", cfg.HandlerConfig.Token, "sync-tok")
		}
		wantPageIDs := []string{"page-1", "page-2", "page-3"}
		if len(cfg.HandlerConfig.PageIDs) != len(wantPageIDs) {
			t.Fatalf("PageIDs len = %d, want %d", len(cfg.HandlerConfig.PageIDs), len(wantPageIDs))
		}
		for i, want := range wantPageIDs {
			if cfg.HandlerConfig.PageIDs[i] != want {
				t.Errorf("PageIDs[%d] = %q, want %q", i, cfg.HandlerConfig.PageIDs[i], want)
			}
		}
		if cfg.HandlerConfig.OwnerID != "owner-abc" {
			t.Errorf("OwnerID = %q, want %q", cfg.HandlerConfig.OwnerID, "owner-abc")
		}
		if cfg.HandlerConfig.CardgroupName != "My Cards" {
			t.Errorf("CardgroupName = %q, want %q", cfg.HandlerConfig.CardgroupName, "My Cards")
		}
	})

	missingVarCases := []struct {
		name    string
		missing string
	}{
		{"missing NOTION_TOKEN", "NOTION_TOKEN"},
		{"missing NOTION_PAGE_IDS", "NOTION_PAGE_IDS"},
		{"missing NOTION_TARGET_OWNER_ID", "NOTION_TARGET_OWNER_ID"},
		{"missing NOTION_TARGET_CARDGROUP_NAME", "NOTION_TARGET_CARDGROUP_NAME"},
		{"missing NOTION_SYNC_TOKEN", "NOTION_SYNC_TOKEN"},
	}
	for _, tc := range missingVarCases {
		t.Run(tc.name, func(t *testing.T) {
			setNotionEnv(t, map[string]string{tc.missing: ""})
			_, err := ConfigFromEnv()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.missing) {
				t.Errorf("error %q does not mention missing var %q", err.Error(), tc.missing)
			}
		})
	}

	t.Run("whitespace-only NOTION_TOKEN treated as missing", func(t *testing.T) {
		setNotionEnv(t, map[string]string{"NOTION_TOKEN": "   "})
		_, err := ConfigFromEnv()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "NOTION_TOKEN") {
			t.Errorf("error %q does not mention NOTION_TOKEN", err.Error())
		}
		if !strings.Contains(err.Error(), "is required") {
			t.Errorf("error %q does not contain 'is required'", err.Error())
		}
	})

	t.Run("NOTION_PAGE_IDS is commas and whitespace only", func(t *testing.T) {
		setNotionEnv(t, map[string]string{"NOTION_PAGE_IDS": ",,, "})
		_, err := ConfigFromEnv()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must contain at least one page id") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})

	t.Run("NOTION_TARGET_OWNER_ID with whitespace is trimmed", func(t *testing.T) {
		setNotionEnv(t, map[string]string{"NOTION_TARGET_OWNER_ID": "  owner-xyz  "})
		cfg, err := ConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.HandlerConfig.OwnerID != "owner-xyz" {
			t.Errorf("OwnerID = %q, want %q", cfg.HandlerConfig.OwnerID, "owner-xyz")
		}
	})
}

func TestOptionalConfigFromEnv(t *testing.T) {
	t.Run("all vars set: cfg fully populated, missing nil, err nil", func(t *testing.T) {
		setNotionEnv(t, nil)
		cfg, missing, err := OptionalConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(missing) != 0 {
			t.Fatalf("missing = %v, want nil", missing)
		}
		if cfg.NotionToken != "tok-123" {
			t.Errorf("NotionToken = %q, want %q", cfg.NotionToken, "tok-123")
		}
		if cfg.HandlerConfig.Token != "sync-tok" {
			t.Errorf("HandlerConfig.Token = %q, want %q", cfg.HandlerConfig.Token, "sync-tok")
		}
		if cfg.HandlerConfig.OwnerID != "owner-abc" {
			t.Errorf("OwnerID = %q, want %q", cfg.HandlerConfig.OwnerID, "owner-abc")
		}
		if cfg.HandlerConfig.CardgroupName != "My Cards" {
			t.Errorf("CardgroupName = %q, want %q", cfg.HandlerConfig.CardgroupName, "My Cards")
		}
		wantPageIDs := []string{"page-1", "page-2", "page-3"}
		if !reflect.DeepEqual(cfg.HandlerConfig.PageIDs, wantPageIDs) {
			t.Errorf("PageIDs = %v, want %v", cfg.HandlerConfig.PageIDs, wantPageIDs)
		}
	})

	t.Run("NOTION_TOKEN only unset: missing == [NOTION_TOKEN], err nil", func(t *testing.T) {
		setNotionEnv(t, map[string]string{"NOTION_TOKEN": ""})
		cfg, missing, err := OptionalConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"NOTION_TOKEN"}
		if !reflect.DeepEqual(missing, want) {
			t.Errorf("missing = %v, want %v", missing, want)
		}
		if cfg.NotionToken != "" || cfg.HandlerConfig.Token != "" {
			t.Errorf("cfg = %+v, want zero value", cfg)
		}
	})

	t.Run("all five vars unset: missing in fixed order, err nil", func(t *testing.T) {
		for _, k := range varOrder {
			t.Setenv(k, "")
		}
		cfg, missing, err := OptionalConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{
			"NOTION_TOKEN",
			"NOTION_PAGE_IDS",
			"NOTION_TARGET_OWNER_ID",
			"NOTION_TARGET_CARDGROUP_NAME",
			"NOTION_SYNC_TOKEN",
		}
		if !reflect.DeepEqual(missing, want) {
			t.Errorf("missing = %v, want %v", missing, want)
		}
		if cfg.NotionToken != "" || cfg.HandlerConfig.Token != "" {
			t.Errorf("cfg = %+v, want zero value", cfg)
		}
	})

	t.Run("whitespace-only NOTION_TARGET_OWNER_ID treated as missing", func(t *testing.T) {
		setNotionEnv(t, map[string]string{"NOTION_TARGET_OWNER_ID": "  "})
		_, missing, err := OptionalConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(missing) != 1 || missing[0] != "NOTION_TARGET_OWNER_ID" {
			t.Errorf("missing = %v, want [NOTION_TARGET_OWNER_ID]", missing)
		}
	})

	t.Run("all vars set + NOTION_PAGE_IDS commas only: missing nil, err non-nil", func(t *testing.T) {
		setNotionEnv(t, map[string]string{"NOTION_PAGE_IDS": ",,,"})
		_, missing, err := OptionalConfigFromEnv()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must contain at least one page id") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
		if len(missing) != 0 {
			t.Errorf("missing = %v, want nil", missing)
		}
	})

	t.Run("trailing whitespace in NOTION_TOKEN is trimmed", func(t *testing.T) {
		setNotionEnv(t, map[string]string{"NOTION_TOKEN": "tok-123 "})
		cfg, missing, err := OptionalConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(missing) != 0 {
			t.Fatalf("missing = %v, want nil", missing)
		}
		if cfg.NotionToken != "tok-123" {
			t.Errorf("NotionToken = %q, want %q", cfg.NotionToken, "tok-123")
		}
	})
}

func TestSplitCSV(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string",
			input: "",
			want:  []string{},
		},
		{
			name:  "single value",
			input: "page-1",
			want:  []string{"page-1"},
		},
		{
			name:  "multiple values with mixed whitespace",
			input: " a , b,c ,  d  ",
			want:  []string{"a", "b", "c", "d"},
		},
		{
			name:  "all-empty segments",
			input: ",,,",
			want:  []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitCSV(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("splitCSV(%q) = %v (len %d), want %v (len %d)",
					tc.input, got, len(got), tc.want, len(tc.want))
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Errorf("splitCSV(%q)[%d] = %q, want %q", tc.input, i, got[i], w)
				}
			}
		})
	}
}
