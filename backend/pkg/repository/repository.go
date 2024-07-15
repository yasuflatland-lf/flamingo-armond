package repository

import "gorm.io/gorm"

type Repository interface {
	GetConfig() DBConfig
	GetDB() *gorm.DB
	DSN() string
	Open() error
	RunGooseMigrationsUp(path string) error
	RunGooseMigrationsDown(path string) error
}
