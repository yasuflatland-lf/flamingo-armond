package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.49

import (
	"backend/graph/model"
	"context"
	"fmt"

	"github.com/m-mizutani/goerr"
)

// CardGroup is the resolver for the cardGroup field in Card.
func (r *cardResolver) CardGroup(ctx context.Context, obj *model.Card) (*model.CardGroup, error) {
	thunk := r.Loaders.CardGroupLoader.Load(ctx, obj.CardGroup.ID)
	cardGroup, err := thunk()
	if err != nil {
		return nil, goerr.Wrap(err, "fetch by CardGroup")
	}
	return cardGroup, nil
}

// Cards is the resolver for the cards field in CardGroup.
func (r *cardGroupResolver) Cards(ctx context.Context, obj *model.CardGroup, first *int, after *int64, last *int, before *int64) (*model.CardConnection, error) {
	var cardIDs []int64
	for _, edge := range obj.Cards.Edges {
		cardIDs = append(cardIDs, edge.Node.ID)
	}

	// Load the cards using dataloader
	thunks := r.Loaders.CardLoader.LoadMany(ctx, cardIDs)
	cards, err := thunks()
	if err != nil {
		return nil, goerr.Wrap(fmt.Errorf("fetch by Cards: %+v", err))
	}

	// Implement pagination logic
	start := 0
	end := len(cards)
	if after != nil {
		start = int(*after) + 1
	}
	if before != nil {
		end = int(*before)
	}
	if first != nil {
		end = start + *first
		if end > len(cards) {
			end = len(cards)
		}
	}
	if last != nil {
		start = end - *last
		if start < 0 {
			start = 0
		}
	}

	paginatedCards := cards[start:end]

	// Prepare edges
	edges := make([]*model.CardEdge, len(paginatedCards))
	for i, card := range paginatedCards {
		edges[i] = &model.CardEdge{
			Node:   card,
			Cursor: card.ID,
		}
	}

	// Prepare PageInfo
	pageInfo := &model.PageInfo{
		StartCursor:     &paginatedCards[0].ID,
		EndCursor:       &paginatedCards[len(paginatedCards)-1].ID,
		HasNextPage:     end < len(cards),
		HasPreviousPage: start > 0,
	}

	return &model.CardConnection{
		Edges:    edges,
		PageInfo: pageInfo,
	}, nil
}

// Users is the resolver for the users field in CardGroup.
func (r *cardGroupResolver) Users(ctx context.Context, obj *model.CardGroup, first *int, after *int64, last *int, before *int64) (*model.UserConnection, error) {
	var userIDs []int64
	for _, edge := range obj.Users.Edges {
		userIDs = append(userIDs, edge.Node.ID)
	}

	// Load the users using dataloader
	thunks := r.Loaders.UserLoader.LoadMany(ctx, userIDs)
	users, err := thunks()
	if err != nil {
		return nil, goerr.Wrap(fmt.Errorf("fetch by Users: %+v", err))
	}

	// Implement pagination logic
	start := 0
	end := len(users)
	if after != nil {
		start = int(*after) + 1
	}
	if before != nil {
		end = int(*before)
	}
	if first != nil {
		end = start + *first
		if end > len(users) {
			end = len(users)
		}
	}
	if last != nil {
		start = end - *last
		if start < 0 {
			start = 0
		}
	}

	paginatedUsers := users[start:end]

	// Prepare edges
	edges := make([]*model.UserEdge, len(paginatedUsers))
	for i, user := range paginatedUsers {
		edges[i] = &model.UserEdge{
			Node:   user,
			Cursor: user.ID,
		}
	}

	// Prepare PageInfo
	pageInfo := &model.PageInfo{
		StartCursor:     &paginatedUsers[0].ID,
		EndCursor:       &paginatedUsers[len(paginatedUsers)-1].ID,
		HasNextPage:     end < len(users),
		HasPreviousPage: start > 0,
	}

	return &model.UserConnection{
		Edges:    edges,
		PageInfo: pageInfo,
	}, nil
}

// CreateCard is the resolver for the createCard field.
func (r *mutationResolver) CreateCard(ctx context.Context, input model.NewCard) (*model.Card, error) {
	if err := r.VW.ValidateStruct(input); err != nil {
		return nil, goerr.Wrap(err, "invalid input CreateCard")
	}
	return r.Srv.CreateCard(ctx, input)
}

