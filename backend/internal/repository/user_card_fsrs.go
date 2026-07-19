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
	LastRating    *int      `gorm:"column:last_rating"`
	ElapsedDays   int       `gorm:"column:elapsed_days"`
	ScheduledDays int       `gorm:"column:scheduled_days"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (gormUserCardFSRS) TableName() string { return "user_card_fsrs" }

type UserCardFSRSRepository interface {
	UpsertTx(ctx context.Context, tx *gorm.DB, u *domain.UserCardFSRS) error
	FindByUserAndCardIDs(ctx context.Context, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error)
	FindByUserAndCardIDsTx(ctx context.Context, tx *gorm.DB, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error)
	// ListFSRSStatesByUser returns one lightweight domain.FSRSStat per studied
	// card for userID, joined to cards for the owning cardgroup.
	ListFSRSStatesByUser(ctx context.Context, userID string) ([]domain.FSRSStat, error)
	// CountCardsByCardgroupForUser returns the total card count per cardgroup the
	// user owns, for every owned deck that has at least one card (the per-deck
	// denominators + the deck list for the acquisition rate). Decks with cards
	// but zero studied cards are included; decks with zero cards are omitted —
	// an empty deck has no acquisition denominator.
	CountCardsByCardgroupForUser(ctx context.Context, userID string) (map[string]int, error)
}

type userCardFSRSRepo struct{ db *gorm.DB }

func NewUserCardFSRSRepository(db *gorm.DB) UserCardFSRSRepository {
	return &userCardFSRSRepo{db: db}
}

func (r *userCardFSRSRepo) UpsertTx(ctx context.Context, tx *gorm.DB, u *domain.UserCardFSRS) error {
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "card_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"state":          int(u.State.Phase),
			"due":            u.State.Due,
			"stability":      u.State.Stability,
			"difficulty":     u.State.Difficulty,
			"reps":           u.State.Reps,
			"lapses":         u.State.Lapses,
			"last_review":    u.State.LastReview,
			"last_rating":    userCardFSRSLastRating(u.State),
			"elapsed_days":   u.State.ElapsedDays,
			"scheduled_days": u.State.ScheduledDays,
			"updated_at":     gorm.Expr("now()"),
		}),
	}).Create(userCardFSRSToRow(u)).Error; err != nil {
		return eris.Wrap(err, "repository: user card fsrs: upsert")
	}
	return nil
}

// ListFSRSStatesByUser returns one row per studied card for userID, joined to
// cards for the owning cardgroup. WHERE user_card_fsrs.user_id = ? is backed by
// the (user_id, card_id) PK.
func (r *userCardFSRSRepo) ListFSRSStatesByUser(ctx context.Context, userID string) ([]domain.FSRSStat, error) {
	var rows []domain.FSRSStat
	err := r.db.WithContext(ctx).
		Table("user_card_fsrs AS f").
		Select("f.card_id AS card_id, c.cardgroup_id AS cardgroup_id, f.state AS phase, f.stability AS stability, f.lapses AS lapses").
		Joins("JOIN cards c ON c.id = f.card_id").
		Where("f.user_id = ?", userID).
		Scan(&rows).Error
	if err != nil {
		return nil, eris.Wrap(err, "repository: user card fsrs: list states by user")
	}
	for i := range rows {
		if !rows[i].Phase.IsValid() {
			return nil, eris.Errorf("repository: user card fsrs: invalid FSRSPhase value %d for card %s", int(rows[i].Phase), rows[i].CardID)
		}
	}
	return rows, nil
}

// CountCardsByCardgroupForUser returns the total card count per cardgroup the
// user owns (cardgroups.owner_id = ?), for every owned deck that has at least
// one card. The result seeds both the per-deck denominators and the deck
// list, so a deck with cards but zero studied cards still appears — but a
// deck with zero cards is omitted (the INNER JOIN to cards drops it; an empty
// deck has no acquisition denominator).
func (r *userCardFSRSRepo) CountCardsByCardgroupForUser(ctx context.Context, userID string) (map[string]int, error) {
	type countRow struct {
		CardgroupID string
		Total       int
	}
	var rows []countRow
	err := r.db.WithContext(ctx).
		Table("cardgroups AS cg").
		Select("cg.id AS cardgroup_id, COUNT(c.id) AS total").
		Joins("JOIN cards c ON c.cardgroup_id = cg.id").
		Where("cg.owner_id = ?", userID).
		Group("cg.id").
		Scan(&rows).Error
	if err != nil {
		return nil, eris.Wrap(err, "repository: user card fsrs: count cards by cardgroup for user")
	}
	out := make(map[string]int, len(rows))
	for _, row := range rows {
		out[row.CardgroupID] = row.Total
	}
	return out, nil
}

func (r *userCardFSRSRepo) FindByUserAndCardIDs(ctx context.Context, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error) {
	out, err := findUserCardFSRSByUserAndCardIDs(ctx, r.db, userID, cardIDs)
	if err != nil {
		return nil, eris.Wrap(err, "repository: user card fsrs: find by user and card ids")
	}
	return out, nil
}

func (r *userCardFSRSRepo) FindByUserAndCardIDsTx(ctx context.Context, tx *gorm.DB, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error) {
	out, err := findUserCardFSRSByUserAndCardIDs(ctx, tx, userID, cardIDs)
	if err != nil {
		return nil, eris.Wrap(err, "repository: user card fsrs: find by user and card ids tx")
	}
	return out, nil
}

// findUserCardFSRSByUserAndCardIDs is shared by FindByUserAndCardIDs (pool) and
// FindByUserAndCardIDsTx (transaction). It runs the (user_id, card_id IN ?)
// fetch against the supplied db handle and maps the rows into domain values. An
// empty cardIDs slice returns an empty map with no SQL (GORM drops an empty
// IN ? clause and would otherwise full-table scan). The error is returned
// unwrapped so each caller can attach its own layer prefix.
func findUserCardFSRSByUserAndCardIDs(ctx context.Context, db *gorm.DB, userID string, cardIDs []string) (map[string]*domain.UserCardFSRS, error) {
	if len(cardIDs) == 0 {
		return map[string]*domain.UserCardFSRS{}, nil
	}
	var rows []gormUserCardFSRS
	if err := db.WithContext(ctx).
		Where("user_id = ? AND card_id IN ?", userID, cardIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rowsToUserCardFSRSMap(rows)
}

func rowsToUserCardFSRSMap(rows []gormUserCardFSRS) (map[string]*domain.UserCardFSRS, error) {
	out := make(map[string]*domain.UserCardFSRS, len(rows))
	for i := range rows {
		ucs, err := userCardFSRSToDomain(rows[i])
		if err != nil {
			return nil, err
		}
		out[ucs.CardID] = ucs
	}
	return out, nil
}

func userCardFSRSToRow(u *domain.UserCardFSRS) *gormUserCardFSRS {
	return &gormUserCardFSRS{
		UserID:        string(u.UserID),
		CardID:        u.CardID,
		State:         int(u.State.Phase),
		Due:           u.State.Due,
		Stability:     u.State.Stability,
		Difficulty:    u.State.Difficulty,
		Reps:          u.State.Reps,
		Lapses:        u.State.Lapses,
		LastReview:    u.State.LastReview,
		LastRating:    userCardFSRSLastRating(u.State),
		ElapsedDays:   u.State.ElapsedDays,
		ScheduledDays: u.State.ScheduledDays,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

func userCardFSRSLastRating(state domain.FSRSState) *int {
	if !state.LastRating.IsValid() {
		return nil
	}
	rating := int(state.LastRating)
	return &rating
}

func userCardFSRSToDomain(row gormUserCardFSRS) (*domain.UserCardFSRS, error) {
	state := domain.FSRSPhase(row.State)
	if !state.IsValid() {
		return nil, eris.Errorf("repository: invalid FSRSPhase value %d for card %s", row.State, row.CardID)
	}
	lastRating := domain.Rating(0)
	if row.LastRating != nil {
		lastRating = domain.Rating(*row.LastRating)
	}
	return &domain.UserCardFSRS{
		UserID: domain.UserID(row.UserID),
		CardID: row.CardID,
		State: domain.FSRSState{
			Due:           row.Due,
			Stability:     row.Stability,
			Difficulty:    row.Difficulty,
			ElapsedDays:   row.ElapsedDays,
			ScheduledDays: row.ScheduledDays,
			Reps:          row.Reps,
			Lapses:        row.Lapses,
			Phase:         state,
			LastReview:    row.LastReview,
			LastRating:    lastRating,
		},
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}
