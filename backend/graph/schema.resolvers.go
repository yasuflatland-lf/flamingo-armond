package graph

import (
	"backend/graph/model"
	"context"
)

// CreateCard is the resolver for the createCard field.
func (r *mutationResolver) CreateCard(ctx context.Context, input model.NewCard) (*model.Card, error) {
	return r.Srv.CreateCard(ctx, input)
}

// UpdateCard is the resolver for the updateCard field.
func (r *mutationResolver) UpdateCard(ctx context.Context, id int64, input model.NewCard) (*model.Card, error) {
	return r.Srv.UpdateCard(ctx, id, input)
}

// DeleteCard is the resolver for the deleteCard field.
func (r *mutationResolver) DeleteCard(ctx context.Context, id int64) (bool, error) {
	return r.Srv.DeleteCard(ctx, id)
}

// CreateCardGroup is the resolver for the createCardGroup field.
func (r *mutationResolver) CreateCardGroup(ctx context.Context, input model.NewCardGroup) (*model.CardGroup, error) {
	return r.Srv.CreateCardGroup(ctx, input)
}

// UpdateCardGroup is the resolver for the updateCardGroup field.
func (r *mutationResolver) UpdateCardGroup(ctx context.Context, id int64, input model.NewCardGroup) (*model.CardGroup, error) {
	return r.Srv.UpdateCardGroup(ctx, id, input)
}

// DeleteCardGroup is the resolver for the deleteCardGroup field.
func (r *mutationResolver) DeleteCardGroup(ctx context.Context, id int64) (bool, error) {
	return r.Srv.DeleteCardGroup(ctx, id)
}

// CreateUser is the resolver for the createUser field.
func (r *mutationResolver) CreateUser(ctx context.Context, input model.NewUser) (*model.User, error) {
	return r.Srv.CreateUser(ctx, input)
}

// UpdateUser is the resolver for the updateUser field.
func (r *mutationResolver) UpdateUser(ctx context.Context, id int64, input model.NewUser) (*model.User, error) {
	return r.Srv.UpdateUser(ctx, id, input)
}

// DeleteUser is the resolver for the deleteUser field.
func (r *mutationResolver) DeleteUser(ctx context.Context, id int64) (bool, error) {
	return r.Srv.DeleteUser(ctx, id)
}

// CreateRole is the resolver for the createRole field.
func (r *mutationResolver) CreateRole(ctx context.Context, input model.NewRole) (*model.Role, error) {
	return r.Srv.CreateRole(ctx, input)
}

// UpdateRole is the resolver for the updateRole field.
func (r *mutationResolver) UpdateRole(ctx context.Context, id int64, input model.NewRole) (*model.Role, error) {
	return r.Srv.UpdateRole(ctx, id, input)
}

// DeleteRole is the resolver for the deleteRole field.
func (r *mutationResolver) DeleteRole(ctx context.Context, id int64) (bool, error) {
	return r.Srv.DeleteRole(ctx, id)
}

// AddUserToCardGroup is the resolver for the addUserToCardGroup field.
func (r *mutationResolver) AddUserToCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error) {
	return r.Srv.AddUserToCardGroup(ctx, userID, cardGroupID)
}

// RemoveUserFromCardGroup is the resolver for the removeUserFromCardGroup field.
func (r *mutationResolver) RemoveUserFromCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error) {
	return r.Srv.RemoveUserFromCardGroup(ctx, userID, cardGroupID)
}

// AssignRoleToUser is the resolver for the assignRoleToUser field.
func (r *mutationResolver) AssignRoleToUser(ctx context.Context, userID int64, roleID int64) (*model.User, error) {
	return r.Srv.AssignRoleToUser(ctx, userID, roleID)
}

// RemoveRoleFromUser is the resolver for the removeRoleFromUser field.
func (r *mutationResolver) RemoveRoleFromUser(ctx context.Context, userID int64, roleID int64) (*model.User, error) {
	return r.Srv.RemoveRoleFromUser(ctx, userID, roleID)
}

// Cards is the resolver for the cards field.
func (r *queryResolver) Cards(ctx context.Context) ([]*model.Card, error) {
	return r.Srv.Cards(ctx)
}

// Card is the resolver for the card field.
func (r *queryResolver) Card(ctx context.Context, id int64) (*model.Card, error) {
	return r.Srv.GetCardByID(ctx, id)
}

// CardGroups is the resolver for the cardGroups field.
func (r *queryResolver) CardGroups(ctx context.Context) ([]*model.CardGroup, error) {
	return r.Srv.CardGroups(ctx)
}

// CardGroup is the resolver for the cardGroup field.
func (r *queryResolver) CardGroup(ctx context.Context, id int64) (*model.CardGroup, error) {
	return r.Srv.GetCardGroupByID(ctx, id)
}

// Roles is the resolver for the roles field.
func (r *queryResolver) Roles(ctx context.Context) ([]*model.Role, error) {
	return r.Srv.Roles(ctx)
}

// Role is the resolver for the role field.
func (r *queryResolver) Role(ctx context.Context, id int64) (*model.Role, error) {
	return r.Srv.GetRoleByID(ctx, id)
}

// Users is the resolver for the users field.
func (r *queryResolver) Users(ctx context.Context) ([]*model.User, error) {
	return r.Srv.Users(ctx)
}

// User is the resolver for the user field.
func (r *queryResolver) User(ctx context.Context, id int64) (*model.User, error) {
	return r.Srv.GetUserByID(ctx, id)
}

// CardsByCardGroup is the resolver for the cardsByCardGroup field.
func (r *queryResolver) CardsByCardGroup(ctx context.Context, cardGroupID int64) ([]*model.Card, error) {
	return r.Srv.CardsByCardGroup(ctx, cardGroupID)
}

// UserRole is the resolver for the userRole field.
func (r *queryResolver) UserRole(ctx context.Context, userID int64) (*model.Role, error) {
	return r.Srv.GetRoleByUserID(ctx, userID)
}

// CardGroupsByUser is the resolver for the cardGroupsByUser field.
func (r *queryResolver) CardGroupsByUser(ctx context.Context, userID int64) ([]*model.CardGroup, error) {
	return r.Srv.GetCardGroupsByUser(ctx, userID)
}

// UsersByRole is the resolver for the usersByRole field.
func (r *queryResolver) UsersByRole(ctx context.Context, roleID int64) ([]*model.User, error) {
	return r.Srv.GetUsersByRole(ctx, roleID)
}

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
