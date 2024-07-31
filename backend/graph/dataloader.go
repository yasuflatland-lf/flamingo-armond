package graph

import (
	"backend/graph/model"
	"backend/graph/services"
	"context"
	"errors"
	"strconv"

	"github.com/graph-gophers/dataloader/v7"
)

type Loaders struct {
	CardLoader      dataloader.Interface[string, *model.Card]
	UserLoader      dataloader.Interface[string, *model.User]
	RoleLoader      dataloader.Interface[string, *model.Role]
	CardGroupLoader dataloader.Interface[string, *model.CardGroup]
}

func NewLoaders(srv services.Services) *Loaders {
	cardBatcher := &cardBatcher{Srv: srv}
	userBatcher := &userBatcher{Srv: srv}
	roleBatcher := &roleBatcher{Srv: srv}
	cardGroupBatcher := &cardGroupBatcher{Srv: srv}

	return &Loaders{
		CardLoader:      dataloader.NewBatchedLoader[string, *model.Card](cardBatcher.BatchGetCards),
		UserLoader:      dataloader.NewBatchedLoader[string, *model.User](userBatcher.BatchGetUsers),
		RoleLoader:      dataloader.NewBatchedLoader[string, *model.Role](roleBatcher.BatchGetRoles),
		CardGroupLoader: dataloader.NewBatchedLoader[string, *model.CardGroup](cardGroupBatcher.BatchGetCardGroups),
	}
}

type cardBatcher struct {
	Srv services.Services
}

func (c *cardBatcher) BatchGetCards(ctx context.Context, keys []string) []*dataloader.Result[*model.Card] {
	ids := make([]int64, len(keys))
	for i, key := range keys {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return []*dataloader.Result[*model.Card]{{
				Error: err,
			}}
		}
		ids[i] = id
	}

	cards, err := c.Srv.GetCardsByIDs(ctx, ids)
	if err != nil {
		return make([]*dataloader.Result[*model.Card], len(keys))
	}

	cardMap := make(map[int64]*model.Card)
	for _, card := range cards {
		cardMap[card.ID] = card
	}

	results := make([]*dataloader.Result[*model.Card], len(keys))
	for i, key := range keys {
		id, _ := strconv.ParseInt(key, 10, 64)
		if card, ok := cardMap[id]; ok {
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

func (u *userBatcher) BatchGetUsers(ctx context.Context, keys []string) []*dataloader.Result[*model.User] {
	ids := make([]int64, len(keys))
	for i, key := range keys {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return []*dataloader.Result[*model.User]{{
				Error: err,
			}}
		}
		ids[i] = id
	}

	users, err := u.Srv.GetUsersByIDs(ctx, ids)
	if err != nil {
		return make([]*dataloader.Result[*model.User], len(keys))
	}

	userMap := make(map[int64]*model.User)
	for _, user := range users {
		userMap[user.ID] = user
	}

	results := make([]*dataloader.Result[*model.User], len(keys))
	for i, key := range keys {
		id, _ := strconv.ParseInt(key, 10, 64)
		if user, ok := userMap[id]; ok {
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

func (r *roleBatcher) BatchGetRoles(ctx context.Context, keys []string) []*dataloader.Result[*model.Role] {
	ids := make([]int64, len(keys))
	for i, key := range keys {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return []*dataloader.Result[*model.Role]{{
				Error: err,
			}}
		}
		ids[i] = id
	}

	roles, err := r.Srv.GetRolesByIDs(ctx, ids)
	if err != nil {
		return make([]*dataloader.Result[*model.Role], len(keys))
	}

	roleMap := make(map[int64]*model.Role)
	for _, role := range roles {
		roleMap[role.ID] = role
	}

	results := make([]*dataloader.Result[*model.Role], len(keys))
	for i, key := range keys {
		id, _ := strconv.ParseInt(key, 10, 64)
		if role, ok := roleMap[id]; ok {
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

func (c *cardGroupBatcher) BatchGetCardGroups(ctx context.Context, keys []string) []*dataloader.Result[*model.CardGroup] {
	ids := make([]int64, len(keys))
	for i, key := range keys {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return []*dataloader.Result[*model.CardGroup]{{
				Error: err,
			}}
		}
		ids[i] = id
	}

	cardGroups, err := c.Srv.GetCardGroupsByIDs(ctx, ids)
	if err != nil {
		return make([]*dataloader.Result[*model.CardGroup], len(keys))
	}

	cardGroupMap := make(map[int64]*model.CardGroup)
	for _, cardGroup := range cardGroups {
		cardGroupMap[cardGroup.ID] = cardGroup
	}

	results := make([]*dataloader.Result[*model.CardGroup], len(keys))
	for i, key := range keys {
		id, _ := strconv.ParseInt(key, 10, 64)
		if cardGroup, ok := cardGroupMap[id]; ok {
			results[i] = &dataloader.Result[*model.CardGroup]{Data: cardGroup}
		} else {
			results[i] = &dataloader.Result[*model.CardGroup]{Error: errors.New("card group not found")}
		}
	}
	return results
}
