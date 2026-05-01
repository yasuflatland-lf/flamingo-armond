package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rotisserie/eris"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"backend/internal/domain"
)

// ErrUserNotFound and ErrRoleNotFound are returned by AssignToUser to
// distinguish which side of the (user, role) pair was missing. Both are
// joined with ErrNotFound so callers that previously branched on the legacy
// sentinel via errors.Is keep working; new callers can branch on the
// specific cause to surface a more precise BAD_USER_INPUT field.
//
// ErrRoleDuplicate is returned by Create and Update when the requested role
// name already exists. It is a standalone sentinel — not joined with
// ErrNotFound — because a duplicate is a "found" condition, not a "missing"
// one.
//
// Plain errors.New (not eris) so errors.Is walks identity directly.
var (
	errUserNotFoundBase = errors.New("repository: user not found")
	errRoleNotFoundBase = errors.New("repository: role not found")

	// ErrUserNotFound matches both itself and ErrNotFound.
	ErrUserNotFound = errors.Join(errUserNotFoundBase, ErrNotFound)
	// ErrRoleNotFound matches both itself and ErrNotFound.
	ErrRoleNotFound = errors.Join(errRoleNotFoundBase, ErrNotFound)

	// ErrRoleDuplicate is returned when a role with the same name already exists.
	// It is a standalone sentinel: duplicate is a found-condition, not a
	// not-found-condition, so it is not joined with ErrNotFound.
	ErrRoleDuplicate = errors.New("repository: role already exists")
)

// gormUserRoleJoinRow is the projected shape of the join between user_roles
// and roles used by ListByUserIDs.
type gormUserRoleJoinRow struct {
	UserID string `gorm:"column:user_id"`
	ID     string `gorm:"column:id"`
	Name   string `gorm:"column:name"`
}

type gormRole struct {
	ID   string `gorm:"column:id;primaryKey;type:uuid"`
	Name string `gorm:"column:name"`
}

func (gormRole) TableName() string { return "roles" }

type RoleRepository interface {
	// FindByID returns the role with the given id. Returns ErrRoleNotFound
	// (which also satisfies ErrNotFound) when no matching row exists.
	FindByID(ctx context.Context, id string) (*domain.Role, error)

	FindByName(ctx context.Context, name string) (*domain.Role, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Role, error)

	// Create inserts a new role with the given name. The name is normalised
	// (lower-cased, trimmed) before insertion. Returns ErrRoleDuplicate when a
	// role with the same normalised name already exists.
	Create(ctx context.Context, name string) (*domain.Role, error)

	// Update replaces the name of the role identified by id. The new name is
	// normalised before being stored. Returns ErrRoleNotFound when the id does
	// not exist, and ErrRoleDuplicate when the normalised new name collides with
	// an existing role.
	Update(ctx context.Context, id, name string) (*domain.Role, error)

	// Delete removes the role with the given id. Because user_roles carries an
	// ON DELETE CASCADE FK on roles.id, all user-role assignments for this role
	// are also deleted. Returns ErrRoleNotFound when no matching row exists.
	Delete(ctx context.Context, id string) error

	// AssignToUser inserts a (user_id, role_id) row. Idempotent: if the row
	// already exists, returns nil without error. Returns ErrUserNotFound when
	// the user is missing and ErrRoleNotFound when the role is missing; both
	// also satisfy errors.Is(_, ErrNotFound) for backward-compatible matching.
	AssignToUser(ctx context.Context, userID, roleID string) error

	// RevokeFromUser deletes the (user_id, role_id) row. Validates that the
	// user and role exist; returns ErrUserNotFound / ErrRoleNotFound when
	// either is missing. When both exist but no assignment link is present,
	// returns nil without error (idempotent on the assignment row).
	RevokeFromUser(ctx context.Context, userID, roleID string) error

	// ListByUser returns all roles assigned to the given user, ordered by
	// role name ascending. Returns an empty slice (not nil, not an error) when
	// the user has no roles.
	ListByUser(ctx context.Context, userID string) ([]*domain.Role, error)

	// ListByUserIDs returns the roles assigned to each user ID in a single
	// query. The map key is the user ID; values are roles ordered by
	// name ASC. Users with no roles are absent from the map (callers
	// fill in an empty slice). An empty userIDs slice short-circuits to
	// an empty map without issuing a query — see the GORM empty-IN gotcha
	// in .claude/rules/go-library-gotchas.md.
	ListByUserIDs(ctx context.Context, userIDs []string) (map[string][]*domain.Role, error)

	// ListAll returns every role ordered by name ASC. Returns an empty
	// slice (never nil) when no roles exist.
	ListAll(ctx context.Context) ([]*domain.Role, error)
}