// UpdateCard is the resolver for the updateCard field.
func (r *mutationResolver) UpdateCard(ctx context.Context, id int64, input model.NewCard) (*model.Card, error) {
	if err := r.VW.ValidateStruct(input); err != nil {
		return nil, goerr.Wrap(err, "invalid input UpdateCard")
	}
	return r.Srv.UpdateCard(ctx, id, input)
}

// DeleteCard is the resolver for the deleteCard field.
func (r *mutationResolver) DeleteCard(ctx context.Context, id int64) (*bool, error) {
	return r.Srv.DeleteCard(ctx, id)
}

// CreateCardGroup is the resolver for the createCardGroup field.
func (r *mutationResolver) CreateCardGroup(ctx context.Context, input model.NewCardGroup) (*model.CardGroup, error) {
	if err := r.VW.ValidateStruct(input); err != nil {
		return nil, goerr.Wrap(err, "invalid input CreateCardGroup")
	}
	return r.Srv.CreateCardGroup(ctx, input)
}

// UpdateCardGroup is the resolver for the updateCardGroup field.
func (r *mutationResolver) UpdateCardGroup(ctx context.Context, id int64, input model.NewCardGroup) (*model.CardGroup, error) {
	if err := r.VW.ValidateStruct(input); err != nil {
		return nil, goerr.Wrap(err, "invalid input UpdateCardGroup")
	}
	return r.Srv.UpdateCardGroup(ctx, id, input)
}

// DeleteCardGroup is the resolver for the deleteCardGroup field.
func (r *mutationResolver) DeleteCardGroup(ctx context.Context, id int64) (*bool, error) {
	return r.Srv.DeleteCardGroup(ctx, id)
}

// CreateUser is the resolver for the createUser field.
func (r *mutationResolver) CreateUser(ctx context.Context, input model.NewUser) (*model.User, error) {
	if err := r.VW.ValidateStruct(input); err != nil {
		return nil, goerr.Wrap(err, "invalid input CreateUser")
	}
	return r.Srv.CreateUser(ctx, input)
}

// UpdateUser is the resolver for the updateUser field.
func (r *mutationResolver) UpdateUser(ctx context.Context, id int64, input model.NewUser) (*model.User, error) {
	if err := r.VW.ValidateStruct(input); err != nil {
		return nil, goerr.Wrap(err, "invalid input UpdateUser")
	}
	return r.Srv.UpdateUser(ctx, id, input)
}

// DeleteUser is the resolver for the deleteUser field.
func (r *mutationResolver) DeleteUser(ctx context.Context, id int64) (*bool, error) {
	return r.Srv.DeleteUser(ctx, id)
}

// CreateRole is the resolver for the createRole field.
func (r *mutationResolver) CreateRole(ctx context.Context, input model.NewRole) (*model.Role, error) {
	if err := r.VW.ValidateStruct(input); err != nil {
		return nil, goerr.Wrap(err, "invalid input CreateRole")
	}
	return r.Srv.CreateRole(ctx, input)
}

// UpdateRole is the resolver for the updateRole field.
func (r *mutationResolver) UpdateRole(ctx context.Context, id int64, input model.NewRole) (*model.Role, error) {
	if err := r.VW.ValidateStruct(input); err != nil {
		return nil, goerr.Wrap(err, "invalid input UpdateRole")
	}
	return r.Srv.UpdateRole(ctx, id, input)
}

