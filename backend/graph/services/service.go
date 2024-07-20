package services

import (
	"backend/graph/model"
	"gorm.io/gorm"

	"context"
)

//go:generate mockgen -source=$GOFILE -package=$GOPACKAGE -destination=../../mock/$GOPACKAGE/service_mock.go
type Services interface {
	UserService
}

type CardService interface {
	GetUserByID(ctx context.Context, id string) (*model.User, error)
}

type services struct {
	*cardService
}

func New(db *gorm.DB) Services {
	return &services{
		cardService: &cardService{db: db},
	}
}