type roleRepo struct{ db *gorm.DB }

func NewRoleRepository(db *gorm.DB) RoleRepository { return &roleRepo{db: db} }

func (r *roleRepo) FindByID(ctx context.Context, id string) (*domain.Role, error) {
	var row gormRole
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, eris.Wrap(err, "repository: find role by id")
	}
	return roleToDomain(row), nil
}

func (r *roleRepo) FindByName(ctx context.Context, name string) (*domain.Role, error) {
	var row gormRole
	err := r.db.WithContext(ctx).Where("name = ?", name).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, eris.Wrap(err, "repository: find role by name")
	}
	return roleToDomain(row), nil
}

func (r *roleRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Role, error) {
	if len(ids) == 0 {
		return map[string]*domain.Role{}, nil
	}
	var rows []gormRole
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find roles by ids")
	}
	out := make(map[string]*domain.Role, len(rows))
	for i := range rows {
		role := roleToDomain(rows[i])
		out[role.ID] = role
	}
	return out, nil
}

// Create inserts a new role. The name is normalised (lower-cased, trimmed)
// defensively even if the usecase already did so. Returns ErrRoleDuplicate when
// a role with the same normalised name already exists.
//
// uuid.NewV7 errors are propagated: the failure mode is a system-level issue
// (crypto/rand unavailable) — a silent fallback would produce a different UUID
// on a still-broken source. See .claude/rules/go-library-gotchas.md.
func (r *roleRepo) Create(ctx context.Context, name string) (*domain.Role, error) {
	name = strings.ToLower(strings.TrimSpace(name))

	id, err := uuid.NewV7()
	if err != nil {
		return nil, eris.Wrap(err, "repository: role: create: uuid")
	}

	row := gormRole{ID: id.String(), Name: name}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		if classified := classifyUniqueError(err); classified != nil {
			return nil, classified
		}
		return nil, eris.Wrap(err, "repository: role: create")
	}
	return roleToDomain(row), nil
}

// Update replaces the name of the role identified by id. The name is normalised
// before storing. Returns ErrRoleNotFound when the id does not match any row,
// and ErrRoleDuplicate when the normalised new name collides with an existing role.
func (r *roleRepo) Update(ctx context.Context, id, name string) (*domain.Role, error) {
	name = strings.ToLower(strings.TrimSpace(name))

	var row gormRole
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, eris.Wrap(err, "repository: role: update: find")
	}

	row.Name = name
	if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
		if classified := classifyUniqueError(err); classified != nil {
			return nil, classified
		}
		return nil, eris.Wrap(err, "repository: role: update")
	}
	return roleToDomain(row), nil
}

// Delete removes the role with the given id. Because user_roles carries
// ON DELETE CASCADE on roles.id, all user-role assignments for this role are
// also removed atomically by the DB. Returns ErrRoleNotFound when no matching
// row exists (RowsAffected == 0).
func (r *roleRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&gormRole{})
	if result.Error != nil {
		return eris.Wrap(result.Error, "repository: role: delete")
	}
	if result.RowsAffected == 0 {
		return ErrRoleNotFound
	}
	return nil
}

// classifyUniqueError inspects a Postgres unique-violation (code 23505) and
// maps it to ErrRoleDuplicate when the violated constraint is on the roles name
// column. Returns nil when err is not a unique-violation so callers can use it
// as a pre-filter before falling through to eris.Wrap.
func classifyUniqueError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return nil
	}
	if strings.Contains(pgErr.ConstraintName, "name") {
		return ErrRoleDuplicate
	}
	return nil
}

// classifyFKError inspects a Postgres FK violation (code 23503) and maps it
// to ErrUserNotFound or ErrRoleNotFound based on which constraint was violated.
// Returns nil when err is not a FK violation, so callers can use it as a
// pre-filter before falling through to eris.Wrap. This helper exists so the
// FK-classification logic can be unit-tested with a fabricated *pgconn.PgError
// without needing a live DB race.
func classifyFKError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return nil
	}
	if strings.Contains(pgErr.ConstraintName, "user_id") {
		return ErrUserNotFound
	}
	if strings.Contains(pgErr.ConstraintName, "role_id") {
		return ErrRoleNotFound
	}
	return nil
}