// DeleteRole is the resolver for the deleteRole field.
func (r *mutationResolver) DeleteRole(ctx context.Context, id int64) (*bool, error) {
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

// CreateSwipeRecord is the resolver for the createSwipeRecord field.
func (r *mutationResolver) CreateSwipeRecord(ctx context.Context, input model.NewSwipeRecord) (*model.SwipeRecord, error) {
	if err := r.VW.ValidateStruct(input); err != nil {
		return nil, goerr.Wrap(err, "invalid input CreateSwipeRecord")
	}
	return r.Srv.CreateSwipeRecord(ctx, input)
}

// UpdateSwipeRecord is the resolver for the updateSwipeRecord field.
func (r *mutationResolver) UpdateSwipeRecord(ctx context.Context, id int64, input model.NewSwipeRecord) (*model.SwipeRecord, error) {
	if err := r.VW.ValidateStruct(input); err != nil {
		return nil, goerr.Wrap(err, "invalid input UpdateSwipeRecord")
	}
	return r.Srv.UpdateSwipeRecord(ctx, id, input)
}

// DeleteSwipeRecord is the resolver for the deleteSwipeRecord field.
func (r *mutationResolver) DeleteSwipeRecord(ctx context.Context, id int64) (*bool, error) {
	return r.Srv.DeleteSwipeRecord(ctx, id)
}

// UpsertDictionary is the resolver for the upsertDictionary field.
func (r *mutationResolver) UpsertDictionary(ctx context.Context, input model.UpsertDictionary) (*model.CardConnection, error) {
	createdCards, err := r.U.UpsertCards(ctx, input.Dictionary,
		input.CardgroupID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to upsert cards")
	}

	cardConnection := &model.CardConnection{
		Edges:    make([]*model.CardEdge, len(createdCards)),
		Nodes:    make([]*model.Card, len(createdCards)),
		PageInfo: &model.PageInfo{},
	}

	for i, card := range createdCards {
		edge := &model.CardEdge{
			Node:   card,
			Cursor: card.ID,
		}
		cardConnection.Edges[i] = edge
		cardConnection.Nodes[i] = card
	}

	if len(createdCards) > 0 {
		cardConnection.PageInfo.StartCursor = &createdCards[0].ID
		cardConnection.PageInfo.EndCursor = &createdCards[len(createdCards)-1].ID
		cardConnection.PageInfo.HasPreviousPage = false // Assuming no pagination for this case
		cardConnection.PageInfo.HasNextPage = false     // Assuming no pagination for this case
	}

	return cardConnection, nil
}

// HandleSwipe is the resolver for the handleSwipe field.
func (r *mutationResolver) HandleSwipe(ctx context.Context, input model.NewSwipeRecord) ([]*model.Card, error) {
	panic(fmt.Errorf("not implemented: HandleSwipe - handleSwipe"))
}

// Card is the resolver for the card field.
func (r *queryResolver) Card(ctx context.Context, id int64) (*model.Card, error) {
	// Use DataLoader to fetch the Card by ID
	thunk := r.Loaders.CardLoader.Load(ctx, id)
	card, err := thunk()
	if err != nil {
		return nil, goerr.Wrap(err, "invalid input Card")
	}

	return card, nil
}

// CardGroup is the resolver for the cardGroup field.
func (r *queryResolver) CardGroup(ctx context.Context, id int64) (*model.CardGroup, error) {
	thunk := r.Loaders.CardGroupLoader.Load(ctx, id)
	cardGroup, err := thunk()
	if err != nil {
		return nil, goerr.Wrap(err, "invalid input CardGroup")
	}
	return cardGroup, nil
}

// Role is the resolver for the role field.
func (r *queryResolver) Role(ctx context.Context, id int64) (*model.Role, error) {
	thunk := r.Loaders.RoleLoader.Load(ctx, id)
	role, err := thunk()
	if err != nil {
		return nil, goerr.Wrap(err, "invalid input Role")
	}

	return role, nil
}

// User is the resolver for the user field.
func (r *queryResolver) User(ctx context.Context, id int64) (*model.User, error) {
	thunk := r.Loaders.UserLoader.Load(ctx, id)
	user, err := thunk()
	if err != nil {
		return nil, goerr.Wrap(err, "invalid input User")
	}
	return user, nil
}

// SwipeRecord is the resolver for the swipeRecord field.
func (r *queryResolver) SwipeRecord(ctx context.Context, id int64) (*model.SwipeRecord, error) {
	// Use DataLoader to fetch the SwipeRecord by ID
	thunk := r.Loaders.SwipeRecordLoader.Load(ctx, id)
	swipeRecord, err := thunk()
	if err != nil {
		return nil, goerr.Wrap(err, "fetch by SwipeRecord")
	}

	return swipeRecord, nil
}

// CardsByCardGroup is the resolver for the cardsByCardGroup field.
func (r *queryResolver) CardsByCardGroup(ctx context.Context, cardGroupID int64, first *int, after *int64, last *int, before *int64) (*model.CardConnection, error) {
	return r.Srv.PaginatedCardsByCardGroup(ctx, cardGroupID, first, after, last, before)
}

// UserRole is the resolver for the userRole field.
func (r *queryResolver) UserRole(ctx context.Context, userID int64) (*model.Role, error) {
	return r.Srv.GetRoleByUserID(ctx, userID)
}

// CardGroupsByUser is the resolver for the cardGroupsByUser field.
func (r *queryResolver) CardGroupsByUser(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.CardGroupConnection, error) {
	return r.Srv.PaginatedCardGroupsByUser(ctx, userID, first, after, last, before)
}

// UsersByRole is the resolver for the usersByRole field.
func (r *queryResolver) UsersByRole(ctx context.Context, roleID int64, first *int, after *int64, last *int, before *int64) (*model.UserConnection, error) {
	return r.Srv.PaginatedUsersByRole(ctx, roleID, first, after, last, before)
}

// SwipeRecords is the resolver for the swipeRecords field.
func (r *queryResolver) SwipeRecords(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.SwipeRecordConnection, error) {
	return r.Srv.PaginatedSwipeRecordsByUser(ctx, userID, first, after, last, before)
}

// Users is the resolver for the users field in Role.
func (r *roleResolver) Users(ctx context.Context, obj *model.Role, first *int, after *int64, last *int, before *int64) (*model.UserConnection, error) {
	var userIDs []int64
	for _, edge := range obj.Users.Edges {
		userIDs = append(userIDs, edge.Node.ID)
	}

	// Load the users using dataloader
	thunks := r.Loaders.UserLoader.LoadMany(ctx, userIDs)
	users, err := thunks()
	if err != nil {
		return nil, goerr.Wrap(fmt.Errorf("fetch by dataloader: %+v", err))
	}

	// Implement pagination logic
	start := 0
	end := len(users)
	if after != nil {
		start = int(*after) + 1
	}
	if before != nil {
		end = int(*before)
	}
	if first != nil {
		end = start + *first
		if end > len(users) {
			end = len(users)
		}
	}
	if last != nil {
		start = end - *last
		if start < 0 {
			start = 0
		}
	}

	paginatedUsers := users[start:end]

	// Prepare edges
	edges := make([]*model.UserEdge, len(paginatedUsers))
	for i, user := range paginatedUsers {
		edges[i] = &model.UserEdge{
			Node:   user,
			Cursor: user.ID,
		}
	}

	// Prepare PageInfo
	pageInfo := &model.PageInfo{
		StartCursor:     &paginatedUsers[0].ID,
		EndCursor:       &paginatedUsers[len(paginatedUsers)-1].ID,
		HasNextPage:     end < len(users),
		HasPreviousPage: start > 0,
	}

	return &model.UserConnection{
		Edges:    edges,
		PageInfo: pageInfo,
	}, nil
}

// CardGroups is the resolver for the cardGroups field in User.
func (r *userResolver) CardGroups(ctx context.Context, obj *model.User, first *int, after *int64, last *int, before *int64) (*model.CardGroupConnection, error) {
	var cardGroupIDs []int64
	for _, edge := range obj.CardGroups.Edges {
		cardGroupIDs = append(cardGroupIDs, edge.Node.ID)
	}
	thunks := r.Loaders.CardGroupLoader.LoadMany(ctx, cardGroupIDs)
	cardGroups, err := thunks()
	if err != nil {
		return nil, goerr.Wrap(fmt.Errorf("fetch by dataloader: %+v", err))
	}

	for i, cardGroup := range cardGroups {
		cardGroups[i] = cardGroup
	}
	return &model.CardGroupConnection{Nodes: cardGroups}, nil
}

// Roles is the resolver for the roles field in User.
func (r *userResolver) Roles(ctx context.Context, obj *model.User, first *int, after *int64, last *int, before *int64) (*model.RoleConnection, error) {
	var roleIDs []int64
	for _, edge := range obj.Roles.Edges {
		roleIDs = append(roleIDs, edge.Node.ID)
	}
	thunks := r.Loaders.RoleLoader.LoadMany(ctx, roleIDs)
	roles, err := thunks()
	if err != nil {
		return nil, goerr.Wrap(fmt.Errorf("fetch by dataloader: %+v", err))
	}

	for i, role := range roles {
		roles[i] = role
	}
	return &model.RoleConnection{Nodes: roles}, nil
}

// Card returns CardResolver implementation.
func (r *Resolver) Card() CardResolver { return &cardResolver{r} }

// CardGroup returns CardGroupResolver implementation.
func (r *Resolver) CardGroup() CardGroupResolver { return &cardGroupResolver{r} }

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

// Role returns RoleResolver implementation.
func (r *Resolver) Role() RoleResolver { return &roleResolver{r} }

// User returns UserResolver implementation.
func (r *Resolver) User() UserResolver { return &userResolver{r} }

type cardResolver struct{ *Resolver }
type cardGroupResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type roleResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
