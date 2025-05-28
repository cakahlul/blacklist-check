.PHONY: build run test migrate-up migrate-down docker-build docker-up docker-down

# Build the application
build:
	go build -o bin/server ./cmd/server

# Run the application
run:
	go run ./cmd/server

# Run tests
test:
	go test -v -race -cover ./...

# Database migrations
migrate-up:
	migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/blacklist?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/blacklist?sslmode=disable" down

# Docker commands
docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# Development setup
setup:
	go mod download
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest 