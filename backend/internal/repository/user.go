package repository

import (
	"context"
	"errors"
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
	Version     int64     `gorm:"column:version"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (gormUser) TableName() string { return "users" }

// authUserLastSignIn is the projection for the auth.users last-sign-in read.
// It is not a full auth.users mapping — only the columns LastSignInByUserIDs
// needs.
type authUserLastSignIn struct {
	ID           string     `gorm:"column:id"`
	LastSignInAt *time.Time `gorm:"column:last_sign_in_at"`
}

// ErrNotFound is returned when a lookup or update targets a row that does not
// exist.
var ErrNotFound = errors.New("repository: not found")

// ErrConcurrentUpdate is returned when a versioned update targets an existing
// row whose version no longer matches the caller's expected version.
var ErrConcurrentUpdate = errors.New("repository: concurrent update")

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
	// UpdateTx applies the patch inside the caller-provided transaction. Unlike
	// Update, it does not re-fetch the row — callers that need the updated
	// value should refetch after the transaction commits. A patch with no
	// non-nil fields returns nil without touching the database. A patch that
	// targets a missing row returns ErrNotFound so the surrounding transaction
	// rolls back atomically.
	UpdateTx(ctx context.Context, tx *gorm.DB, id string, patch UserUpdate) error
	// UpdateTxVersioned applies the patch inside the caller-provided
	// transaction only when the row's current version matches expectedVersion.
	// It always issues an UPDATE and bumps version, even for an empty patch.
	// A missing row returns ErrNotFound; an existing row with a different
	// version returns ErrConcurrentUpdate.
	UpdateTxVersioned(ctx context.Context, tx *gorm.DB, id string, patch UserUpdate, expectedVersion int64) error
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

	// DeleteAuthUser deletes the auth.users row identified by id. The public.users
	// row and every cascade-linked row (user_roles, cardgroups → cards →
	// swipe_records / user_card_fsrs, user_preferences) are removed automatically
	// by the existing ON DELETE CASCADE foreign keys — no application-level
	// multi-step delete is needed. Returns ErrNotFound when no auth.users row
	// matches id (RowsAffected == 0).
	//
	// The operation requires the connection role to have DELETE privilege on
	// auth.users; the production DSN connects as the postgres role, which has it
	// (the same statement is exercised by the repository integration tests). This
	// is the single isolation point for auth-account deletion: swapping to the
	// Supabase Admin REST API is a change to this method body alone.
	DeleteAuthUser(ctx context.Context, id string) error

	// AuthUserExists reports whether an auth.users row with the given id exists.
	// It is the existence probe that lets a caller distinguish a deleted account
	// from a public.users row the handle_new_user trigger has not written yet —
	// both surface as ErrNotFound on a public.users read.
	//
	// Like DeleteAuthUser, this is an isolated auth.users access point; it
	// requires the connection role to have SELECT on auth.users (the production
	// postgres role has it).
	AuthUserExists(ctx context.Context, id string) (bool, error)

	// LastSignInByUserIDs reads auth.users.last_sign_in_at for the given user
	// ids. Each known id maps to its *time.Time (nil when the column is NULL,
	// i.e. the user has never signed in); ids without an auth.users row are
	// omitted from the map. Empty ids returns an empty map (GORM turns
	// "WHERE id IN ()" into an unfiltered full scan, so the call is
	// short-circuited).
	//
	// Like DeleteAuthUser, this is an isolated auth.users access point; it
	// requires the connection role to have SELECT on auth.users (the
	// production postgres role has it).
	LastSignInByUserIDs(ctx context.Context, ids []string) (map[string]*time.Time, error)
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
		return nil, eris.Wrap(err, "repository: user: find by id")
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
		return nil, eris.Wrap(err, "repository: user: find by ids")
	}
	out := make(map[string]*domain.User, len(rows))
	for i := range rows {
		u := userToDomain(rows[i])
		out[string(u.ID)] = u
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
		return nil, eris.Wrap(res.Error, "repository: user: update")
	}
	// Re-fetch so callers see the trigger-refreshed updated_at.
	return refetchAfterUpdate(res.RowsAffected, ErrNotFound,
		func() (*domain.User, error) { return r.FindByID(ctx, id) }, "")
}

func (r *userRepo) UpdateTx(ctx context.Context, tx *gorm.DB, id string, patch UserUpdate) error {
	updates := userUpdates(patch)
	if len(updates) == 0 {
		return nil
	}
	res := tx.WithContext(ctx).Model(&gormUser{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: user: update tx")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *userRepo) UpdateTxVersioned(ctx context.Context, tx *gorm.DB, id string, patch UserUpdate, expectedVersion int64) error {
	updates := userUpdates(patch)
	updates["version"] = gorm.Expr("version + 1")

	res := tx.WithContext(ctx).
		Model(&gormUser{}).
		Where("id = ? AND version = ?", id, expectedVersion).
		Updates(updates)
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: user: update tx versioned")
	}
	if res.RowsAffected > 0 {
		return nil
	}

	var row gormUser
	err := tx.WithContext(ctx).Select("id").Where("id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return eris.Wrap(err, "repository: user: update tx versioned: probe user")
	}
	return ErrConcurrentUpdate
}

// DeleteAuthUser deletes the auth.users row, cascading to public.users and all
// child data via the schema's ON DELETE CASCADE foreign keys. See the interface
// doc for the privilege and isolation rationale.
func (r *userRepo) DeleteAuthUser(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Exec("DELETE FROM auth.users WHERE id = ?", id)
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: user: delete auth user")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *userRepo) AuthUserExists(ctx context.Context, id string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("auth.users").
		Where("id = ?", id).
		Limit(1).
		Count(&count).Error; err != nil {
		return false, eris.Wrap(err, "repository: user: auth user exists")
	}
	return count > 0, nil
}

func (r *userRepo) LastSignInByUserIDs(ctx context.Context, ids []string) (map[string]*time.Time, error) {
	// GORM turns "WHERE id IN ()" into an unfiltered full scan, so short-circuit
	// empty input.
	if len(ids) == 0 {
		return map[string]*time.Time{}, nil
	}
	var rows []authUserLastSignIn
	if err := r.db.WithContext(ctx).
		Table("auth.users").
		Select("id", "last_sign_in_at").
		Where("id IN ?", ids).
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: user: last sign-in by ids")
	}
	out := make(map[string]*time.Time, len(rows))
	for i := range rows {
		out[rows[i].ID] = rows[i].LastSignInAt
	}
	return out, nil
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
	searchPattern, hasSearch := searchLikePattern(search)

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
		return nil, 0, eris.Wrap(err, "repository: user: count")
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
			return nil, 0, eris.Wrap(err, "repository: user: hydrate cursor")
		}

		clauseSQL, args := userCursorWhere(createdAtAsc, idAsc, cursorRow)
		q = q.Where(clauseSQL, args...)
	}

	q = q.Order(userOrderClause(createdAtAsc, idAsc)).Limit(limit)

	var rows []gormUser
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: user: list page")
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
		ID:          domain.UserID(g.ID),
		DisplayName: (*domain.DisplayName)(g.DisplayName),
		Bio:         domain.BioFromPtr(g.Bio),
		AvatarURL:   g.AvatarURL,
		Version:     g.Version,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
	}
}
