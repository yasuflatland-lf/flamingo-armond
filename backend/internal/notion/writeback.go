package notion

import (
	"context"
	"log/slog"
	"time"

	"backend/internal/domain"
)

const cardWritebackTimeout = 15 * time.Second

type ParagraphAppender interface {
	AppendParagraph(ctx context.Context, pageID, text string) error
}

type CardWritebacker struct {
	appender ParagraphAppender
	pageID   string
	logger   *slog.Logger
}

func NewCardWritebacker(appender ParagraphAppender, pageID string, logger *slog.Logger) *CardWritebacker {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &CardWritebacker{appender: appender, pageID: pageID, logger: logger}
}

func (w *CardWritebacker) OnCardCreated(_ context.Context, card *domain.Card) {
	w.append("card create", card)
}

func (w *CardWritebacker) OnCardUpdated(_ context.Context, card *domain.Card) {
	w.append("card update", card)
}

func (w *CardWritebacker) append(event string, card *domain.Card) {
	if w == nil || w.appender == nil || w.pageID == "" || card == nil {
		return
	}
	text := string(card.Front) + " " + string(card.Back)
	cardID := card.ID
	cardgroupID := card.CardgroupID
	pageID := w.pageID
	appender := w.appender
	logger := w.logger

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), cardWritebackTimeout)
		defer cancel()
		if err := appender.AppendParagraph(ctx, pageID, text); err != nil {
			logger.WarnContext(ctx, event+": notion writeback failed",
				"card_id", cardID, "page_id", pageID, "cardgroup_id", cardgroupID, "err", err)
		}
	}()
}
