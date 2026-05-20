package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rotisserie/eris"
	"gorm.io/gorm"

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
	if res.RowsAffected == 0 {
		return nil, ErrRoleNotFound
	}
	role, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, eris.Wrap(err, "repository: role: update: find after update")
	}
	return role, nil
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
