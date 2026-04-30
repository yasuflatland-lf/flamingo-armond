package repository

import (
	"context"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

type gormSwipeRecord struct {
	ID            string    `gorm:"column:id;primaryKey;type:uuid"`
	UserID        string    `gorm:"column:user_id"`
	CardID        string    `gorm:"column:card_id"`
	Rating        int       `gorm:"column:rating"`
	ReviewedAt    time.Time `gorm:"column:reviewed_at"`
	Due           time.Time `gorm:"column:due"`
	Stability     float64   `gorm:"column:stability"`
	Difficulty    float64   `gorm:"column:difficulty"`
	ElapsedDays   int       `gorm:"column:elapsed_days"`
	ScheduledDays int       `gorm:"column:scheduled_days"`
	Reps          int       `gorm:"column:reps"`
	Lapses        int       `gorm:"column:lapses"`
	State         int       `gorm:"column:state"`
	LastReview    time.Time `gorm:"column:last_review"`
}

func (gormSwipeRecord) TableName() string { return "swipe_records" }

type SwipeRecordRepository interface {
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.SwipeRecord, error)
	FindByUserAndCardgroup(ctx context.Context, userID, cardgroupID string) ([]*domain.SwipeRecord, error)
	ListRecentByUser(ctx context.Context, userID string, limit int) ([]*domain.SwipeRecord, error)
	CreateTx(ctx context.Context, tx *gorm.DB, sr *domain.SwipeRecord) error
}

type swipeRecordRepo struct{ db *gorm.DB }

func NewSwipeRecordRepository(db *gorm.DB) SwipeRecordRepository {
	return &swipeRecordRepo{db: db}
}

func (r *swipeRecordRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.SwipeRecord, error) {
	if len(ids) == 0 {
		return map[string]*domain.SwipeRecord{}, nil
	}
	var rows []gormSwipeRecord
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find swipe records by ids")
	}
	out := make(map[string]*domain.SwipeRecord, len(rows))
	for i := range rows {
		sr := swipeRecordToDomain(rows[i])
		out[sr.ID] = sr
	}
	return out, nil
}

func (r *swipeRecordRepo) FindByUserAndCardgroup(ctx context.Context, userID, cardgroupID string) ([]*domain.SwipeRecord, error) {
	var rows []gormSwipeRecord
	if err := r.db.WithContext(ctx).
		Joins("JOIN cards ON cards.id = swipe_records.card_id").
		Where("swipe_records.user_id = ? AND cards.cardgroup_id = ?", userID, cardgroupID).
		Order("swipe_records.reviewed_at DESC, swipe_records.id DESC").
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find swipe records by user and cardgroup")
	}
	out := make([]*domain.SwipeRecord, len(rows))
	for i := range rows {
		out[i] = swipeRecordToDomain(rows[i])
	}
	return out, nil
}

func (r *swipeRecordRepo) ListRecentByUser(ctx context.Context, userID string, limit int) ([]*domain.SwipeRecord, error) {
	if limit <= 0 {
		return []*domain.SwipeRecord{}, nil
	}

	var rows []gormSwipeRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("reviewed_at DESC, id DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: list recent swipe records by user")
	}

	out := make([]*domain.SwipeRecord, len(rows))
	for i := range rows {
		out[i] = swipeRecordToDomain(rows[i])
	}
	return out, nil
}

func (r *swipeRecordRepo) CreateTx(ctx context.Context, tx *gorm.DB, sr *domain.SwipeRecord) error {
	if err := tx.WithContext(ctx).Create(swipeRecordToRow(sr)).Error; err != nil {
		return eris.Wrap(err, "repository: create swipe record")
	}
	return nil
}

func swipeRecordToRow(sr *domain.SwipeRecord) *gormSwipeRecord {
	return &gormSwipeRecord{
		ID:            sr.ID,
		UserID:        sr.UserID,
		CardID:        sr.CardID,
		Rating:        int(sr.Rating),
		ReviewedAt:    sr.ReviewedAt,
		Due:           sr.StateAfter.Due,
		Stability:     sr.StateAfter.Stability,
		Difficulty:    sr.StateAfter.Difficulty,
		ElapsedDays:   sr.StateAfter.ElapsedDays,
		ScheduledDays: sr.StateAfter.ScheduledDays,
		Reps:          sr.StateAfter.Reps,
		Lapses:        sr.StateAfter.Lapses,
		State:         int(sr.StateAfter.State),
		LastReview:    sr.StateAfter.LastReview,
	}
}

func swipeRecordToDomain(row gormSwipeRecord) *domain.SwipeRecord {
	return &domain.SwipeRecord{
		ID:         row.ID,
		UserID:     row.UserID,
		CardID:     row.CardID,
		Rating:     domain.Rating(row.Rating),
		ReviewedAt: row.ReviewedAt,
		StateAfter: domain.FSRSState{
			Due:           row.Due,
			Stability:     row.Stability,
			Difficulty:    row.Difficulty,
			ElapsedDays:   row.ElapsedDays,
			ScheduledDays: row.ScheduledDays,
			Reps:          row.Reps,
			Lapses:        row.Lapses,
			State:         domain.FSRSCardState(row.State),
			LastReview:    row.LastReview,
		},
	}
}
