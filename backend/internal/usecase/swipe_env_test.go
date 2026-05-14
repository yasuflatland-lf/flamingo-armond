package usecase_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"backend/internal/usecase"
)

func TestSwipeNextBatchSize(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		want    int
		wantLog string
	}{
		{"empty uses default", "", 10, ""},
		{"valid positive", "20", 20, ""},
		{"invalid string uses default", "bad", 10, "invalid SWIPE_NEXT_BATCH_SIZE"},
		{"zero uses default", "0", 10, "invalid SWIPE_NEXT_BATCH_SIZE"},
		{"negative uses default", "-5", 10, "invalid SWIPE_NEXT_BATCH_SIZE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SWIPE_NEXT_BATCH_SIZE", tc.env)
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			got := usecase.SwipeNextBatchSize(logger)
			if got != tc.want {
				t.Errorf("SwipeNextBatchSize(%q) = %d, want %d", tc.env, got, tc.want)
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
