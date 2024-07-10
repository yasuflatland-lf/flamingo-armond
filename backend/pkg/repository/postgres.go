package repository

import (
	"fmt"
	"os"
	"os/exec"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DBConfig struct {
	Host     string
	User     string
	Password string
	DBName   string
	Port     string
	SSLMode  string
}

type Postgres struct {
	Config DBConfig
	DB     *gorm.DB
}

func NewPostgres(config DBConfig) *Postgres {
	return &Postgres{Config: config}
}

func (pg *Postgres) DSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		pg.Config.Host, pg.Config.User, pg.Config.Password, pg.Config.DBName, pg.Config.Port, pg.Config.SSLMode,
	)
}

func (pg *Postgres) Open() error {
	dsn := pg.DSN()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}
	pg.DB = db
	return nil
}

func (pg *Postgres) RunGooseMigrations(path string) error {
	dsn := pg.DSN()
	cmd := exec.Command("goose", "-dir", path, "postgres", dsn, "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("goose migration failed: %w", err)
	}
	return nil
}
