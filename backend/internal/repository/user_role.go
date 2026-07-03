package repository

import (
	"context"
	"errors"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"backend/internal/domain"
)

type gormUserRole struct {
	UserID string `gorm:"column:user_id;primaryKey;type:uuid"`
	RoleID string `gorm:"column:role_id;primaryKey;type:uuid"`
}

func (gormUserRole) TableName() string { return "user_roles" }

// gormUserRoleJoinRow is the projected shape of the join between user_roles
// and roles used by ListByUserIDs.
type gormUserRoleJoinRow struct {
	UserID string `gorm:"column:user_id"`
	ID     string `gorm:"column:id"`
	Name   string `gorm:"column:name"`
}

// UserRoleRepository queries the many-to-many membership between users and roles.
type UserRoleRepository interface {
	// HasRole reports whether userID holds the named role.
	// Returns (false, nil) when the user has no rows or the role name does not exist.
	// Only DB errors return a non-nil error. Role lookup is by name (case-sensitive).
	HasRole(ctx context.Context, userID string, roleName domain.RoleName) (bool, error)

	// AssignToUser inserts a (user_id, role_id) row. Idempotent: if the row
	// already exists, returns nil without error. Returns ErrUserNotFound when
	// the user is missing and ErrRoleNotFound when the role is missing; both
	// also satisfy errors.Is(_, ErrNotFound) for backward-compatible matching.
	AssignToUser(ctx context.Context, userID, roleID string) error

	// SetUserRolesTx replaces the user's role set inside the supplied
	// transaction. roleIDs is the final declarative state; an empty slice
	// removes every role assignment for the user.
	SetUserRolesTx(ctx context.Context, tx *gorm.DB, userID string, roleIDs []string) error

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

	// CountAdmins returns the number of distinct user_roles rows that
	// reference the role named "admin". Returns 0 (not an error) when the
	// admin role row itself does not exist — callers treat the absence of
	// the role and an empty assignment table as the same operational state.
	CountAdmins(ctx context.Context) (int64, error)
}

type userRoleRepo struct{ db *gorm.DB }

func NewUserRoleRepository(db *gorm.DB) UserRoleRepository {
	return &userRoleRepo{db: db}
}

func (r *userRoleRepo) HasRole(ctx context.Context, userID string, roleName domain.RoleName) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ? AND roles.name = ?", userID, roleName).
		Count(&count).Error
	if err != nil {
		return false, eris.Wrap(err, "repository: check user role")
	}
	return count > 0, nil
}

// requireExists returns ErrUserNotFound / ErrRoleNotFound when no row matches
// the given primary key. The wrap label feeds into the eris chain when the
// underlying COUNT query itself fails. notFound is the sentinel returned for
// the zero-count case.
func (r *userRoleRepo) requireExists(ctx context.Context, table, id, wrap string, notFound error) error {
	return requireExistsOn(ctx, r.db, table, id, wrap, notFound)
}

func requireExistsOn(ctx context.Context, db *gorm.DB, table, id, wrap string, notFound error) error {
	var count int64
	if err := db.WithContext(ctx).
		Table(table).
		Where("id = ?", id).
		Count(&count).Error; err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
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
// is missing.
func (r *userRoleRepo) AssignToUser(ctx context.Context, userID, roleID string) error {
	if err := r.requireExists(ctx, "users", userID, "repository: user role: assign: check user", ErrUserNotFound); err != nil {
		return err
	}
	if err := r.requireExists(ctx, "roles", roleID, "repository: user role: assign: check role", ErrRoleNotFound); err != nil {
		return err
	}

	row := gormUserRole{UserID: userID, RoleID: roleID}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&row).Error; err != nil {
		if classified := classifyFKError(err); classified != nil {
			return classified
		}
		return eris.Wrap(err, "repository: user role: assign to user")
	}
	return nil
}

func (r *userRoleRepo) SetUserRolesTx(ctx context.Context, tx *gorm.DB, userID string, roleIDs []string) error {
	if tx == nil {
		return eris.New("repository: user role: set tx is nil")
	}
	if err := requireExistsOn(ctx, tx, "users", userID, "repository: user role: set: check user", ErrUserNotFound); err != nil {
		return err
	}

	uniqueRoleIDs := make([]string, 0, len(roleIDs))
	seen := make(map[string]bool, len(roleIDs))
	for _, roleID := range roleIDs {
		if seen[roleID] {
			continue
		}
		seen[roleID] = true
		uniqueRoleIDs = append(uniqueRoleIDs, roleID)
	}
	// Validate that every submitted role exists in one batched COUNT rather than
	// N sequential round trips inside the row-locked transaction. The empty-slice
	// short-circuit must stay ahead of the IN query: GORM drops an empty-slice IN
	// clause and scans the whole table (see .claude/rules/go-library-gotchas.md).
	if len(uniqueRoleIDs) > 0 {
		var n int64
		if err := tx.WithContext(ctx).
			Table("roles").
			Where("id IN ?", uniqueRoleIDs).
			Count(&n).Error; err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return eris.Wrap(err, "repository: user role: set: check roles")
		}
		if n != int64(len(uniqueRoleIDs)) {
			return ErrRoleNotFound
		}
	}

	q := tx.WithContext(ctx).Where("user_id = ?", userID)
	if len(uniqueRoleIDs) > 0 {
		q = q.Where("role_id NOT IN ?", uniqueRoleIDs)
	}
	if err := q.Delete(&gormUserRole{}).Error; err != nil {
		return eris.Wrap(err, "repository: user role: set: delete removed roles")
	}

	if len(uniqueRoleIDs) == 0 {
		return nil
	}
	rows := make([]gormUserRole, 0, len(uniqueRoleIDs))
	for _, roleID := range uniqueRoleIDs {
		rows = append(rows, gormUserRole{UserID: userID, RoleID: roleID})
	}
	if err := tx.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&rows).Error; err != nil {
		if classified := classifyFKError(err); classified != nil {
			return classified
		}
		return eris.Wrap(err, "repository: user role: set: insert roles")
	}
	return nil
}

func (r *userRoleRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Role, error) {
	var rows []gormRole
	if err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Order("roles.name ASC").
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: user role: list by user")
	}
	out := make([]*domain.Role, len(rows))
	for i := range rows {
		out[i] = roleToDomain(rows[i])
	}
	return out, nil
}

func (r *userRoleRepo) ListByUserIDs(ctx context.Context, userIDs []string) (map[string][]*domain.Role, error) {
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
		return nil, eris.Wrap(err, "repository: user role: list by user ids")
	}

	out := make(map[string][]*domain.Role, len(rows))
	for i := range rows {
		out[rows[i].UserID] = append(out[rows[i].UserID], &domain.Role{
			ID:   rows[i].ID,
			Name: domain.RoleName(rows[i].Name),
		})
	}
	return out, nil
}

func (r *userRoleRepo) CountAdmins(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("roles.name = ?", domain.AdminRoleName).
		Count(&n).Error
	if err != nil {
		return 0, eris.Wrap(err, "repository: user role: count admins")
	}
	return n, nil
}