// requireExists returns ErrUserNotFound / ErrRoleNotFound when no row matches
// the given primary key. The wrap label feeds into the eris chain when the
// underlying COUNT query itself fails. notFound is the sentinel returned for
// the zero-count case.
func (r *roleRepo) requireExists(ctx context.Context, table, id, wrap string, notFound error) error {
	var count int64
	if err := r.db.WithContext(ctx).
		Table(table).
		Where("id = ?", id).
		Count(&count).Error; err != nil {
		return eris.Wrap(err, wrap)
	}
	if count == 0 {
		return notFound
	}
	return nil
}

// AssignToUser inserts a user_roles row. Idempotent via ON CONFLICT DO NOTHING.
// Validates that both the user and role exist before inserting; returns
// ErrUserNotFound when the user is missing and ErrRoleNotFound when the role
// is missing. Both sentinels also satisfy errors.Is(_, ErrNotFound) so legacy
// callers that only branch on ErrNotFound keep working — new callers should
// match the specific sentinel first to surface the correct field in
// BAD_USER_INPUT responses.
//
// FK race: if the user or role is deleted between the existence check and the
// INSERT (concurrent admin operation), the resulting Postgres FK violation
// (23503) is classified by classifyFKError into the same sentinels, so the
// caller still receives BAD_USER_INPUT rather than INTERNAL.
func (r *roleRepo) AssignToUser(ctx context.Context, userID, roleID string) error {
	if err := r.requireExists(ctx, "users", userID, "repository: assign role: check user", ErrUserNotFound); err != nil {
		return err
	}
	if err := r.requireExists(ctx, "roles", roleID, "repository: assign role: check role", ErrRoleNotFound); err != nil {
		return err
	}

	row := gormUserRole{UserID: userID, RoleID: roleID}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&row).Error; err != nil {
		// A concurrent delete between the existence check and the INSERT can
		// produce a FK violation. Classify it into the appropriate sentinel
		// so the caller sees BAD_USER_INPUT rather than INTERNAL.
		if classified := classifyFKError(err); classified != nil {
			return classified
		}
		return eris.Wrap(err, "repository: assign role to user")
	}
	return nil
}

// RevokeFromUser deletes the (user_id, role_id) row. Validates that the user
// and role both exist before attempting the delete; returns ErrUserNotFound
// when the user is missing and ErrRoleNotFound when the role is missing. When
// both exist but no assignment row is present, the operation is a silent
// no-op (idempotent on the assignment link itself).
func (r *roleRepo) RevokeFromUser(ctx context.Context, userID, roleID string) error {
	if err := r.requireExists(ctx, "users", userID, "repository: revoke role: check user", ErrUserNotFound); err != nil {
		return err
	}
	if err := r.requireExists(ctx, "roles", roleID, "repository: revoke role: check role", ErrRoleNotFound); err != nil {
		return err
	}

	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND role_id = ?", userID, roleID).
		Delete(&gormUserRole{}).Error; err != nil {
		return eris.Wrap(err, "repository: revoke role from user")
	}
	return nil
}

// ListByUser returns all roles assigned to userID, ordered by name ascending.
// Returns an empty slice (never nil) when the user has no roles.
func (r *roleRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Role, error) {
	var rows []gormRole
	if err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Order("roles.name ASC").
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: list roles by user")
	}
	out := make([]*domain.Role, len(rows))
	for i := range rows {
		out[i] = roleToDomain(rows[i])
	}
	return out, nil
}

// ListByUserIDs returns the roles assigned to each user ID in a single query.
// Returns an empty map (never nil) when userIDs is empty.
func (r *roleRepo) ListByUserIDs(ctx context.Context, userIDs []string) (map[string][]*domain.Role, error) {
	if len(userIDs) == 0 {
		return map[string][]*domain.Role{}, nil
	}

	var rows []gormUserRoleJoinRow
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Select("user_roles.user_id AS user_id, roles.id AS id, roles.name AS name").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id IN ?", userIDs).
		Order("roles.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, eris.Wrap(err, "repository: list roles by user ids")
	}

	out := make(map[string][]*domain.Role, len(rows))
	for i := range rows {
		out[rows[i].UserID] = append(out[rows[i].UserID], &domain.Role{
			ID:   rows[i].ID,
			Name: rows[i].Name,
		})
	}
	return out, nil
}

// ListAll returns every role ordered by name ASC. Returns an empty slice
// (never nil) when the roles table is empty.
func (r *roleRepo) ListAll(ctx context.Context) ([]*domain.Role, error) {
	var rows []gormRole
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "role repo: list all")
	}
	out := make([]*domain.Role, len(rows))
	for i := range rows {
		out[i] = roleToDomain(rows[i])
	}
	return out, nil
}

func roleToDomain(g gormRole) *domain.Role {
	return &domain.Role{
		ID:   g.ID,
		Name: g.Name,
	}
}
