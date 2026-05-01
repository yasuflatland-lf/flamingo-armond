package repository

import (
	"context"
	"errors"

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
// Plain errors.New (not eris) so errors.Is walks identity directly.
var (
	errUserNotFoundBase = errors.New("repository: user not found")
	errRoleNotFoundBase = errors.New("repository: role not found")

	// ErrUserNotFound matches both itself and ErrNotFound.
	ErrUserNotFound = errors.Join(errUserNotFoundBase, ErrNotFound)
	// ErrRoleNotFound matches both itself and ErrNotFound.
	ErrRoleNotFound = errors.Join(errRoleNotFoundBase, ErrNotFound)
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
	FindByName(ctx context.Context, name string) (*domain.Role, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Role, error)

	// AssignToUser inserts a (user_id, role_id) row. Idempotent: if the row
	// already exists, returns nil without error. Returns ErrUserNotFound when
	// the user is missing and ErrRoleNotFound when the role is missing; both
	// also satisfy errors.Is(_, ErrNotFound) for backward-compatible matching.
	AssignToUser(ctx context.Context, userID, roleID string) error

	// RevokeFromUser deletes the (user_id, role_id) row. Idempotent: if the row
	// does not exist, returns nil without error.
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

func (r *roleRepo) FindByName(ctx context.Context, name string) (*domain.Role, error) {
	var row gormRole
	err := r.db.WithContext(ctx).Where("name = ?", name).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
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

// AssignToUser inserts a user_roles row. Idempotent via ON CONFLICT DO NOTHING.
// Validates that both the user and role exist before inserting; returns
// ErrUserNotFound when the user is missing and ErrRoleNotFound when the role
// is missing. Both sentinels also satisfy errors.Is(_, ErrNotFound) so legacy
// callers that only branch on ErrNotFound keep working — new callers should
// match the specific sentinel first to surface the correct field in
// BAD_USER_INPUT responses.
func (r *roleRepo) AssignToUser(ctx context.Context, userID, roleID string) error {
	// Validate user exists.
	var userCount int64
	if err := r.db.WithContext(ctx).
		Table("users").
		Where("id = ?", userID).
		Count(&userCount).Error; err != nil {
		return eris.Wrap(err, "repository: assign role: check user")
	}
	if userCount == 0 {
		return ErrUserNotFound
	}

	// Validate role exists.
	var roleCount int64
	if err := r.db.WithContext(ctx).
		Table("roles").
		Where("id = ?", roleID).
		Count(&roleCount).Error; err != nil {
		return eris.Wrap(err, "repository: assign role: check role")
	}
	if roleCount == 0 {
		return ErrRoleNotFound
	}

	row := gormUserRole{UserID: userID, RoleID: roleID}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&row).Error; err != nil {
		return eris.Wrap(err, "repository: assign role to user")
	}
	return nil
}

// RevokeFromUser deletes the (user_id, role_id) row. Idempotent: no error when
// the row does not exist.
func (r *roleRepo) RevokeFromUser(ctx context.Context, userID, roleID string) error {
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
