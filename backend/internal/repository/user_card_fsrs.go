package repository

import (
	"context"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"backend/internal/domain"
)

type gormUserCardFSRS struct {
	UserID        string    `gorm:"column:user_id;primaryKey;type:uuid"`
	CardID        string    `gorm:"column:card_id;primaryKey;type:uuid"`
	State         int       `gorm:"column:state"`
	Due           time.Time `gorm:"column:due"`
	Stability     float64   `gorm:"column:stability"`
	Difficulty    float64   `gorm:"column:difficulty"`
	Reps          int       `gorm:"column:reps"`
	Lapses        int       `gorm:"column:lapses"`
	LastReview    time.Time `gorm:"column:last_review"`
	ElapsedDays   int       `gorm:"column:elapsed_days"`
	ScheduledDays int       `gorm:"column:scheduled_days"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (gormUserCardFSRS) TableName() string { return "user_card_fsrs" }

type UserCardFSRSRepository interface {
	UpsertTx(ctx context.Context, tx *gorm.DB, u *domain.UserCardFSRS) error
	FindByUserAndCardIDs(ctx context.Context, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error)
}

type userCardFSRSRepo struct{ db *gorm.DB }

func NewUserCardFSRSRepository(db *gorm.DB) UserCardFSRSRepository {
	return &userCardFSRSRepo{db: db}
}

func (r *userCardFSRSRepo) UpsertTx(ctx context.Context, tx *gorm.DB, u *domain.UserCardFSRS) error {
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "card_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"state":          int(u.State.State),
			"due":            u.State.Due,
			"stability":      u.State.Stability,
			"difficulty":     u.State.Difficulty,
			"reps":           u.State.Reps,
			"lapses":         u.State.Lapses,
			"last_review":    u.State.LastReview,
			"elapsed_days":   u.State.ElapsedDays,
			"scheduled_days": u.State.ScheduledDays,
			"updated_at":     gorm.Expr("now()"),
		}),
	}).Create(userCardFSRSToRow(u)).Error; err != nil {
		return eris.Wrap(err, "repository: upsert user card fsrs")
	}
	return nil
}

func (r *userCardFSRSRepo) FindByUserAndCardIDs(ctx context.Context, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error) {
	if len(cardIDs) == 0 {
		return map[string]*domain.UserCardFSRS{}, nil
	}
	var rows []gormUserCardFSRS
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND card_id IN ?", userID, cardIDs).
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find user card fsrs by user and card ids")
	}
	out := make(map[string]*domain.UserCardFSRS, len(rows))
	for i := range rows {
		ucs := userCardFSRSToDomain(rows[i])
		out[ucs.CardID] = ucs
	}
	return out, nil
}

func userCardFSRSToRow(u *domain.UserCardFSRS) *gormUserCardFSRS {
	return &gormUserCardFSRS{
		UserID:        u.UserID,
		CardID:        u.CardID,
		State:         int(u.State.State),
		Due:           u.State.Due,
		Stability:     u.State.Stability,
		Difficulty:    u.State.Difficulty,
		Reps:          u.State.Reps,
		Lapses:        u.State.Lapses,
		LastReview:    u.State.LastReview,
		ElapsedDays:   u.State.ElapsedDays,
		ScheduledDays: u.State.ScheduledDays,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

func userCardFSRSToDomain(row gormUserCardFSRS) *domain.UserCardFSRS {
	return &domain.UserCardFSRS{
		UserID: row.UserID,
		CardID: row.CardID,
		State: domain.FSRSState{
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
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
