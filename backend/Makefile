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

.PHONY: clear_gen
clear_gen: ## Clear GraphQL implementations
	printf "${GREEN} Clean up files\n\n"; \
	cd ./graph; rm -fR generated.go resolver.go schema.resolvers.go model; cd ${ORG_PATH} ; \
	printf "${GREEN}Done\n"; \

.PHONY: gen
gen: ## Generate GraphQL implementations
	printf "${GREEN} Update gqlgen\n\n"; \
 	go get github.com/99designs/gqlgen@latest; cd ${ORG_PATH} ; \
	printf "${GREEN} Generating GraphQL related sources\n\n"; \
	go run ./cmd/gqlgenerate; cd ${ORG_PATH}; \
	printf "${GREEN}Done\n"; \

.PHONY: fmt
fmt: ## Format code
	go fmt ./...; \
	printf "${GREEN} Code formatted.\n\n"; \

.PHONY: test
test: ## Run tests
	printf "${GREEN}Run all tests\n\n${WHITE}"; \
	go clean -cache -testcache -i -r; \
 	go test -cover -race -json -v ./... ; \
	printf "${GREEN}Done\n"; \

# Directories to exclude from coverage (use | to separate multiple directories)
EXCLUDE_DIRS=backend/pkg/notion

.PHONY: pre-coverage
pre-coverage: ## Generate coverage report for all packages
	@echo "mode: count" > coverage.out
	@for d in $(shell go list ./...); do \
		skip=false; \
		for exclude in $(EXCLUDE_DIRS); do \
			case $$d in *$$exclude*) skip=true;; esac; \
		done; \
		if [ "$$skip" = false ]; then \
			output=$$(go test -coverprofile=profile.out -covermode=count $$d 2>&1); \
			coverage=$$(echo "$$output" | awk '/coverage:/{print $$2}' | sed 's/%//'); \
			case "$$output" in \
				*\[no\ test\ files\]*) echo "Skipping $$d: no test files";; \
				*coverage:\ 0.0*) echo "Skipping $$d: coverage is 0.0%";; \
				*) \
					echo "$$output"; \
					[ -f profile.out ] && tail -n +2 profile.out >> coverage.out && rm profile.out;; \
			esac; \
		fi \
	done

.PHONY: cover
cover: pre-coverage ## Generate report from coverage data
	@TOTAL_COVERAGE=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{print $$3}'); \
	printf "\033[32mTotal coverage: %s\033[0m\n" "$$TOTAL_COVERAGE"; \
	rm -f coverage.out;

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
	goyacc -o ./pkg/textdic/parser.go -p yy ./pkg/textdic/parser.y; \
	rm -fR y.output; \
	printf "${GREEN}Done\n${WHITE}"; \

.PHONY: help
help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sed s/Makefile:// | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
