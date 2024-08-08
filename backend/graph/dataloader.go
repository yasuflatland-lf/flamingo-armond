package graph

import (
	"backend/graph/model"
	"backend/graph/services"
	"backend/pkg/logger"
	"context"
	"errors"

	"github.com/graph-gophers/dataloader/v7"
)

type Loaders struct {
	CardLoader        dataloader.Interface[int64, *model.Card]
	UserLoader        dataloader.Interface[int64, *model.User]
	RoleLoader        dataloader.Interface[int64, *model.Role]
	CardGroupLoader   dataloader.Interface[int64, *model.CardGroup]
	SwipeRecordLoader dataloader.Interface[int64, *model.SwipeRecord]
}

func NewLoaders(srv services.Services) *Loaders {
	cardBatcher := &cardBatcher{Srv: srv}
	userBatcher := &userBatcher{Srv: srv}
	roleBatcher := &roleBatcher{Srv: srv}
	cardGroupBatcher := &cardGroupBatcher{Srv: srv}
	swipeRecordBatcher := &swipeRecordBatcher{Srv: srv}

	return &Loaders{
		CardLoader:        dataloader.NewBatchedLoader[int64, *model.Card](cardBatcher.BatchGetCards),
		UserLoader:        dataloader.NewBatchedLoader[int64, *model.User](userBatcher.BatchGetUsers),
		RoleLoader:        dataloader.NewBatchedLoader[int64, *model.Role](roleBatcher.BatchGetRoles),
		CardGroupLoader:   dataloader.NewBatchedLoader[int64, *model.CardGroup](cardGroupBatcher.BatchGetCardGroups),
		SwipeRecordLoader: dataloader.NewBatchedLoader[int64, *model.SwipeRecord](swipeRecordBatcher.BatchGetSwipeRecords),
	}
}

type cardBatcher struct {
	Srv services.Services
}

func (c *cardBatcher) BatchGetCards(ctx context.Context, keys []int64) []*dataloader.Result[*model.Card] {
	cards, err := c.Srv.GetCardsByIDs(ctx, keys)
	if err != nil {
		logger.Logger.ErrorContext(ctx, "No cards found", err)
		return make([]*dataloader.Result[*model.Card], len(keys))
	}

	cardMap := make(map[int64]*model.Card)
	for _, card := range cards {
		cardMap[card.ID] = card
	}

	results := make([]*dataloader.Result[*model.Card], len(keys))
	for i, key := range keys {
		if card, ok := cardMap[key]; ok {
			results[i] = &dataloader.Result[*model.Card]{Data: card}
		} else {
			results[i] = &dataloader.Result[*model.Card]{Error: errors.New("card not found")}
		}
	}
	return results
}

type userBatcher struct {
	Srv services.Services
}

func (u *userBatcher) BatchGetUsers(ctx context.Context, keys []int64) []*dataloader.Result[*model.User] {
	users, err := u.Srv.GetUsersByIDs(ctx, keys)
	if err != nil {
		logger.Logger.ErrorContext(ctx, "No users found", err)
		return make([]*dataloader.Result[*model.User], len(keys))
	}

	userMap := make(map[int64]*model.User)
	for _, user := range users {
		userMap[user.ID] = user
	}

	results := make([]*dataloader.Result[*model.User], len(keys))
	for i, key := range keys {
		if user, ok := userMap[key]; ok {
			results[i] = &dataloader.Result[*model.User]{Data: user}
		} else {
			results[i] = &dataloader.Result[*model.User]{Error: errors.New("user not found")}
		}
	}
	return results
}

type roleBatcher struct {
	Srv services.Services
}

func (r *roleBatcher) BatchGetRoles(ctx context.Context, keys []int64) []*dataloader.Result[*model.Role] {
	roles, err := r.Srv.GetRolesByIDs(ctx, keys)
	if err != nil {
		logger.Logger.ErrorContext(ctx, "No roles found", err)
		return make([]*dataloader.Result[*model.Role], len(keys))
	}

	roleMap := make(map[int64]*model.Role)
	for _, role := range roles {
		roleMap[role.ID] = role
	}

	results := make([]*dataloader.Result[*model.Role], len(keys))
	for i, key := range keys {
		if role, ok := roleMap[key]; ok {
			results[i] = &dataloader.Result[*model.Role]{Data: role}
		} else {
			results[i] = &dataloader.Result[*model.Role]{Error: errors.New("role not found")}
		}
	}
	return results
}

type cardGroupBatcher struct {
	Srv services.Services
}

func (c *cardGroupBatcher) BatchGetCardGroups(ctx context.Context, keys []int64) []*dataloader.Result[*model.CardGroup] {
	cardGroups, err := c.Srv.GetCardGroupsByIDs(ctx, keys)
	if err != nil {
		logger.Logger.ErrorContext(ctx, "No cardGroups found", err)
		return make([]*dataloader.Result[*model.CardGroup], len(keys))
	}

	cardGroupMap := make(map[int64]*model.CardGroup)
	for _, cardGroup := range cardGroups {
		cardGroupMap[cardGroup.ID] = cardGroup
	}

	results := make([]*dataloader.Result[*model.CardGroup], len(keys))
	for i, key := range keys {
		if cardGroup, ok := cardGroupMap[key]; ok {
			results[i] = &dataloader.Result[*model.CardGroup]{Data: cardGroup}
		} else {
			results[i] = &dataloader.Result[*model.CardGroup]{Error: errors.New("card group not found")}
		}
	}
	return results
}

type swipeRecordBatcher struct {
	Srv services.Services
}

func (s *swipeRecordBatcher) BatchGetSwipeRecords(ctx context.Context, keys []int64) []*dataloader.Result[*model.SwipeRecord] {
	swipeRecords, err := s.Srv.GetSwipeRecordsByIDs(ctx, keys)
	if err != nil {
		logger.Logger.ErrorContext(ctx, "No swipe records found", err)
		return make([]*dataloader.Result[*model.SwipeRecord], len(keys))
	}

	swipeRecordMap := make(map[int64]*model.SwipeRecord)
	for _, swipeRecord := range swipeRecords {
		swipeRecordMap[swipeRecord.ID] = swipeRecord
	}

	results := make([]*dataloader.Result[*model.SwipeRecord], len(keys))
	for i, key := range keys {
		if swipeRecord, ok := swipeRecordMap[key]; ok {
			results[i] = &dataloader.Result[*model.SwipeRecord]{Data: swipeRecord}
		} else {
			results[i] = &dataloader.Result[*model.SwipeRecord]{Error: errors.New("swipe record not found")}
		}
	}
	return results
}
