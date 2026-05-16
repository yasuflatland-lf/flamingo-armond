package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

// gormUserPreference is the row mapping for public.user_preferences.
// Package-private so callers cannot bypass the domain conversion.
type gormUserPreference struct {
	UserID                string    `gorm:"column:user_id;primaryKey;type:uuid"`
	LastViewedCardgroupID *string   `gorm:"column:last_viewed_cardgroup_id;type:uuid"`
	UpdatedAt             time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (gormUserPreference) TableName() string { return "user_preferences" }

// ErrCardgroupNotFound is returned when UpsertLastViewedCardgroup targets a
// cardgroup that is missing OR not owned by the calling user. Joined with
// ErrNotFound so legacy callers that match the general sentinel keep working.
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
}

type userPreferenceRepo struct{ db *gorm.DB }

// NewUserPreferenceRepository returns a GORM-backed UserPreferenceRepository.
func NewUserPreferenceRepository(db *gorm.DB) UserPreferenceRepository {
	return &userPreferenceRepo{db: db}
}

// FindByUserID returns the preference row for userID, or ErrNotFound when no
// row exists (i.e. the user has never set any preference).
func (r *userPreferenceRepo) FindByUserID(ctx context.Context, userID string) (*domain.UserPreference, error) {
	var row gormUserPreference
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: find user preference by user_id")
	}
	return toDomainUserPreference(row), nil
}

// FindByUserIDs returns preference rows for all given userIDs. The returned
// slice contains only rows that exist; missing users are absent. Short-circuits
// on an empty input to avoid the GORM WHERE IN () full-table scan bug.
func (r *userPreferenceRepo) FindByUserIDs(ctx context.Context, userIDs []string) ([]*domain.UserPreference, error) {
	// GORM turns WHERE user_id IN () into an unfiltered scan, so short-circuit.
	if len(userIDs) == 0 {
		return []*domain.UserPreference{}, nil
	}
	var rows []gormUserPreference
	if err := r.db.WithContext(ctx).Where("user_id IN ?", userIDs).Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find user preferences by user_ids")
	}
	out := make([]*domain.UserPreference, len(rows))
	for i := range rows {
		out[i] = toDomainUserPreference(rows[i])
	}
	return out, nil
}

// UpsertLastViewedCardgroup performs an ownership-checked UPSERT in a single
// SQL statement. The INSERT fires only when a cardgroup row exists with
// id = cardgroupID AND owner_id = userID. A missing or non-owned cardgroup
// yields RowsAffected == 0 (the ON CONFLICT branch does not fire when the
// SELECT returns no rows), which is reported as ErrCardgroupNotFound. A
// concurrent deletion between the EXISTS evaluation and the write produces a
// Postgres FK violation (23503), classified to the same sentinel by
// classifyUserPreferenceCardgroupFKError.
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
		return eris.Wrap(res.Error, "repository: upsert last viewed cardgroup")
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
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return nil
	}
	if strings.Contains(pgErr.ConstraintName, "last_viewed_cardgroup_id") {
		return ErrCardgroupNotFound
	}
	return nil
}

// toDomainUserPreference converts a gormUserPreference row to the domain type.
func toDomainUserPreference(g gormUserPreference) *domain.UserPreference {
	return &domain.UserPreference{
		UserID:                g.UserID,
		LastViewedCardgroupID: g.LastViewedCardgroupID,
		UpdatedAt:             g.UpdatedAt,
	}
}
