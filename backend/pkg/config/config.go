package config

import (
	"fmt"
	"github.com/caarlos0/env/v11"
	"log/slog"
)

const (
	APP_MODE_DEV  = "dev"
	APP_MODE_PROD = "production"
	APP_MODE_TEST = "test"
)

var validEnvs = []string{APP_MODE_DEV, APP_MODE_PROD, APP_MODE_TEST}

// Config structure holds all the configuration values
type Config struct {
	// System Settings
	Port  int    `env:"PORT,notEmpty" envDefault:"1323"`
	GoEnv string `env:"GO_ENV,notEmpty" envDefault:"test"`

	// GraphQL related configurations
	GQLComplexity int `env:"GQL_COMPLEXITY,notEmpty" envDefault:"10"`

	// PostgresSQL configuration
	PGHost       string `env:"PG_HOST,notEmpty" envDefault:"localhost"`
	PGUser       string `env:"PG_USER,notEmpty" envDefault:"testuser"`
	PGPassword   string `env:"PG_PASSWORD,notEmpty" envDefault:"testpassword"`
	PGDBName     string `env:"PG_DBNAME,notEmpty" envDefault:"flamingodb"`
	PGPort       string `env:"PG_PORT,notEmpty" envDefault:"5432"`
	PGSSLMode    string `env:"PG_SSLMODE,notEmpty" envDefault:"allow"`
	PGQueryLimit int    `env:"PG_QUERY_LIMIT,notEmpty" envDefault:"100"`

	// Application configuration
	JWTSecret            string `env:"FL_JWT_SECRET,notEmpty" envDefault:"jwt_secret to be replaced."`
	FLBatchDefaultAmount int    `env:"FL_BATCH_DEFAULT_AMOUNT,notEmpty" envDefault:"10"`
}

// Cfg is the package-level variable that holds the parsed configuration
var Cfg Config

// init function initializes the package-level variable Cfg by parsing environment variables
func init() {
	if err := env.Parse(&Cfg); err != nil {
		slog.Error("Failed to parse environment variables: %+v", err)
	}

	if !isValidEnv(Cfg.GoEnv) {
		slog.Error(fmt.Sprintf("Invalid GO_ENV value: %s. Must be one of %v", Cfg.GoEnv, validEnvs))
	}

	if Cfg.PGQueryLimit <= Cfg.FLBatchDefaultAmount {
		slog.Error(fmt.Sprintf("FLBatchDefaultAmount<%d> must be smaller than PGQueryLimit<%d>",
			Cfg.FLBatchDefaultAmount, Cfg.PGQueryLimit))
	}
}

// isValidEnv checks if the provided env is valid
func isValidEnv(env string) bool {
	for _, v := range validEnvs {
		if env == v {
			return true
		}
	}
	return false
}

func IsTest() bool {
	return Cfg.GoEnv == APP_MODE_TEST
}
