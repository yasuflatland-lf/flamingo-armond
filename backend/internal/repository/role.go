package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/rotisserie/eris"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"backend/internal/domain"
)

// ErrUserNotFound and ErrRoleNotFound distinguish which side of a (user, role)
// pair was missing. Both are joined with ErrNotFound so legacy callers that
// match the general sentinel keep working; new callers can branch on the
// specific cause to surface a more precise BAD_USER_INPUT field. ErrRoleDuplicate
// is standalone — a duplicate is a "found" condition, not a "missing" one.
//
// Plain errors.New (not eris) so errors.Is walks identity directly.
var (
	errUserNotFoundBase = errors.New("repository: user not found")
	errRoleNotFoundBase = errors.New("repository: role not found")

	ErrUserNotFound  = errors.Join(errUserNotFoundBase, ErrNotFound)
	ErrRoleNotFound  = errors.Join(errRoleNotFoundBase, ErrNotFound)
	ErrRoleDuplicate = errors.New("repository: role already exists")
)

type gormRole struct {
	ID   string `gorm:"column:id;primaryKey;type:uuid"`
	Name string `gorm:"column:name"`
}

func (gormRole) TableName() string { return "roles" }

type RoleRepository interface {
	// FindByID returns the role with the given id. Returns ErrRoleNotFound
	// (which also satisfies ErrNotFound) when no matching row exists.
	FindByID(ctx context.Context, id string) (*domain.Role, error)

	FindByName(ctx context.Context, name domain.RoleName) (*domain.Role, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Role, error)

	// FindByIDsTx returns the roles for ids inside the supplied transaction,
	// locking the matched rows FOR UPDATE so a concurrent rename/delete of any
	// returned role blocks until the caller's transaction commits. Used by the
	// self-demotion guard, which must read role names so no other transaction
	// can rename or delete them before the user_roles write that follows.
	// An empty ids slice returns an empty map with no SQL and no lock acquired.
	FindByIDsTx(ctx context.Context, tx *gorm.DB, ids []string) (map[string]*domain.Role, error)

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

func (r *roleRepo) FindByName(ctx context.Context, name domain.RoleName) (*domain.Role, error) {
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
	return findRolesByIDs(ctx, r.db, ids, false)
}

func (r *roleRepo) FindByIDsTx(ctx context.Context, tx *gorm.DB, ids []string) (map[string]*domain.Role, error) {
	if tx == nil {
		return nil, eris.New("repository: find roles by ids tx is nil")
	}
	return findRolesByIDs(ctx, tx, ids, true)
}

// findRolesByIDs is shared by FindByIDs (pool, no lock) and FindByIDsTx
// (transaction, FOR UPDATE). lock=true acquires a row lock on the matched
// rows so no other transaction can rename or delete them before the caller's
// write commits.
func findRolesByIDs(ctx context.Context, db *gorm.DB, ids []string, lock bool) (map[string]*domain.Role, error) {
	if len(ids) == 0 {
		return map[string]*domain.Role{}, nil
	}
	q := db.WithContext(ctx).Where("id IN ?", ids)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var rows []gormRole
	if err := q.Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find roles by ids")
	}
	out := make(map[string]*domain.Role, len(rows))
	for i := range rows {
		role := roleToDomain(rows[i])
		out[role.ID] = role
	}
	return out, nil
}

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

func (r *roleRepo) Update(ctx context.Context, id, name string) (*domain.Role, error) {
	name = strings.ToLower(strings.TrimSpace(name))

	res := r.db.WithContext(ctx).
		Model(&gormRole{}).
		Where("id = ?", id).
		Updates(map[string]any{"name": name})
	if res.Error != nil {
		if classified := classifyUniqueError(res.Error); classified != nil {
			return nil, classified
		}
		return nil, eris.Wrap(res.Error, "repository: role: update")
	}
	return refetchAfterUpdate(res.RowsAffected, ErrRoleNotFound,
		func() (*domain.Role, error) { return r.FindByID(ctx, id) },
		"repository: role: update: find after update")
}

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

// classifyUniqueError maps a Postgres unique-violation (code 23505) on the
// roles.name column to ErrRoleDuplicate. Returns nil for any other error so
// callers can use it as a pre-filter before falling through to eris.Wrap.
func classifyUniqueError(err error) error {
	if pgConstraintViolation(err, "23505", "name") {
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
	if pgConstraintViolation(err, "23503", "user_id") {
		return ErrUserNotFound
	}
	if pgConstraintViolation(err, "23503", "role_id") {
		return ErrRoleNotFound
	}
	return nil
}

func (r *roleRepo) ListAll(ctx context.Context) ([]*domain.Role, error) {
	var rows []gormRole
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: role: list all")
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
		Name: domain.RoleName(g.Name),
	}
}
