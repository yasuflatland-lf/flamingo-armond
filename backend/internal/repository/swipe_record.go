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
	CardgroupID   string    `gorm:"column:cardgroup_id;type:uuid"`
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
	// Pre-swipe snapshot columns. Nullable: a NULL marks a legacy row recorded
	// before the columns existed.
	PhaseBefore         *int16 `gorm:"column:phase_before"`
	ScheduledDaysBefore *int   `gorm:"column:scheduled_days_before"`
}

func (gormSwipeRecord) TableName() string { return "swipe_records" }

type SwipeRecordRepository interface {
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.SwipeRecord, error)
	FindByUserAndCardgroup(ctx context.Context, userID, cardgroupID string) ([]*domain.SwipeRecord, error)
	ListRecentByUser(ctx context.Context, userID string, limit int) ([]*domain.SwipeRecord, error)
	ListByUserSince(ctx context.Context, userID string, since time.Time) ([]*domain.SwipeRecord, error)
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
		return nil, eris.Wrap(err, "repository: swipe record: find by ids")
	}
	out := make(map[string]*domain.SwipeRecord, len(rows))
	for i := range rows {
		sr, err := swipeRecordToDomain(rows[i])
		if err != nil {
			return nil, err
		}
		out[sr.ID] = sr
	}
	return out, nil
}

func (r *swipeRecordRepo) FindByUserAndCardgroup(ctx context.Context, userID, cardgroupID string) ([]*domain.SwipeRecord, error) {
	var rows []gormSwipeRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND cardgroup_id = ?", userID, cardgroupID).
		Order("reviewed_at DESC, id DESC").
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: swipe record: find by user and cardgroup")
	}
	out := make([]*domain.SwipeRecord, len(rows))
	for i := range rows {
		sr, err := swipeRecordToDomain(rows[i])
		if err != nil {
			return nil, err
		}
		out[i] = sr
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
		return nil, eris.Wrap(err, "repository: swipe record: list recent by user")
	}

	out := make([]*domain.SwipeRecord, len(rows))
	for i := range rows {
		sr, err := swipeRecordToDomain(rows[i])
		if err != nil {
			return nil, err
		}
		out[i] = sr
	}
	return out, nil
}

// ListByUserSince returns userID's swipes with reviewed_at >= since, in
// unspecified order (ComputeMetrics is order-independent). Windowed read for the
// stats diagnostic snapshot.
func (r *swipeRecordRepo) ListByUserSince(ctx context.Context, userID string, since time.Time) ([]*domain.SwipeRecord, error) {
	var rows []gormSwipeRecord
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND reviewed_at >= ?", userID, since).
		Find(&rows).Error
	if err != nil {
		return nil, eris.Wrap(err, "repository: swipe record: list by user since")
	}
	out := make([]*domain.SwipeRecord, 0, len(rows))
	for i := range rows {
		sr, err := swipeRecordToDomain(rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, sr)
	}
	return out, nil
}

func (r *swipeRecordRepo) CreateTx(ctx context.Context, tx *gorm.DB, sr *domain.SwipeRecord) error {
	if err := tx.WithContext(ctx).Create(swipeRecordToRow(sr)).Error; err != nil {
		return eris.Wrap(err, "repository: swipe record: create")
	}
	return nil
}

func swipeRecordToRow(sr *domain.SwipeRecord) *gormSwipeRecord {
	var phaseBefore *int16
	if sr.PhaseBefore != nil {
		v := int16(*sr.PhaseBefore)
		phaseBefore = &v
	}
	var scheduledDaysBefore *int
	if sr.ScheduledDaysBefore != nil {
		v := *sr.ScheduledDaysBefore
		scheduledDaysBefore = &v
	}
	return &gormSwipeRecord{
		ID:                  sr.ID,
		UserID:              string(sr.UserID),
		CardID:              sr.CardID,
		CardgroupID:         string(sr.CardgroupID),
		Rating:              int(sr.Rating),
		ReviewedAt:          sr.ReviewedAt,
		Due:                 sr.StateAfter.Due,
		Stability:           sr.StateAfter.Stability,
		Difficulty:          sr.StateAfter.Difficulty,
		ElapsedDays:         sr.StateAfter.ElapsedDays,
		ScheduledDays:       sr.StateAfter.ScheduledDays,
		Reps:                sr.StateAfter.Reps,
		Lapses:              sr.StateAfter.Lapses,
		State:               int(sr.StateAfter.Phase),
		LastReview:          sr.StateAfter.LastReview,
		PhaseBefore:         phaseBefore,
		ScheduledDaysBefore: scheduledDaysBefore,
	}
}

func swipeRecordToDomain(row gormSwipeRecord) (*domain.SwipeRecord, error) {
	rating := domain.Rating(row.Rating)
	if !rating.IsValid() {
		return nil, eris.Errorf("repository: swipe record: invalid Rating value %d for swipe record %s", row.Rating, row.ID)
	}
	phase := domain.FSRSPhase(row.State)
	if !phase.IsValid() {
		return nil, eris.Errorf("repository: swipe record: invalid FSRSPhase value %d for swipe record %s", row.State, row.ID)
	}
	// Pre-swipe snapshot columns are nullable (NULL = legacy row). A non-nil
	// phase_before that is out of range is a corrupt persisted value; reject it
	// rather than reconstitute a SwipeRecord carrying an invalid phase.
	var phaseBefore *domain.FSRSPhase
	if row.PhaseBefore != nil {
		p := domain.FSRSPhase(*row.PhaseBefore)
		if !p.IsValid() {
			return nil, eris.Errorf("repository: swipe record: invalid phase_before value %d for swipe record %s", *row.PhaseBefore, row.ID)
		}
		phaseBefore = &p
	}
	var scheduledDaysBefore *int
	if row.ScheduledDaysBefore != nil {
		v := *row.ScheduledDaysBefore
		scheduledDaysBefore = &v
	}
	return &domain.SwipeRecord{
		ID:          row.ID,
		UserID:      domain.UserID(row.UserID),
		CardID:      row.CardID,
		CardgroupID: domain.CardgroupID(row.CardgroupID),
		Rating:      rating,
		ReviewedAt:  row.ReviewedAt,
		StateAfter: domain.FSRSState{
			Due:           row.Due,
			Stability:     row.Stability,
			Difficulty:    row.Difficulty,
			ElapsedDays:   row.ElapsedDays,
			ScheduledDays: row.ScheduledDays,
			Reps:          row.Reps,
			Lapses:        row.Lapses,
			Phase:         phase,
			LastReview:    row.LastReview,
		},
		PhaseBefore:         phaseBefore,
		ScheduledDaysBefore: scheduledDaysBefore,
	}, nil
}
