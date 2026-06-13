package repository

import (
	"context"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

// gormMasterCard is the row mapping for public.master_cards. Package-private so
// callers cannot bypass the domain conversion. It is intentionally NOT embedded
// in any outer scan target: gormMasterCard carries a TableName() method that
// confuses GORM's embedded-struct schema parser when the outer scan target is a
// different type (see `.claude/rules/go-library-gotchas.md`).
type gormMasterCard struct {
	ID                string    `gorm:"column:id;primaryKey;type:uuid"`
	MasterCardgroupID string    `gorm:"column:master_cardgroup_id"`
	Front             string    `gorm:"column:front"`
	Back              string    `gorm:"column:back"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
	Position          int       `gorm:"column:position"`
}

func (gormMasterCard) TableName() string { return "master_cards" }

// MasterCardRepository provides persistence operations for the MasterCard
// aggregate. The bulk Tx methods share the table-parameterized helpers in
// card.go (upsertManyTx / listFrontsByCardgroupTx / deleteByCardgroupAndFrontsTx)
// with "master_cards" and "master_cardgroup_id".
type MasterCardRepository interface {
	ListByMasterCardgroup(ctx context.Context, masterCardgroupID string) ([]*domain.MasterCard, error)
	Create(ctx context.Context, c *domain.MasterCard) error
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.MasterCard) (UpsertManyTxResult, error)
	ListFrontsByMasterCardgroupTx(ctx context.Context, tx *gorm.DB, masterCardgroupID string) ([]string, error)
	DeleteByMasterCardgroupAndFrontsTx(ctx context.Context, tx *gorm.DB, masterCardgroupID string, fronts []string) (int64, error)
	Delete(ctx context.Context, id string) error
}

type masterCardRepo struct{ db *gorm.DB }

func NewMasterCardRepository(db *gorm.DB) MasterCardRepository { return &masterCardRepo{db: db} }

// ListByMasterCardgroup returns every master card in the group ordered by
// (position, id) so the deck order is deterministic. An empty masterCardgroupID
// short-circuits to (nil, nil): with an empty value GORM still emits the WHERE
// clause, but the empty-input guard keeps the contract explicit and mirrors the
// GORM empty-IN discipline in `.claude/rules/go-library-gotchas.md`.
func (r *masterCardRepo) ListByMasterCardgroup(ctx context.Context, masterCardgroupID string) ([]*domain.MasterCard, error) {
	if masterCardgroupID == "" {
		return nil, nil
	}
	var rows []gormMasterCard
	if err := r.db.WithContext(ctx).
		Where("master_cardgroup_id = ?", masterCardgroupID).
		Order("position ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: master card: list by master cardgroup")
	}
	out := make([]*domain.MasterCard, len(rows))
	for i := range rows {
		out[i] = masterCardToDomain(rows[i])
	}
	return out, nil
}

// Create inserts a single master card. When the ID is empty a UUID v7 is
// generated via domain.NewID(); the error is propagated (no v4 fallback) per
// `.claude/rules/go-library-gotchas.md` § "`uuid.NewV7` failure must propagate".
func (r *masterCardRepo) Create(ctx context.Context, c *domain.MasterCard) error {
	if c.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return eris.Wrap(err, "repository: master card: create")
		}
		c.ID = id
	}
	if err := r.db.WithContext(ctx).Create(masterCardToRow(c)).Error; err != nil {
		return eris.Wrap(err, "repository: master card: create")
	}
	return nil
}

// UpsertManyTx upserts master cards by (master_cardgroup_id, front). Existing
// rows have `back`, `updated_at`, and `position` overwritten. Returns the
// per-row Inserted/Updated split. Empty input is a no-op (handled by the shared
// helper). Operates on the supplied tx only.
func (r *masterCardRepo) UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.MasterCard) (UpsertManyTxResult, error) {
	rows := make([]upsertCardRow, len(cards))
	for i, c := range cards {
		rows[i] = upsertCardRow{
			ID:        c.ID,
			GroupID:   c.MasterCardgroupID,
			Front:     string(c.Front),
			Back:      string(c.Back),
			Position:  c.Position,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		}
	}
	res, err := upsertManyTx(ctx, tx, rows, "master_cards", "master_cardgroup_id")
	if err != nil {
		return UpsertManyTxResult{}, eris.Wrap(err, "repository: master card: upsert many")
	}
	return res, nil
}

// ListFrontsByMasterCardgroupTx returns the sorted `front` values for the group.
func (r *masterCardRepo) ListFrontsByMasterCardgroupTx(ctx context.Context, tx *gorm.DB, masterCardgroupID string) ([]string, error) {
	fronts, err := listFrontsByCardgroupTx(ctx, tx, masterCardgroupID, "master_cards", "master_cardgroup_id")
	if err != nil {
		return nil, eris.Wrap(err, "repository: master card: list fronts by master cardgroup")
	}
	return fronts, nil
}

// DeleteByMasterCardgroupAndFrontsTx hard-deletes master cards by the scoped
// (master_cardgroup_id, front) natural key. Empty fronts short-circuits to
// (0, nil) inside the shared helper.
func (r *masterCardRepo) DeleteByMasterCardgroupAndFrontsTx(ctx context.Context, tx *gorm.DB, masterCardgroupID string, fronts []string) (int64, error) {
	affected, err := deleteByCardgroupAndFrontsTx(ctx, tx, masterCardgroupID, fronts, "master_cards", "master_cardgroup_id")
	if err != nil {
		return 0, eris.Wrap(err, "repository: master card: delete by master cardgroup and fronts")
	}
	return affected, nil
}

// Delete hard-deletes the master card by primary key. Returns ErrNotFound when
// no row matched.
func (r *masterCardRepo) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&gormMasterCard{})
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: master card: delete")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func masterCardToRow(c *domain.MasterCard) *gormMasterCard {
	return &gormMasterCard{
		ID:                c.ID,
		MasterCardgroupID: c.MasterCardgroupID,
		Front:             string(c.Front),
		Back:              string(c.Back),
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
		Position:          c.Position,
	}
}

func masterCardToDomain(row gormMasterCard) *domain.MasterCard {
	return &domain.MasterCard{
		ID:                row.ID,
		MasterCardgroupID: row.MasterCardgroupID,
		Front:             domain.CardText(row.Front),
		Back:              domain.CardText(row.Back),
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
		Position:          row.Position,
	}
}
