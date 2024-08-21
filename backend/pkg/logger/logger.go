package logger

import (
	"backend/pkg/config"
	"log/slog"
	"os"
	"strings"
)

var Logger *slog.Logger

func init() {
	maskKeywords := []string{"password", "secret"}
	masking := NewMaskingReplaceAttr(maskKeywords)

	var handler slog.Handler
	if config.Cfg.GoEnv == config.APP_MODE_DEV {
		// Use console mode for development environment
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			ReplaceAttr: masking,
		})
	} else {
		// Use JSON mode for other environments
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			ReplaceAttr: masking,
		})
	}

	Logger = slog.New(handler)
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
