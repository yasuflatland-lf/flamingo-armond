package backend

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"os/exec"
)

type DBConfig struct {
	Host     string
	User     string
	Password string
	DBName   string
	Port     string
	SSLMode  string
}

type Postgress struct {
	Config DBConfig
	DB     *gorm.DB
}

func NewPostgress(config DBConfig) *Postgress {
	return &Postgress{Config: config}
}

func (pg *Postgress) DSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		pg.Config.Host, pg.Config.User, pg.Config.Password, pg.Config.DBName, pg.Config.Port, pg.Config.SSLMode,
	)
}

func (pg *Postgress) Open() error {
	dsn := pg.DSN()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}
	pg.DB = db
	return nil
}

func (pg *Postgress) runGooseMigrations(dsn string) error {
	cmd := exec.Command("goose", "-dir", "db/migrations", "postgres", dsn, "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("goose migration failed: %w", err)
	}
	return nil
}
