package services

import (
	"backend/graph/model"
	"gorm.io/gorm"

	"context"
)

//go:generate mockgen -source=$GOFILE -package=$GOPACKAGE -destination=../../mock/$GOPACKAGE/service_mock.go
type Services interface {
	CardService
}

type CardService interface {
	GetCardByID(ctx context.Context, id int64) (*model.Card, error)
	CreateCard(ctx context.Context, input model.NewCard) (*model.Card, error)
	UpdateCard(ctx context.Context, id int64, input model.NewCard) (*model.Card, error)
	DeleteCard(ctx context.Context, id int64) (bool, error)
	Cards(ctx context.Context) ([]*model.Card, error)
	CardsByCardGroup(ctx context.Context, cardGroupID int64) ([]*model.Card, error)
}

type services struct {
	*cardService
}

func New(db *gorm.DB) Services {
	return &services{
		cardService: &cardService{db: db},
	}
}
