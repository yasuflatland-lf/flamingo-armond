package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/logging"
)

// gormUserPreference is the row mapping for public.user_preferences.
// Package-private so callers cannot bypass the domain conversion.
// All writes go through raw UPSERT with now(); no GORM auto-fill.
type gormUserPreference struct {
	UserID                string    `gorm:"column:user_id;primaryKey;type:uuid"`
	LastViewedCardgroupID *string   `gorm:"column:last_viewed_cardgroup_id;type:uuid"`
	LearnDisplayMode      string    `gorm:"column:learn_display_mode"`
	NewCardRatioNum       int       `gorm:"column:new_card_ratio_num"`
	NewCardRatioDen       int       `gorm:"column:new_card_ratio_den"`
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
	// UpsertLearnDisplayMode upserts userID's learn display mode. The mode string
	// is the persisted form of a domain.LearnDisplayMode converted via
	// mode.String() at the usecase call site; unvalidated strings are never passed
	// here. The column CHECK constraint is a backstop for direct DB writes that
	// bypass the usecase layer.
	UpsertLearnDisplayMode(ctx context.Context, userID, mode string) error
	// UpsertNewCardRatio upserts the userID's new-card ratio. num/den are the reduced
	// fraction from a domain.NewCardRatio (numerator = new share, denominator =
	// total); the VO guarantees the invariant before this is called. The column
	// CHECK is a backstop for direct DB writes.
	UpsertNewCardRatio(ctx context.Context, userID string, num, den int) error
}

type userPreferenceRepo struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewUserPreferenceRepository builds the repository. logger receives the
// read-path WARN emitted when a stored new-card ratio fails domain validation;
// a nil logger falls back to slog.Default() so callers that do not thread one
// keep working.
func NewUserPreferenceRepository(db *gorm.DB, logger *slog.Logger) UserPreferenceRepository {
	if logger == nil {
		logger = slog.Default()
	}
	return &userPreferenceRepo{db: db, logger: logger}
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
	return toDomainUserPreference(ctx, r.logger, row), nil
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
		out[i] = toDomainUserPreference(ctx, r.logger, rows[i])
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

// classifyUserPreferenceCardgroupFKError maps a last-viewed-cardgroup FK
// violation or malformed cardgroup id to ErrCardgroupNotFound. The former
// closes the delete-after-EXISTS race; the latter enforces the id contract.
// Unrelated errors return nil so the caller can fall through to eris.Wrap.
func classifyUserPreferenceCardgroupFKError(err error) error {
	// Do not apply SQLSTATE 22P02 where another client-controlled bind could fail; cardgroupID is the only one here.
	if pgInvalidTextRepresentation(err) {
		return ErrCardgroupNotFound
	}
	if pgConstraintViolation(err, "23503", "last_viewed_cardgroup_id") {
		return ErrCardgroupNotFound
	}
	return nil
}

func (r *userPreferenceRepo) UpsertLearnDisplayMode(ctx context.Context, userID, mode string) error {
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

func (r *userPreferenceRepo) UpsertNewCardRatio(ctx context.Context, userID string, num, den int) error {
	sql := `INSERT INTO user_preferences (user_id, new_card_ratio_num, new_card_ratio_den, updated_at)
VALUES (?, ?, ?, now())
ON CONFLICT (user_id) DO UPDATE
SET new_card_ratio_num = EXCLUDED.new_card_ratio_num,
    new_card_ratio_den = EXCLUDED.new_card_ratio_den,
    updated_at         = EXCLUDED.updated_at`
	res := r.db.WithContext(ctx).Exec(sql, userID, num, den)
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: user preference: upsert new card ratio")
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
//
// new_card_ratio_num / new_card_ratio_den follow the same non-fatal posture:
// an out-of-bounds or legacy zero value falls back to domain.DefaultNewCardRatio
// rather than failing the read. This is a DB-read normalization, not the owner
// of the "zero means default" business rule — that rule lives in
// domain.UserPreference.EffectiveNewCardRatio, which read paths call. Keeping
// the normalization here means a corrupt or legacy stored value never leaves
// the repository as an invalid zero; the domain method is the backstop for any
// UserPreference not constructed through this mapper.
//
// A rejected ratio is now logged at WARN with the offending num/den pair through
// the injected logger, so a corrupt or out-of-range row is discoverable in logs
// instead of vanishing into the default.
func toDomainUserPreference(ctx context.Context, logger *slog.Logger, g gormUserPreference) *domain.UserPreference {
	mode := domain.DefaultLearnDisplayMode
	if parsed, err := domain.ParseLearnDisplayMode(g.LearnDisplayMode); err == nil {
		mode = parsed
	}
	ratio := domain.DefaultNewCardRatio
	if parsed, err := domain.ParseNewCardRatio(g.NewCardRatioNum, g.NewCardRatioDen); err == nil {
		ratio = parsed
	} else if g.NewCardRatioNum != 0 || g.NewCardRatioDen != 0 {
		// Zero is the legacy "unset" sentinel, absorbed silently; anything else is a row
		// the CHECK should have rejected and is worth surfacing.
		logging.LogWarn(ctx, logger,
			"repository: user preference: stored new card ratio rejected",
			err,
			slog.String("user_id", g.UserID),
			slog.Int("new_card_ratio_num", g.NewCardRatioNum),
			slog.Int("new_card_ratio_den", g.NewCardRatioDen),
		)
	}
	return &domain.UserPreference{
		UserID:                domain.UserID(g.UserID),
		LastViewedCardgroupID: g.LastViewedCardgroupID,
		LearnDisplayMode:      mode,
		NewCardRatio:          ratio,
		UpdatedAt:             g.UpdatedAt,
	}
}
