package main

import (
	"backend/pkg/config"
	"backend/pkg/repository"
	"backend/web/server"
)

var migrationFilePath = "./db/migrations"

func main() {
	dbConfig := repository.DBConfig{
		Host:              config.Cfg.PGHost,
		User:              config.Cfg.PGUser,
		Password:          config.Cfg.PGPassword,
		DBName:            config.Cfg.PGDBName,
		Port:              config.Cfg.PGPort,
		SSLMode:           config.Cfg.PGSSLMode,
		MigrationFilePath: migrationFilePath,
	}
	server.StartServer(dbConfig)
}
