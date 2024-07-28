package graph

import (
	"backend/graph/model"
	"backend/pkg/logger"
	"context"
	"fmt"
)

// CreateCard is the resolver for the createCard field.
func (r *mutationResolver) CreateCard(ctx context.Context, input model.NewCard) (*model.Card, error) {
	if err := r.VW.Validator().Struct(input); err != nil {
		logger.Logger.ErrorContext(ctx, "Validation error", err)
		return nil, fmt.Errorf("invalid input: %+v", err)
	}
	return r.Srv.CreateCard(ctx, input)
}

// UpdateCard is the resolver for the updateCard field.
func (r *mutationResolver) UpdateCard(ctx context.Context, id int64, input model.NewCard) (*model.Card, error) {
	if err := r.VW.Validator().Struct(input); err != nil {
		logger.Logger.ErrorContext(ctx, "Validation error", err)
		return nil, fmt.Errorf("invalid input: %+v", err)
	}
	return r.Srv.UpdateCard(ctx, id, input)
}

// DeleteCard is the resolver for the deleteCard field.
func (r *mutationResolver) DeleteCard(ctx context.Context, id int64) (*bool, error) {
	return r.Srv.DeleteCard(ctx, id)
}

// CreateCardGroup is the resolver for the createCardGroup field.
func (r *mutationResolver) CreateCardGroup(ctx context.Context, input model.NewCardGroup) (*model.CardGroup, error) {
	if err := r.VW.Validator().Struct(input); err != nil {
		logger.Logger.ErrorContext(ctx, "Validation error", err)
		return nil, fmt.Errorf("invalid input: %+v", err)
	}
	return r.Srv.CreateCardGroup(ctx, input)
}

// UpdateCardGroup is the resolver for the updateCardGroup field.
func (r *mutationResolver) UpdateCardGroup(ctx context.Context, id int64, input model.NewCardGroup) (*model.CardGroup, error) {
	if err := r.VW.Validator().Struct(input); err != nil {
		logger.Logger.ErrorContext(ctx, "Validation error", err)
		return nil, fmt.Errorf("invalid input: %+v", err)
	}
	return r.Srv.UpdateCardGroup(ctx, id, input)
}

// DeleteCardGroup is the resolver for the deleteCardGroup field.
func (r *mutationResolver) DeleteCardGroup(ctx context.Context, id int64) (*bool, error) {
	return r.Srv.DeleteCardGroup(ctx, id)
}

// CreateUser is the resolver for the createUser field.
func (r *mutationResolver) CreateUser(ctx context.Context, input model.NewUser) (*model.User, error) {
	if err := r.VW.Validator().Struct(input); err != nil {
		logger.Logger.ErrorContext(ctx, "Validation error", err)
		return nil, fmt.Errorf("invalid input: %+v", err)
	}
	return r.Srv.CreateUser(ctx, input)
}

// UpdateUser is the resolver for the updateUser field.
func (r *mutationResolver) UpdateUser(ctx context.Context, id int64, input model.NewUser) (*model.User, error) {
	if err := r.VW.Validator().Struct(input); err != nil {
		logger.Logger.ErrorContext(ctx, "Validation error", err)
		return nil, fmt.Errorf("invalid input: %+v", err)
	}
	return r.Srv.UpdateUser(ctx, id, input)
}

// DeleteUser is the resolver for the deleteUser field.
func (r *mutationResolver) DeleteUser(ctx context.Context, id int64) (*bool, error) {
	return r.Srv.DeleteUser(ctx, id)
}

// CreateRole is the resolver for the createRole field.
func (r *mutationResolver) CreateRole(ctx context.Context, input model.NewRole) (*model.Role, error) {
	if err := r.VW.Validator().Struct(input); err != nil {
		logger.Logger.ErrorContext(ctx, "Validation error", err)
		return nil, fmt.Errorf("invalid input: %+v", err)
	}
	return r.Srv.CreateRole(ctx, input)
}

// UpdateRole is the resolver for the updateRole field.
func (r *mutationResolver) UpdateRole(ctx context.Context, id int64, input model.NewRole) (*model.Role, error) {
	if err := r.VW.Validator().Struct(input); err != nil {
		logger.Logger.ErrorContext(ctx, "Validation error", err)
		return nil, fmt.Errorf("invalid input: %+v", err)
	}
	return r.Srv.UpdateRole(ctx, id, input)
}

// DeleteRole is the resolver for the deleteRole field.
func (r *mutationResolver) DeleteRole(ctx context.Context, id int64) (*bool, error) {
	return r.Srv.DeleteRole(ctx, id)
}

// AddUserToCardGroup is the resolver for the addUserToCardGroup field.
func (r *mutationResolver) AddUserToCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error) {
	if userID <= 0 || cardGroupID <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid userID or cardGroupID"))
		return nil, fmt.Errorf("invalid userID or cardGroupID")
	}
	return r.Srv.AddUserToCardGroup(ctx, userID, cardGroupID)
}

