package services

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/pkg/logger"
	"context"
	"fmt"

	"github.com/m-mizutani/goerr"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type roleService struct {
	db           *gorm.DB
	defaultLimit int
}

type RoleService interface {
	GetRoleByUserID(ctx context.Context, userID int64) (*model.Role, error)
	GetRoleByID(ctx context.Context, id int64) (*model.Role, error)
	CreateRole(ctx context.Context, input model.NewRole) (*model.Role, error)
	UpdateRole(ctx context.Context, id int64, input model.NewRole) (*model.Role, error)
	DeleteRole(ctx context.Context, id int64) (*bool, error)
	AssignRoleToUser(ctx context.Context, userID int64, roleID int64) (*model.User, error)
	RemoveRoleFromUser(ctx context.Context, userID int64, roleID int64) (*model.User, error)
	Roles(ctx context.Context) ([]*model.Role, error)
	GetRolesByIDs(ctx context.Context, ids []int64) ([]*model.Role, error)
}

func NewRoleService(db *gorm.DB, defaultLimit int) RoleService {
	return &roleService{db: db, defaultLimit: defaultLimit}
}

func convertToRole(role repository.Role) *model.Role {
	return &model.Role{
		ID:      role.ID,
		Name:    role.Name,
		Created: role.Created,
		Updated: role.Updated,
	}
}

func convertToGormRole(input model.NewRole) repository.Role {
	return repository.Role{
		Name: input.Name,
	}
}

func (s *roleService) GetRoleByUserID(ctx context.Context, userID int64) (*model.Role, error) {
	var user repository.User
	if err := s.db.WithContext(ctx).Preload("Roles").First(&user, userID).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Errorf("failed to get user by ID : %d", userID))
	}
	if len(user.Roles) == 0 {
		err := fmt.Errorf("user has no role")
		return nil, goerr.Wrap(err, "no roles found for user")
	}
	role := user.Roles[0] // Assuming a user has only one role
	return convertToRole(role), nil
}

func (s *roleService) GetRoleByID(ctx context.Context, id int64) (*model.Role, error) {
	var role repository.Role
	if err := s.db.WithContext(ctx).First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, goerr.Wrap(err, fmt.Errorf("role not found : %d", id))
		}
		return nil, goerr.Wrap(err, "failed to get role by ID")
	}
	return convertToRole(role), nil
}

func (s *roleService) CreateRole(ctx context.Context, input model.NewRole) (*model.Role, error) {
	gormRole := convertToGormRole(input)
	result := s.db.WithContext(ctx).Create(&gormRole)
	if result.Error != nil {
		return nil, goerr.Wrap(result.Error, "failed to create role")
	}
	return convertToRole(gormRole), nil
}

func (s *roleService) UpdateRole(ctx context.Context, id int64, input model.NewRole) (*model.Role, error) {
	var role repository.Role
	if err := s.db.WithContext(ctx).First(&role, id).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Errorf("role not found for update : %d", id))
	}
	role.Name = input.Name
	if err := s.db.WithContext(ctx).Save(&role).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to update role")
	}
	return convertToRole(role), nil
}

func (s *roleService) DeleteRole(ctx context.Context, id int64) (*bool, error) {
	success := false
	result := s.db.WithContext(ctx).Delete(&repository.Role{}, id)
	if result.Error != nil {
		return &success, goerr.Wrap(result.Error, "failed to delete role")
	}
	if result.RowsAffected == 0 {
		return &success, goerr.Wrap(fmt.Errorf("role not found for deletion"))
	}
	success = true
	return &success, nil
}

func (s *roleService) AssignRoleToUser(ctx context.Context, userID int64, roleID int64) (*model.User, error) {
	var user repository.User
	var role repository.Role
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Errorf("user not found : %d", userID))
	}
	if err := s.db.WithContext(ctx).First(&role, roleID).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Errorf("role not found : %d", roleID))
	}
	if err := s.db.WithContext(ctx).Model(&user).Association("Roles").Append(&role); err != nil {
		return nil, goerr.Wrap(err, "failed to assign role to user")
	}
	return &model.User{
		ID:      user.ID,
		Name:    user.Name,
		Created: user.Created,
		Updated: user.Updated,
	}, nil
}

func (s *roleService) RemoveRoleFromUser(ctx context.Context, userID int64, roleID int64) (*model.User, error) {
	var user repository.User

	// Fetch the user along with the specified role in one query
	if err := s.db.WithContext(ctx).Preload("Roles", "id = ?", roleID).First(&user, userID).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Errorf("failed to find user or role for removal : userID=%d, roleID=%d", userID, roleID))
	}

	// Check if the role exists in the user's roles
	if len(user.Roles) == 0 {
		err := fmt.Errorf("role not found for user")
		return nil, goerr.Wrap(err, "role not found in user's roles")
	}

	// Remove the role from the user's roles
	if err := s.db.WithContext(ctx).Model(&user).Association("Roles").Delete(&user.Roles[0]); err != nil {
		return nil, goerr.Wrap(err, "failed to remove role from user")
	}

	return &model.User{
		ID:      user.ID,
		Name:    user.Name,
		Created: user.Created,
		Updated: user.Updated,
	}, nil
}

func (s *roleService) Roles(ctx context.Context) ([]*model.Role, error) {
	var roles []repository.Role
	if err := s.db.WithContext(ctx).Find(&roles).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to retrieve roles")
	}
	var gqlRoles []*model.Role
	for _, role := range roles {
		gqlRoles = append(gqlRoles, convertToRole(role))
	}
	return gqlRoles, nil
}

func (s *roleService) PaginatedRolesByUser(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.RoleConnection, error) {
	var roles []repository.Role
	query := s.db.WithContext(ctx).Model(&repository.User{ID: userID})

	if after != nil {
		query = query.Where("id > ?", *after)
	}
	if before != nil {
		query = query.Where("id < ?", *before)
	}
	if first != nil {
		query = query.Order("id asc").Limit(*first)
	} else if last != nil {
		query = query.Order("id desc").Limit(*last)
	} else {
		query = query.Order("id asc").Limit(s.defaultLimit)
	}

	if err := query.Association("Roles").Find(&roles).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Error retrieving paginated roles by user", "error", err)
		return nil, fmt.Errorf("error %+v", err())
	}

	var edges []*model.RoleEdge
	var nodes []*model.Role
	for _, role := range roles {
		node := convertToRole(role)
		edges = append(edges, &model.RoleEdge{
			Cursor: role.ID,
			Node:   node,
		})
		nodes = append(nodes, node)
	}

	pageInfo := &model.PageInfo{}
	if len(roles) > 0 {
		pageInfo.HasNextPage = len(roles) == s.defaultLimit
		pageInfo.HasPreviousPage = len(roles) == s.defaultLimit
		if len(edges) > 0 {
			pageInfo.StartCursor = &edges[0].Cursor
			pageInfo.EndCursor = &edges[len(edges)-1].Cursor
		}
	}

	return &model.RoleConnection{
		Edges:      edges,
		Nodes:      nodes,
		PageInfo:   pageInfo,
		TotalCount: len(roles),
	}, nil
}

func (s *roleService) GetRolesByIDs(ctx context.Context, ids []int64) ([]*model.Role, error) {
	var roles []*repository.Role
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&roles).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to retrieve roles by IDs")
	}

	var gqlRoles []*model.Role
	for _, role := range roles {
		gqlRoles = append(gqlRoles, convertToRole(*role))
	}

	return gqlRoles, nil
}
