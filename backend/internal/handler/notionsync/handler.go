package notionsync

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"backend/internal/logging"
	"backend/internal/notion"
	"backend/internal/usecase"
)

type SyncUsecase interface {
	Sync(ctx context.Context, input usecase.SyncFromNotionInput) (usecase.SyncFromNotionOutput, error)
}

type Config struct {
	Token         string
	PageIDs       []string
	OwnerID       string
	CardgroupName string
}

type Handler struct {
	uc     SyncUsecase
	config Config
	logger *slog.Logger
}

func New(uc SyncUsecase, cfg Config) *Handler {
	if uc == nil {
		panic("notionsync.New: usecase must not be nil")
	}
	if cfg.Token == "" {
		panic("notionsync.New: token must not be empty")
	}
	return &Handler{uc: uc, config: cfg, logger: slog.Default()}
}

func (h *Handler) Handle(c *echo.Context) error {
	if !validBearer(c.Request().Header.Get("Authorization"), h.config.Token) {
		h.logger.WarnContext(c.Request().Context(), "notion sync: unauthorized",
			"remote_ip", c.RealIP(),
		)
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	out, err := h.uc.Sync(c.Request().Context(), usecase.SyncFromNotionInput{
		PageIDs:       h.config.PageIDs,
		OwnerID:       h.config.OwnerID,
		CardgroupName: h.config.CardgroupName,
	})
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func validBearer(authHeader, expected string) bool {
	got, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok {
		return false
	}
	gotHash := sha256.Sum256([]byte(got))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(gotHash[:], expectedHash[:]) == 1
}

func (h *Handler) handleError(c *echo.Context, err error) error {
	ctx := c.Request().Context()
	remoteIP := slog.String("remote_ip", c.RealIP())
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), errors.Is(err, notion.ErrRetryElapsed), errors.Is(err, notion.ErrRetryAttempts):
		logging.LogWarn(ctx, h.logger, "notion sync: timeout", err, remoteIP)
		return c.JSON(http.StatusGatewayTimeout, map[string]string{"error": "notion sync timed out"})
	case errors.Is(err, usecase.ErrNotionSyncFetch):
		logging.LogWarn(ctx, h.logger, "notion sync: fetch failed", err, remoteIP)
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "notion fetch failed"})
	case errors.Is(err, usecase.ErrNotionSyncInvalidInput):
		logging.LogWarn(ctx, h.logger, "notion sync: invalid input", err, remoteIP)
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "invalid input"})
	case errors.Is(err, usecase.ErrNotionSyncParse):
		logging.LogWarn(ctx, h.logger, "notion sync: parse error", err, remoteIP)
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "notion parse error"})
	case errors.Is(err, usecase.ErrNotionSyncPersist):
		logging.LogError(ctx, h.logger, "notion sync: persist failed", err, remoteIP)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "persist error"})
	default:
		logging.LogError(ctx, h.logger, "notion sync: failed", err, remoteIP)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}
