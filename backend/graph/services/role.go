package services

import (
	"backend/graph/db"
	"backend/graph/model"
	"context"
	"fmt"
	"gorm.io/gorm"
)

type roleService struct {
	db           *gorm.DB
	defaultLimit int
}

func convertToRole(role db.Role) *model.Role {
	return &model.Role{
		ID:      role.ID,
		Name:    role.Name,
		Created: role.Created,
		Updated: role.Updated,
	}
}

func convertToGormRole(input model.NewRole) db.Role {
	return db.Role{
		Name: input.Name,
	}
}

func (s *roleService) GetRoleByUserID(ctx context.Context, userID int64) (*model.Role, error) {
	var user db.User
	if err := s.db.WithContext(ctx).Preload("Roles").First(&user, userID).Error; err != nil {
		return nil, err
	}
	if len(user.Roles) == 0 {
		return nil, fmt.Errorf("user has no role")
	}
	role := user.Roles[0] // Assuming a user has only one role
	return convertToRole(role), nil
}

func (s *roleService) GetRoleByID(ctx context.Context, id int64) (*model.Role, error) {
	var role db.Role
	if err := s.db.WithContext(ctx).First(&role, id).Error; err != nil {
		return nil, err
	}
	return convertToRole(role), nil
}

func (s *roleService) CreateRole(ctx context.Context, input model.NewRole) (*model.Role, error) {
	gormRole := convertToGormRole(input)
	result := s.db.WithContext(ctx).Create(&gormRole)
	if result.Error != nil {
		return nil, result.Error
	}
	return convertToRole(gormRole), nil
}

func (s *roleService) UpdateRole(ctx context.Context, id int64, input model.NewRole) (*model.Role, error) {
	var role db.Role
	if err := s.db.WithContext(ctx).First(&role, id).Error; err != nil {
		return nil, err
	}
	role.Name = input.Name
	if err := s.db.WithContext(ctx).Save(&role).Error; err != nil {
		return nil, err
	}
	return convertToRole(role), nil
}

func (s *roleService) DeleteRole(ctx context.Context, id int64) (*bool, error) {
	success := false
	result := s.db.WithContext(ctx).Delete(&db.Role{}, id)
	if result.Error != nil {
		return &success, result.Error
	}
	if result.RowsAffected == 0 {
		return &success, fmt.Errorf("record not found")
	}
	success = true
	return &success, nil
}

func (s *roleService) AssignRoleToUser(ctx context.Context, userID int64, roleID int64) (*model.User, error) {
	var user db.User
	var role db.Role
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).First(&role, roleID).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&user).Association("Roles").Append(&role); err != nil {
		return nil, err
	}
	return &model.User{
		ID:      user.ID,
		Name:    user.Name,
		Created: user.Created,
		Updated: user.Updated,
	}, nil
}

func (s *roleService) RemoveRoleFromUser(ctx context.Context, userID int64, roleID int64) (*model.User, error) {
	var user db.User
	var role db.Role
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).First(&role, roleID).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&user).Association("Roles").Delete(&role); err != nil {
		return nil, err
	}
	return &model.User{
		ID:      user.ID,
		Name:    user.Name,
		Created: user.Created,
		Updated: user.Updated,
	}, nil
}

func (s *roleService) Roles(ctx context.Context) ([]*model.Role, error) {
	var roles []db.Role
	if err := s.db.WithContext(ctx).Find(&roles).Error; err != nil {
		return nil, err
	}
	var gqlRoles []*model.Role
	for _, role := range roles {
		gqlRoles = append(gqlRoles, convertToRole(role))
	}
	return gqlRoles, nil
}

func (s *roleService) PaginatedRoles(ctx context.Context, first *int, after *int64, last *int, before *int64) (*model.RoleConnection, error) {
	var roles []db.Role
	query := s.db.WithContext(ctx)

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

	if err := query.Find(&roles).Error; err != nil {
		return nil, err
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

func (s *roleService) PaginatedRolesByUser(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.RoleConnection, error) {
	var roles []db.Role
	query := s.db.WithContext(ctx).Model(&db.User{ID: userID})

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
	var roles []*model.Role
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}
