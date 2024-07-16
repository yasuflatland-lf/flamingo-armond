package main

import (
	"backend/web/server"
)

var migrationFilePath = "./db/migrations"

func main() {
	server.StartServer(migrationFilePath)
}
