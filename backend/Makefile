# Include all env file and extract as environment valuables entire this Makefile.
include .env
.EXPORT_ALL_VARIABLES:

RED=`tput setaf 1`
GREEN=`tput setaf 2`
WHITE=`tput setaf 7`
RESET=`tput sgr0`

ORG_PATH := ${CURDIR}

ENVS = \
	export TF_VAR_HOME=$(HOME); \

.PHONY: update
update: ## Update go.mod
	go mod tidy

.PHONY: gen
gen: ## Generate GraphQL implementations
	printf "${GREEN} Clean up files\n\n"; \
	cd ./graph; rm -fR generated.go resolver.go schema.resolvers.go model; cd ${ORG_PATH} ; \
	printf "${GREEN} Update gqlgen\n\n"; \
 	go get github.com/99designs/gqlgen@latest; cd ${ORG_PATH} ; \
	printf "${GREEN} Generating GraphQL related sources\n\n"; \
 	go get github.com/99designs/gqlgen@latest; cd ${ORG_PATH} ; \
	go run github.com/99designs/gqlgen generate; cd ${ORG_PATH}; \
	printf "${GREEN}Done\n"; \

.PHONY: fmt
fmt: ## Format code
	go fmt ./...; \
	printf "${GREEN} Code formatted.\n\n"; \

.PHONY: test
test: ## Run tests
	printf "${GREEN}Run all tests\n\n${WHITE}"; \
	go clean -testcache; \
 	go test -v -race -run=./... -bench=./... ./...; \
	printf "${GREEN}Done\n"; \

.PHONY: init
init: ## Init go.mod and go.sum
	printf "${GREEN}Init go.mod and go.sum\n\n${WHITE}"; \
	rm -fR go.mod go.sum; \
	go mod init backend; \
	go mod tidy;
	printf "${GREEN}Done\n${WHITE}"; \

.PHONY: parsergen
parsergen: ## Generate Card data parser
	printf "${GREEN}Generate parser.go\n\n${WHITE}"; \
	go install golang.org/x/tools/cmd/goyacc@master ; \
	goyacc -o ./pkg/notion/parser.go -p yy ./pkg/notion/parser.y; \
	rm -fR y.output; \
	printf "${GREEN}Done\n${WHITE}"; \

.PHONY: help
help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sed s/Makefile:// | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
