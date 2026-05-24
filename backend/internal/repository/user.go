package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

// gormUser is the row mapping for public.users. Package-private so
// callers cannot bypass the domain conversion.
type gormUser struct {
	ID          string    `gorm:"column:id;primaryKey;type:uuid"`
	DisplayName *string   `gorm:"column:display_name"`
	Bio         *string   `gorm:"column:bio"`
	AvatarURL   *string   `gorm:"column:avatar_url"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (gormUser) TableName() string { return "users" }

// ErrNotFound is returned when a lookup or update targets a row that does not
// exist.
var ErrNotFound = errors.New("repository: not found")

// ErrCursorNotFound is returned by paginated queries when the supplied cursor
// references a user that no longer exists (e.g. deleted between page fetches).
// Plain errors.New so callers can branch with errors.Is without going through
// eris's chain walk.
var ErrCursorNotFound = errors.New("user pagination: cursor user not found")

// UserUpdate carries patch fields. nil means "leave untouched"; a non-nil
// pointer to "" is a request to clear the column.
type UserUpdate struct {
	DisplayName *string
	Bio         *string
	AvatarURL   *string
}

type UserRepository interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.User, error)
	Update(ctx context.Context, id string, patch UserUpdate) (*domain.User, error)
	UpdateTx(ctx context.Context, tx *gorm.DB, id string, patch UserUpdate) error
	// ListPage returns a page of users ordered by created_at DESC with id ASC
	// as a stable tiebreaker. The cursor is the user UUID. Forward paging uses
	// `after` (exclusive); backward paging uses `before` (exclusive). `first`
	// and `last` are mutually exclusive at the usecase layer; this method
	// accepts both and lets the usecase enforce.
	//
	// search is a substring match on display_name (case-insensitive ILIKE).
	// Whitespace-only input is treated as nil. `%` and `_` literals in the
	// search term are escaped so they match literally.
	//
	// total is computed via a separate COUNT(*) scoped by the same search
	// predicate as the page query (cursor predicate excluded).
	//
	// Repository caps page size at PageCap so callers can use the
	// usecase-level +1 fetch trick at the documented maximum.
	ListPage(
		ctx context.Context,
		after, before *string,
		first, last int,
		search *string,
	) (users []*domain.User, total int64, err error)
}

type userRepo struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) UserRepository { return &userRepo{db: db} }

func (r *userRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var row gormUser
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: find user by id")
	}
	return userToDomain(row), nil
}

func (r *userRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.User, error) {
	// GORM turns WHERE id IN () into an unfiltered scan, so short-circuit empty input.
	if len(ids) == 0 {
		return map[string]*domain.User{}, nil
	}
	var rows []gormUser
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find users by ids")
	}
	out := make(map[string]*domain.User, len(rows))
	for i := range rows {
		u := userToDomain(rows[i])
		out[u.ID] = u
	}
	return out, nil
}

func (r *userRepo) Update(ctx context.Context, id string, patch UserUpdate) (*domain.User, error) {
	updates := userUpdates(patch)
	if len(updates) == 0 {
		// No-op patch: return current value rather than touching DB.
		return r.FindByID(ctx, id)
	}

	res := r.db.WithContext(ctx).Model(&gormUser{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, eris.Wrap(res.Error, "repository: update user")
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	// Re-fetch so callers see the trigger-refreshed updated_at.
	return r.FindByID(ctx, id)
}

func (r *userRepo) UpdateTx(ctx context.Context, tx *gorm.DB, id string, patch UserUpdate) error {
	updates := userUpdates(patch)
	if len(updates) == 0 {
		return nil
	}
	res := tx.WithContext(ctx).Model(&gormUser{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: update user tx")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func userUpdates(patch UserUpdate) map[string]any {
	updates := map[string]any{}
	if patch.DisplayName != nil {
		updates["display_name"] = *patch.DisplayName
	}
	if patch.Bio != nil {
		updates["bio"] = *patch.Bio
	}
	if patch.AvatarURL != nil {
		updates["avatar_url"] = *patch.AvatarURL
	}
	return updates
}

// ListPage implements the cursor-paginated user list. Order is fixed at
// (created_at DESC, id ASC) so cursors stay deterministic even when multiple
// users share a created_at. Backward paging inverts the direction, applies
// LIMIT, then reverses the slice in memory so the caller sees the same display
// order as forward paging.
func (r *userRepo) ListPage(
	ctx context.Context,
	after, before *string,
	first, last int,
	search *string,
) ([]*domain.User, int64, error) {
	if first < 0 || last < 0 {
		return nil, 0, eris.New("user repo: first/last must be >= 0")
	}
	first = ClampPageSize(first)
	last = ClampPageSize(last)

	// Normalise search: trim, treat blank as nil, escape ILIKE wildcards so
	// "%" and "_" supplied by the caller match literally.
	var searchPattern string
	hasSearch := false
	if search != nil {
		trimmed := strings.TrimSpace(*search)
		if trimmed != "" {
			searchPattern = "%" + escapeLikePattern(trimmed) + "%"
			hasSearch = true
		}
	}

	// totalCount mirrors the page predicate (search only) but ignores the
	// cursor predicate, so callers can compute "items after this point" /
	// "page X of Y" without a second round-trip. Run before the zero-page
	// short-circuit so callers asking only for totalCount still get a real
	// value.
	countQ := r.db.WithContext(ctx).Model(&gormUser{})
	if hasSearch {
		countQ = countQ.Where("display_name ILIKE ?", searchPattern)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: count users")
	}

	if first == 0 && last == 0 {
		return []*domain.User{}, total, nil
	}

	// Backward pagination: invert the SQL order, fetch, then reverse the
	// slice so the caller still observes (created_at DESC, id ASC).
	// Forward direction is (created_at DESC, id ASC); for backward we flip
	// both axes.
	createdAtAsc := false
	idAsc := true
	limit := first
	cursorID := after
	reverse := false
	if last > 0 {
		createdAtAsc = !createdAtAsc
		idAsc = !idAsc
		limit = last
		cursorID = before
		reverse = true
	}

	q := r.db.WithContext(ctx).Model(&gormUser{})
	if hasSearch {
		q = q.Where("display_name ILIKE ?", searchPattern)
	}

	if cursorID != nil {
		// Hydrate the cursor user's created_at so we can build the tuple
		// comparison. A missing user means the cursor row was deleted between
		// fetches — surface as ErrCursorNotFound so callers can map to a
		// BAD_USER_INPUT-shaped error.
		var cursorRow gormUser
		err := r.db.WithContext(ctx).Select("id", "created_at").
			Where("id = ?", *cursorID).Take(&cursorRow).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, 0, ErrCursorNotFound
			}
			return nil, 0, eris.Wrap(err, "repository: hydrate user cursor")
		}

		clauseSQL, args := userCursorWhere(createdAtAsc, idAsc, cursorRow)
		q = q.Where(clauseSQL, args...)
	}

	q = q.Order(userOrderClause(createdAtAsc, idAsc)).Limit(limit)

	var rows []gormUser
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: list users page")
	}

	if reverse {
		ReverseSlice(rows)
	}

	out := make([]*domain.User, len(rows))
	for i := range rows {
		out[i] = userToDomain(rows[i])
	}
	return out, total, nil
}

// userOrderClause renders the SQL ORDER BY tail for the configured axis
// directions. The default forward order is (created_at DESC, id ASC).
func userOrderClause(createdAtAsc, idAsc bool) string {
	caDir := "DESC"
	if createdAtAsc {
		caDir = "ASC"
	}
	idDir := "ASC"
	if !idAsc {
		idDir = "DESC"
	}
	return "created_at " + caDir + ", id " + idDir
}

// userCursorWhere builds the tuple-comparison WHERE for (created_at, id)
// against the supplied cursor row. The expanded form `field op ? OR (field = ?
// AND id op ?)` is portable across SQL dialects (the row-constructor
// `(a, b) > (?, ?)` is Postgres-only).
//
// For forward paging (created_at DESC, id ASC) we want rows strictly past the
// cursor — so we emit:
//
//	created_at < cursor.created_at OR (created_at = cursor.created_at AND id > cursor.id)
//
// For backward paging the direction flips on both axes.
func userCursorWhere(createdAtAsc, idAsc bool, cursor gormUser) (string, []any) {
	caOp := "<"
	if createdAtAsc {
		caOp = ">"
	}
	idOp := ">"
	if !idAsc {
		idOp = "<"
	}
	clauseSQL := "(created_at " + caOp + " ? OR (created_at = ? AND id " + idOp + " ?))"
	return clauseSQL, []any{cursor.CreatedAt, cursor.CreatedAt, cursor.ID}
}

func userToDomain(g gormUser) *domain.User {
	// The gormUser DisplayName field is *string (DB column type); the domain
	// DisplayName is a string newtype, so (*domain.DisplayName)(g.DisplayName)
	// is a straight pointer cast across the same underlying type. Bio is the
	// trinary VO; BioFromPtr maps a NULL column to Bio{} (IsSet=false) and a
	// text column to a set Bio whose Ptr() returns a defensive copy with the same string value.
	return &domain.User{
		ID:          g.ID,
		DisplayName: (*domain.DisplayName)(g.DisplayName),
		Bio:         domain.BioFromPtr(g.Bio),
		AvatarURL:   g.AvatarURL,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
	}
}
