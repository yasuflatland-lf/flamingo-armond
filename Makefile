RED=`tput setaf 1`
GREEN=`tput setaf 2`
WHITE=`tput setaf 7`
RESET=`tput sgr0`

ORG_PATH := $(CURDIR)

PG_HOST := localhost
PG_USER := testuser
PG_PASSWORD := testpassword
PG_DBNAME := testdb
PG_PORT := 5432
PG_SSLMODE := disable

ENVS = \
	export PG_HOST=$(PG_HOST); \
	export PG_USER=$(PG_USER); \
	export PG_PASSWORD=$(PG_PASSWORD); \
	export PG_DBNAME=$(PG_DBNAME); \
	export PG_PORT=$(PG_PORT); \
	export PG_SSLMODE=$(PG_SSLMODE);

.PHONY: server
server: ## Run Application
	@$(ENVS) docker-compose up --build

.PHONY: stop
stop: ## Stop the servers
	@$(ENVS) docker-compose down

.PHONY: help
help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	sed 's/Makefile://' | \
	awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
