package config

import (
	"github.com/caarlos0/env/v11"
	"golang.org/x/xerrors"
)

// Config structure holds all the configuration values
type Config struct {
	// System Settings
	Port  int    `env:"PORT" envDefault:"1323"`
	GoEnv string `env:"GO_ENV" envDefault:"dev"`

	// GraphQL related configurations
	GQLComplexity int `env:"GQL_COMPLEXITY" envDefault:"10"`

	// PostgreSQL configuration
	PGHost     string `env:"PG_HOST" envDefault:"localhost"`
	PGUser     string `env:"PG_USER" envDefault:"testuser"`
	PGPassword string `env:"PG_PASSWORD" envDefault:"testpassword"`
	PGDBName   string `env:"PG_DBNAME" envDefault:"flamingodb"`
	PGPort     string `env:"PG_PORT" envDefault:"5432"`
	PGSSLMode  string `env:"PG_SSLMODE" envDefault:"disable"`
}

// Cfg is the package-level variable that holds the parsed configuration
var Cfg Config

// init function initializes the package-level variable Cfg by parsing environment variables
func init() {
	if err := env.Parse(&Cfg); err != nil {
		xerrors.Errorf("Failed to parse environment variables: %+v", err)
	}
}
