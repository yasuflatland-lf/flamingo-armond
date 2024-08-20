package services

import (
	"backend/pkg/config"
	"github.com/m-mizutani/goerr"
	"golang.org/x/net/context"
	"gorm.io/gorm"
)

//go:generate mockgen -source=$GOFILE -package=$GOPACKAGE -destination=../../mock/$GOPACKAGE/service_mock.go
type Services interface {
	CardService
	CardGroupService
	UserService
	RoleService
	SwipeRecordService
	BeginTx(ctx context.Context) (*gorm.DB, error)
}

type services struct {
	*cardService
	*cardGroupService
	*userService
	*roleService
	*swipeRecordService
	db *gorm.DB
}

func New(db *gorm.DB) Services {
	return &services{
		cardService:        &cardService{db: db, defaultLimit: config.Cfg.PGQueryLimit},
		cardGroupService:   &cardGroupService{db: db, defaultLimit: config.Cfg.PGQueryLimit},
		userService:        &userService{db: db, defaultLimit: config.Cfg.PGQueryLimit},
		roleService:        &roleService{db: db, defaultLimit: config.Cfg.PGQueryLimit},
		swipeRecordService: &swipeRecordService{db: db, defaultLimit: config.Cfg.PGQueryLimit},
		db:                 db,
	}
}

func (s *services) BeginTx(ctx context.Context) (*gorm.DB, error) {
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, goerr.Wrap(tx.Error)
	}
	return tx, nil
}
