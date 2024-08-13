package config

import (
	"github.com/caarlos0/env/v11"
	"log/slog"
)

// Config structure holds all the configuration values
type Config struct {
	// System Settings
	Port  int    `env:"PORT,notEmpty" envDefault:"1323"`
	GoEnv string `env:"GO_ENV,notEmpty" envDefault:"dev"`

	// GraphQL related configurations
	GQLComplexity int `env:"GQL_COMPLEXITY,notEmpty" envDefault:"10"`

	// PostgresSQL configuration
	PGHost       string `env:"PG_HOST,notEmpty" envDefault:"localhost"`
	PGUser       string `env:"PG_USER,notEmpty" envDefault:"testuser"`
	PGPassword   string `env:"PG_PASSWORD,notEmpty" envDefault:"testpassword"`
	PGDBName     string `env:"PG_DBNAME,notEmpty" envDefault:"flamingodb"`
	PGPort       string `env:"PG_PORT,notEmpty" envDefault:"5432"`
	PGSSLMode    string `env:"PG_SSLMODE,notEmpty" envDefault:"allow"`
	PGQueryLimit int    `env:"PG_QUERY_LIMIT,notEmpty" envDefault:"20"`
}

// Cfg is the package-level variable that holds the parsed configuration
var Cfg Config

// init function initializes the package-level variable Cfg by parsing environment variables
func init() {
	if err := env.Parse(&Cfg); err != nil {
		slog.Error("Failed to parse environment variables: %+v", err)
	}
}
