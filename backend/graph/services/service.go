package services

import (
	"backend/graph/model"
	"context"
	"gorm.io/gorm"
)

//go:generate mockgen -source=$GOFILE -package=$GOPACKAGE -destination=../../mock/$GOPACKAGE/service_mock.go
type Services interface {
	CardService
	CardGroupService
	UserService
	RoleService
	SwipeRecordService
}

type CardService interface {
	GetCardByID(ctx context.Context, id int64) (*model.Card, error)
	CreateCard(ctx context.Context, input model.NewCard) (*model.Card, error)
	UpdateCard(ctx context.Context, id int64, input model.NewCard) (*model.Card, error)
	DeleteCard(ctx context.Context, id int64) (*bool, error)
	PaginatedCardsByCardGroup(ctx context.Context, cardGroupID int64, first *int, after *int64, last *int, before *int64) (*model.CardConnection, error)
	GetCardsByIDs(ctx context.Context, ids []int64) ([]*model.Card, error)
	FetchAllCardsByCardGroup(ctx context.Context, cardGroupID int64, first *int) ([]*model.Card, error)
	AddNewCards(ctx context.Context, targetCards []model.Card, cardGroupID int64) ([]*model.Card, error)
}

type CardGroupService interface {
	GetCardGroupByID(ctx context.Context, id int64) (*model.CardGroup, error)
	CreateCardGroup(ctx context.Context, input model.NewCardGroup) (*model.CardGroup, error)
	CardGroups(ctx context.Context) ([]*model.CardGroup, error)
	UpdateCardGroup(ctx context.Context, id int64, input model.NewCardGroup) (*model.CardGroup, error)
	DeleteCardGroup(ctx context.Context, id int64) (*bool, error)
	AddUserToCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error)
	RemoveUserFromCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error)
	GetCardGroupsByUser(ctx context.Context, userID int64) ([]*model.CardGroup, error)
	PaginatedCardGroupsByUser(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.CardGroupConnection, error)
	GetCardGroupsByIDs(ctx context.Context, ids []int64) ([]*model.CardGroup, error)
}

type UserService interface {
	GetUsersByRole(ctx context.Context, roleID int64) ([]*model.User, error)
	Users(ctx context.Context) ([]*model.User, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	CreateUser(ctx context.Context, input model.NewUser) (*model.User, error)
	UpdateUser(ctx context.Context, id int64, input model.NewUser) (*model.User, error)
	DeleteUser(ctx context.Context, id int64) (*bool, error)
	PaginatedUsersByRole(ctx context.Context, roleID int64, first *int, after *int64, last *int, before *int64) (*model.UserConnection, error)
	GetUsersByIDs(ctx context.Context, ids []int64) ([]*model.User, error)
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

type SwipeRecordService interface {
	GetSwipeRecordByID(ctx context.Context, id int64) (*model.SwipeRecord, error)
	CreateSwipeRecord(ctx context.Context, input model.NewSwipeRecord) (*model.SwipeRecord, error)
	UpdateSwipeRecord(ctx context.Context, id int64, input model.NewSwipeRecord) (*model.SwipeRecord, error)
	DeleteSwipeRecord(ctx context.Context, id int64) (*bool, error)
	SwipeRecords(ctx context.Context) ([]*model.SwipeRecord, error)
	SwipeRecordsByUser(ctx context.Context, userID int64) ([]*model.SwipeRecord, error)
	PaginatedSwipeRecordsByUser(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.SwipeRecordConnection, error)
	GetSwipeRecordsByIDs(ctx context.Context, ids []int64) ([]*model.SwipeRecord, error)
}

type services struct {
	*cardService
	*cardGroupService
	*userService
	*roleService
	*swipeRecordService
}

func New(db *gorm.DB) Services {
	return &services{
		cardService:        &cardService{db: db, defaultLimit: 20},
		cardGroupService:   &cardGroupService{db: db, defaultLimit: 20},
		userService:        &userService{db: db, defaultLimit: 20},
		roleService:        &roleService{db: db, defaultLimit: 20},
		swipeRecordService: &swipeRecordService{db: db, defaultLimit: 20},
	}
}
