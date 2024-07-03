# Backend of Flamingo Armond

# Precondition
- Go 1.22.5
- Goose v3

# Set up
## Install Goose
```
go install github.com/pressly/goose/v3/cmd/goose@latest
```

## Install libraries
```
go install
```

# Test
## How to Run All Tests
```
go clean -testcache
go test -v ./...
```

# How to run Locally
At the root directory, run 
```
docker compose up --build
```