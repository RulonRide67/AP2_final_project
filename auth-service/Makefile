.PHONY: help deps tidy lint test build run docker-up docker-down migrate-up migrate-down proto

APP_NAME := auth-service
CMD := ./cmd/auth
BINARY := bin/$(APP_NAME)
DOCKER_COMPOSE := docker compose -f deployments/docker-compose.yml

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

deps: ## Download Go module dependencies
	go mod download

tidy: ## Tidy go.mod / go.sum
	go mod tidy

lint: ## Run golangci-lint (install separately)
	golangci-lint run ./...

test: ## Run unit tests
	go test -race -cover ./...

build: ## Build the auth service binary
	@mkdir -p bin
	go build -o $(BINARY) $(CMD)

run: build ## Run the service locally (requires .env)
	set -a && [ -f .env ] && . ./.env; set +a && $(BINARY)

docker-up: ## Start Postgres, Redis, NATS, Prometheus, Grafana
	$(DOCKER_COMPOSE) up -d

docker-down: ## Stop infrastructure containers
	$(DOCKER_COMPOSE) down

docker-logs: ## Tail docker-compose logs
	$(DOCKER_COMPOSE) logs -f

migrate-up: ## Apply database migrations (implement migrate tool)
	@echo "TODO: wire golang-migrate or goose"

migrate-down: ## Roll back database migrations
	@echo "TODO: wire golang-migrate or goose"

proto: ## Generate gRPC code from protobuf definitions
	@echo "TODO: buf generate or protoc -I api/proto ..."

.PHONY: docker-build
docker-build: ## Build application Docker image
	docker build -f deployments/docker/Dockerfile -t $(APP_NAME):latest .