// RemoveUserFromCardGroup is the resolver for the removeUserFromCardGroup field.
func (r *mutationResolver) RemoveUserFromCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error) {
	if userID <= 0 || cardGroupID <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid userID or cardGroupID"))
		return nil, fmt.Errorf("invalid userID or cardGroupID")
	}
	return r.Srv.RemoveUserFromCardGroup(ctx, userID, cardGroupID)
}

// AssignRoleToUser is the resolver for the assignRoleToUser field.
func (r *mutationResolver) AssignRoleToUser(ctx context.Context, userID int64, roleID int64) (*model.User, error) {
	if userID <= 0 || roleID <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid userID or roleID"))
		return nil, fmt.Errorf("invalid userID or roleID")
	}
	return r.Srv.AssignRoleToUser(ctx, userID, roleID)
}

// RemoveRoleFromUser is the resolver for the removeRoleFromUser field.
func (r *mutationResolver) RemoveRoleFromUser(ctx context.Context, userID int64, roleID int64) (*model.User, error) {
	if userID <= 0 || roleID <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid userID or roleID"))
		return nil, fmt.Errorf("invalid userID or roleID")
	}
	return r.Srv.RemoveRoleFromUser(ctx, userID, roleID)
}

// Cards is the resolver for the cards field.
func (r *queryResolver) Cards(ctx context.Context, first *int, after *int64, last *int, before *int64) (*model.CardConnection, error) {
	return r.Srv.PaginatedCards(ctx, first, after, last, before)
}

// Card is the resolver for the card field.
func (r *queryResolver) Card(ctx context.Context, id int64) (*model.Card, error) {
	if id <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid id: %d", id))
		return nil, fmt.Errorf("invalid id: %d", id)
	}
	return r.Srv.GetCardByID(ctx, id)
}

// CardGroups is the resolver for the cardGroups field.
func (r *queryResolver) CardGroups(ctx context.Context, first *int, after *int64, last *int, before *int64) (*model.CardGroupConnection, error) {
	return r.Srv.PaginatedCardGroups(ctx, first, after, last, before)
}

// CardGroup is the resolver for the cardGroup field.
func (r *queryResolver) CardGroup(ctx context.Context, id int64) (*model.CardGroup, error) {
	if id <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid id: %d", id))
		return nil, fmt.Errorf("invalid id: %d", id)
	}
	return r.Srv.GetCardGroupByID(ctx, id)
}

// Roles is the resolver for the roles field.
func (r *queryResolver) Roles(ctx context.Context, first *int, after *int64, last *int, before *int64) (*model.RoleConnection, error) {
	return r.Srv.PaginatedRoles(ctx, first, after, last, before)
}

// Role is the resolver for the role field.
func (r *queryResolver) Role(ctx context.Context, id int64) (*model.Role, error) {
	if id <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid id: %d", id))
		return nil, fmt.Errorf("invalid id: %d", id)
	}
	return r.Srv.GetRoleByID(ctx, id)
}

// Users is the resolver for the users field.
func (r *queryResolver) Users(ctx context.Context, first *int, after *int64, last *int, before *int64) (*model.UserConnection, error) {
	return r.Srv.PaginatedUsers(ctx, first, after, last, before)
}

// User is the resolver for the user field.
func (r *queryResolver) User(ctx context.Context, id int64) (*model.User, error) {
	if id <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid id: %d", id))
		return nil, fmt.Errorf("invalid id: %d", id)
	}
	return r.Srv.GetUserByID(ctx, id)
}

// CardsByCardGroup is the resolver for the cardsByCardGroup field.
func (r *queryResolver) CardsByCardGroup(ctx context.Context, cardGroupID int64, first *int, after *int64, last *int, before *int64) (*model.CardConnection, error) {
	if cardGroupID <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid cardGroupID: %d", cardGroupID))
		return nil, fmt.Errorf("invalid cardGroupID: %d", cardGroupID)
	}
	return r.Srv.PaginatedCardsByCardGroup(ctx, cardGroupID, first, after, last, before)
}

// UserRole is the resolver for the userRole field.
func (r *queryResolver) UserRole(ctx context.Context, userID int64) (*model.Role, error) {
	if userID <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid userID: %d", userID))
		return nil, fmt.Errorf("invalid userID: %d", userID)
	}
	return r.Srv.GetRoleByUserID(ctx, userID)
}

// CardGroupsByUser is the resolver for the cardGroupsByUser field.
func (r *queryResolver) CardGroupsByUser(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.CardGroupConnection, error) {
	if userID <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid userID: %d", userID))
		return nil, fmt.Errorf("invalid userID: %d", userID)
	}
	return r.Srv.PaginatedCardGroupsByUser(ctx, userID, first, after, last, before)
}

// UsersByRole is the resolver for the usersByRole field.
func (r *queryResolver) UsersByRole(ctx context.Context, roleID int64, first *int, after *int64, last *int, before *int64) (*model.UserConnection, error) {
	if roleID <= 0 {
		logger.Logger.ErrorContext(ctx, "Validation error", fmt.Errorf("invalid roleID: %d", roleID))
		return nil, fmt.Errorf("invalid roleID: %d", roleID)
	}
	return r.Srv.PaginatedUsersByRole(ctx, roleID, first, after, last, before)
}

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
