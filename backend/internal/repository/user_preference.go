package repository

import (
	"context"
	"errors"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

// gormUserPreference is the row mapping for public.user_preferences.
// Package-private so callers cannot bypass the domain conversion.
// All writes go through raw UPSERT with now(); no GORM auto-fill.
type gormUserPreference struct {
	UserID                string    `gorm:"column:user_id;primaryKey;type:uuid"`
	LastViewedCardgroupID *string   `gorm:"column:last_viewed_cardgroup_id;type:uuid"`
	LearnDisplayMode      string    `gorm:"column:learn_display_mode"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
}

func (gormUserPreference) TableName() string { return "user_preferences" }

// ErrCardgroupNotFound is returned when UpsertLastViewedCardgroup targets a
// cardgroup that is missing OR not owned by the calling user. Joined with
// ErrNotFound so generic 'not found' classifiers (logging, metrics) keep working
// without learning the specific sentinel.
//
// Both "missing" and "not owned" collapse to the same sentinel deliberately:
// surfacing distinct sentinels would let a caller distinguish the two cases
// and probe the existence of cardgroups owned by other users.
var ErrCardgroupNotFound = errors.Join(
	errors.New("repository: cardgroup not found"),
	ErrNotFound,
)

// UserPreferenceRepository provides persistence operations for the
// UserPreference aggregate.
type UserPreferenceRepository interface {
	FindByUserID(ctx context.Context, userID string) (*domain.UserPreference, error)
	FindByUserIDs(ctx context.Context, userIDs []string) ([]*domain.UserPreference, error)
	// UpsertLastViewedCardgroup records userID's last-viewed cardgroup atomically,
	// verifying ownership in the same SQL statement. The INSERT fires only when
	// the cardgroup exists AND is owned by userID. Returns ErrCardgroupNotFound
	// when no row is written — the same sentinel for "cardgroup missing" and
	// "cardgroup not owned" so the caller cannot probe other users' cardgroups
	// via error shape.
	UpsertLastViewedCardgroup(ctx context.Context, userID, cardgroupID string) error
	// UpdateLearnDisplayMode upserts userID's learn display mode. The mode string
	// is the persisted form of a domain.LearnDisplayMode converted via
	// mode.String() at the usecase call site; unvalidated strings are never passed
	// here. The column CHECK constraint is a backstop for direct DB writes that
	// bypass the usecase layer.
	UpdateLearnDisplayMode(ctx context.Context, userID, mode string) error
}

type userPreferenceRepo struct{ db *gorm.DB }

func NewUserPreferenceRepository(db *gorm.DB) UserPreferenceRepository {
	return &userPreferenceRepo{db: db}
}

func (r *userPreferenceRepo) FindByUserID(ctx context.Context, userID string) (*domain.UserPreference, error) {
	var row gormUserPreference
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: user preference: find by user_id")
	}
	return toDomainUserPreference(row), nil
}

func (r *userPreferenceRepo) FindByUserIDs(ctx context.Context, userIDs []string) ([]*domain.UserPreference, error) {
	// GORM turns WHERE user_id IN () into an unfiltered scan, so short-circuit.
	if len(userIDs) == 0 {
		return []*domain.UserPreference{}, nil
	}
	var rows []gormUserPreference
	if err := r.db.WithContext(ctx).Where("user_id IN ?", userIDs).Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: user preference: find by user_ids")
	}
	out := make([]*domain.UserPreference, len(rows))
	for i := range rows {
		out[i] = toDomainUserPreference(rows[i])
	}
	return out, nil
}

// UpsertLastViewedCardgroup performs an ownership-checked UPSERT in a single
// SQL statement. A missing or non-owned cardgroup causes the
// INSERT ... SELECT ... WHERE EXISTS to insert zero rows, yielding
// RowsAffected == 0, which is reported as ErrCardgroupNotFound. A concurrent
// DELETE on the cardgroup row between the EXISTS check and the actual insert
// (TOCTOU race) raises Postgres FK violation 23503; the classifier maps it to
// the same sentinel so both code paths converge.
func (r *userPreferenceRepo) UpsertLastViewedCardgroup(ctx context.Context, userID, cardgroupID string) error {
	sql := `INSERT INTO user_preferences (user_id, last_viewed_cardgroup_id, updated_at)
SELECT ?, ?, now()
WHERE EXISTS (
    SELECT 1 FROM cardgroups WHERE id = ? AND owner_id = ?
)
ON CONFLICT (user_id) DO UPDATE
SET last_viewed_cardgroup_id = EXCLUDED.last_viewed_cardgroup_id,
    updated_at               = EXCLUDED.updated_at`

	res := r.db.WithContext(ctx).Exec(sql, userID, cardgroupID, cardgroupID, userID)
	if res.Error != nil {
		if classified := classifyUserPreferenceCardgroupFKError(res.Error); classified != nil {
			return classified
		}
		return eris.Wrap(res.Error, "repository: user preference: upsert last viewed cardgroup")
	}
	if res.RowsAffected == 0 {
		return ErrCardgroupNotFound
	}
	return nil
}

// classifyUserPreferenceCardgroupFKError maps a Postgres FK violation (code
// 23503) on the user_preferences.last_viewed_cardgroup_id column to
// ErrCardgroupNotFound. This shields the usecase from a TOCTOU race where the
// cardgroup is deleted between the EXISTS subquery and the UPSERT. The
// constraint name match is anchored on "last_viewed_cardgroup_id" — that
// column name is unique to this FK in the user_preferences table. Returns nil
// when err is not a FK violation so callers can fall through to eris.Wrap.
func classifyUserPreferenceCardgroupFKError(err error) error {
	if pgConstraintViolation(err, "23503", "last_viewed_cardgroup_id") {
		return ErrCardgroupNotFound
	}
	return nil
}

func (r *userPreferenceRepo) UpdateLearnDisplayMode(ctx context.Context, userID, mode string) error {
	sql := `INSERT INTO user_preferences (user_id, learn_display_mode, updated_at)
VALUES (?, ?, now())
ON CONFLICT (user_id) DO UPDATE
SET learn_display_mode = EXCLUDED.learn_display_mode,
    updated_at         = EXCLUDED.updated_at`
	res := r.db.WithContext(ctx).Exec(sql, userID, mode)
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: user preference: upsert learn display mode")
	}
	return nil
}

// toDomainUserPreference converts a raw database row to a domain.UserPreference.
// An empty or unrecognised learn_display_mode column value silently falls back
// to domain.DefaultLearnDisplayMode — the one deliberate exception to
// ParseLearnDisplayMode's "unknown value is a caller error" contract. The
// silent fallback keeps reads non-fatal during rolling deploys and for legacy
// rows written before the column existed; a corrupt value in production is
// surfaced only as the safe default, not a load error.
func toDomainUserPreference(g gormUserPreference) *domain.UserPreference {
	mode := domain.DefaultLearnDisplayMode
	if parsed, err := domain.ParseLearnDisplayMode(g.LearnDisplayMode); err == nil {
		mode = parsed
	}
	return &domain.UserPreference{
		UserID:                domain.UserID(g.UserID),
		LastViewedCardgroupID: g.LastViewedCardgroupID,
		LearnDisplayMode:      mode,
		UpdatedAt:             g.UpdatedAt,
	}
}
