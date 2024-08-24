package config_test

import (
	"os"
	"testing"

	"backend/pkg/config"
	"github.com/caarlos0/env/v11"
	"github.com/stretchr/testify/assert"
)

func TestConfigDefaults(t *testing.T) {
	// Explicitly set environment variables
	os.Setenv("PORT", "")
	os.Setenv("GO_ENV", "")
	os.Setenv("GQL_COMPLEXITY", "")
	os.Setenv("PG_HOST", "")
	os.Setenv("PG_USER", "")
	os.Setenv("PG_PASSWORD", "")
	os.Setenv("PG_DBNAME", "")
	os.Setenv("PG_PORT", "")
	os.Setenv("PG_SSLMODE", "")

	// Parse environment variables
	err := env.Parse(&config.Cfg)
	assert.NoError(t, err, "Config should parse without error")

	// Verify default values
	assert.Equal(t, 1323, config.Cfg.Port, "Default Port should be 1323")
	assert.Equal(t, "test", config.Cfg.GoEnv, "Default GoEnv should be 'dev'")
	assert.Equal(t, 10, config.Cfg.GQLComplexity, "Default GQLComplexity should be 10")
	assert.Equal(t, "localhost", config.Cfg.PGHost, "Default PGHost should be 'localhost'")
	assert.Equal(t, "testuser", config.Cfg.PGUser, "Default PGUser should be 'testuser'")
	assert.Equal(t, "testpassword", config.Cfg.PGPassword, "Default PGPassword should be 'testpassword'")
	assert.Equal(t, "flamingodb", config.Cfg.PGDBName, "Default PGDBName should be 'flamingodb'")
	assert.Equal(t, "5432", config.Cfg.PGPort, "Default PGPort should be '5432'")
	assert.Equal(t, "allow", config.Cfg.PGSSLMode, "Default PGSSLMode should be 'allow'")
}

func TestConfigCustomValues(t *testing.T) {
	// Set environment variables to custom values
	os.Setenv("PORT", "8080")
	os.Setenv("GO_ENV", "production")
	os.Setenv("GQL_COMPLEXITY", "20")
	os.Setenv("PG_HOST", "customhost")
	os.Setenv("PG_USER", "customuser")
	os.Setenv("PG_PASSWORD", "custompassword")
	os.Setenv("PG_DBNAME", "customdb")
	os.Setenv("PG_PORT", "6543")
	os.Setenv("PG_SSLMODE", "require")

	// Parse environment variables
	err := env.Parse(&config.Cfg)
	assert.NoError(t, err, "Config should parse without error")

	// Verify custom values
	assert.Equal(t, 8080, config.Cfg.Port, "Port should be 8080")
	assert.Equal(t, "production", config.Cfg.GoEnv, "GoEnv should be 'production'")
	assert.Equal(t, 20, config.Cfg.GQLComplexity, "GQLComplexity should be 20")
	assert.Equal(t, "customhost", config.Cfg.PGHost, "PGHost should be 'customhost'")
	assert.Equal(t, "customuser", config.Cfg.PGUser, "PGUser should be 'customuser'")
	assert.Equal(t, "custompassword", config.Cfg.PGPassword, "PGPassword should be 'custompassword'")
	assert.Equal(t, "customdb", config.Cfg.PGDBName, "PGDBName should be 'customdb'")
	assert.Equal(t, "6543", config.Cfg.PGPort, "PGPort should be '6543'")
	assert.Equal(t, "require", config.Cfg.PGSSLMode, "PGSSLMode should be 'require'")
}
