package repository

import (
	"backend/pkg/utils"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
	"os/exec"
)

// DBConfig holds the database configuration details.
type DBConfig struct {
	Host              string
	User              string
	Password          string
	DBName            string
	Port              string
	SSLMode           string
	MigrationFilePath string
}

// Postgres represents a PostgreSQL database connection.
type Postgres struct {
	Config DBConfig
	DB     *gorm.DB
}

// NewPostgres creates a new instance of Postgres with the given configuration.
func NewPostgres(config DBConfig) *Postgres {
	return &Postgres{Config: config}
}

// GetDB returns the underlying gorm.DB instance.
func (pg *Postgres) GetDB() *gorm.DB {
	return pg.DB
}

// DSN returns the Data Source Name for the PostgreSQL connection.
func (pg *Postgres) DSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		pg.Config.Host, pg.Config.User, pg.Config.Password, pg.Config.DBName, pg.Config.Port, pg.Config.SSLMode,
	)
}

// Open establishes a database connection.
func (pg *Postgres) Open() error {
	dsn := pg.DSN()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}
	pg.DB = db
	return nil
}

// RunGooseMigrationsUp runs the Goose migrations to upgrade the database schema.
func (pg *Postgres) RunGooseMigrationsUp(path string) error {
	dsn := pg.DSN()
	cmd := exec.Command("goose", "-dir", path, "postgres", dsn, "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("goose migration failed: %w", err)
	}
	return nil
}

// RunGooseMigrationsDown runs the Goose migrations to downgrade the database schema.
func (pg *Postgres) RunGooseMigrationsDown(path string) error {
	dsn := pg.DSN()
	cmd := exec.Command("goose", "-dir", path, "postgres", dsn, "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("goose migration down failed: %w", err)
	}
	return nil
}

// InitializeDatabase encapsulates the database configuration and initialization logic
func InitializeDatabase(config DBConfig) *gorm.DB {
	// Initialize the Postgres instance
	pg := NewPostgres(config)

	// Open the database connection
	if err := pg.Open(); err != nil {
		log.Fatalf("failed to connect to the database: %v", err)
	}

	// Get Full path to the migration DB file.
	fullPath, err := utils.GetFullPath(config.MigrationFilePath)
	if err != nil {
		log.Fatalf("Failed to get full path to the migration db file : %+v", err)
	}

	// Run migrations
	log.Printf("Data Migration start ===============")
	if err := pg.RunGooseMigrationsUp(fullPath); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Printf("Data Migration Done ===============")

	return pg.DB
}
