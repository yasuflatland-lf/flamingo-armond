package logger

import (
	"log/slog"
	"os"
	"strings"
)

var Logger *slog.Logger

func init() {
	maskKeywords := []string{"password", "secret"}
	masking := NewMaskingReplaceAttr(maskKeywords)
	Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: masking,
	}))
}

func NewMaskingReplaceAttr(maskKeywords []string) func(groups []string, a slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		for _, v := range maskKeywords {
			if v == a.Key {
				return slog.String(a.Key, strings.Repeat("*", len(a.Value.String())))
			}
		}
		return a
	}
}
