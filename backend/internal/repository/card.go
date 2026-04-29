package repository

import (
	"context"
	"errors"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

type gormCard struct {
	ID            string    `gorm:"column:id;primaryKey;type:uuid"`
	CardgroupID   string    `gorm:"column:cardgroup_id"`
	Front         string    `gorm:"column:front"`
	Back          string    `gorm:"column:back"`
	Due           time.Time `gorm:"column:due"`
	Stability     float64   `gorm:"column:stability"`
	Difficulty    float64   `gorm:"column:difficulty"`
	ElapsedDays   int       `gorm:"column:elapsed_days"`
	ScheduledDays int       `gorm:"column:scheduled_days"`
	Reps          int       `gorm:"column:reps"`
	Lapses        int       `gorm:"column:lapses"`
	State         int       `gorm:"column:state"`
	LastReview    time.Time `gorm:"column:last_review"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (gormCard) TableName() string { return "cards" }

type CardUpdate struct {
	Front *string
	Back  *string
}

type CardRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Card, error)
	FindByIDTx(ctx context.Context, tx *gorm.DB, id string) (*domain.Card, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error)
	FindByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error)
	FindDueCardsTx(ctx context.Context, tx *gorm.DB, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error)
	Create(ctx context.Context, card *domain.Card) error
	UpdateFSRSStateTx(ctx context.Context, tx *gorm.DB, id string, state domain.FSRSState) error
	Update(ctx context.Context, id string, patch CardUpdate) (*domain.Card, error)
	Delete(ctx context.Context, id string) error
}

type cardRepo struct{ db *gorm.DB }

func NewCardRepository(db *gorm.DB) CardRepository { return &cardRepo{db: db} }

func (r *cardRepo) FindByID(ctx context.Context, id string) (*domain.Card, error) {
	return findCardByID(ctx, r.db, id)
}

func (r *cardRepo) FindByIDTx(ctx context.Context, tx *gorm.DB, id string) (*domain.Card, error) {
	return findCardByID(ctx, tx, id)
}

func findCardByID(ctx context.Context, db *gorm.DB, id string) (*domain.Card, error) {
	var row gormCard
	err := db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: find card by id")
	}
	return cardToDomain(row), nil
}

func (r *cardRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error) {
	if len(ids) == 0 {
		return map[string]*domain.Card{}, nil
	}
	var rows []gormCard
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find cards by ids")
	}
	out := make(map[string]*domain.Card, len(rows))
	for i := range rows {
		card := cardToDomain(rows[i])
		out[card.ID] = card
	}
	return out, nil
}

func (r *cardRepo) FindByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error) {
	var rows []gormCard
	if err := r.db.WithContext(ctx).
		Where("cardgroup_id = ?", cardgroupID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find cards by cardgroup")
	}
	out := make([]*domain.Card, len(rows))
	for i := range rows {
		out[i] = cardToDomain(rows[i])
	}
	return out, nil
}

func (r *cardRepo) FindDueCardsTx(ctx context.Context, tx *gorm.DB, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error) {
	if limit <= 0 {
		return []*domain.Card{}, nil
	}
	var rows []gormCard
	if err := tx.WithContext(ctx).
		Where("cardgroup_id = ? AND due <= ?", cardgroupID, now).
		Order("due ASC, id ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find due cards")
	}
	out := make([]*domain.Card, len(rows))
	for i := range rows {
		out[i] = cardToDomain(rows[i])
	}
	return out, nil
}

func (r *cardRepo) Create(ctx context.Context, card *domain.Card) error {
	if err := r.db.WithContext(ctx).Create(cardToRow(card)).Error; err != nil {
		return eris.Wrap(err, "repository: create card")
	}
	return nil
}

func (r *cardRepo) UpdateFSRSStateTx(ctx context.Context, tx *gorm.DB, id string, state domain.FSRSState) error {
	updates := map[string]any{
		"due":            state.Due,
		"stability":      state.Stability,
		"difficulty":     state.Difficulty,
		"elapsed_days":   state.ElapsedDays,
		"scheduled_days": state.ScheduledDays,
		"reps":           state.Reps,
		"lapses":         state.Lapses,
		"state":          int(state.State),
		"last_review":    state.LastReview,
	}
	res := tx.WithContext(ctx).Model(&gormCard{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: update card fsrs state")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *cardRepo) Update(ctx context.Context, id string, patch CardUpdate) (*domain.Card, error) {
	updates := map[string]any{}
	if patch.Front != nil {
		updates["front"] = *patch.Front
	}
	if patch.Back != nil {
		updates["back"] = *patch.Back
	}
	if len(updates) == 0 {
		return r.FindByID(ctx, id)
	}

	res := r.db.WithContext(ctx).Model(&gormCard{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, eris.Wrap(res.Error, "repository: update card")
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.FindByID(ctx, id)
}

func (r *cardRepo) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&gormCard{})
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: delete card")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func cardToRow(card *domain.Card) *gormCard {
	return &gormCard{
		ID:            card.ID,
		CardgroupID:   card.CardgroupID,
		Front:         card.Front,
		Back:          card.Back,
		Due:           card.FSRS.Due,
		Stability:     card.FSRS.Stability,
		Difficulty:    card.FSRS.Difficulty,
		ElapsedDays:   card.FSRS.ElapsedDays,
		ScheduledDays: card.FSRS.ScheduledDays,
		Reps:          card.FSRS.Reps,
		Lapses:        card.FSRS.Lapses,
		State:         int(card.FSRS.State),
		LastReview:    card.FSRS.LastReview,
		CreatedAt:     card.CreatedAt,
		UpdatedAt:     card.UpdatedAt,
	}
}

func cardToDomain(row gormCard) *domain.Card {
	return &domain.Card{
		ID:          row.ID,
		CardgroupID: row.CardgroupID,
		Front:       row.Front,
		Back:        row.Back,
		FSRS: domain.FSRSState{
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
