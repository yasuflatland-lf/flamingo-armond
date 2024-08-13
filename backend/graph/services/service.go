package services

import (
	"backend/pkg/config"
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

type services struct {
	*cardService
	*cardGroupService
	*userService
	*roleService
	*swipeRecordService
}

func New(db *gorm.DB) Services {
	return &services{
		cardService:        &cardService{db: db, defaultLimit: config.Cfg.PGQueryLimit},
		cardGroupService:   &cardGroupService{db: db, defaultLimit: config.Cfg.PGQueryLimit},
		userService:        &userService{db: db, defaultLimit: config.Cfg.PGQueryLimit},
		roleService:        &roleService{db: db, defaultLimit: config.Cfg.PGQueryLimit},
		swipeRecordService: &swipeRecordService{db: db, defaultLimit: config.Cfg.PGQueryLimit},
	}
}
