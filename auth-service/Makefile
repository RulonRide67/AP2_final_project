.PHONY: help deps tidy lint test build run docker-up docker-down migrate-up migrate-down \
	proto proto-install proto-deps proto-lint proto-breaking proto-clean docker-build

APP_NAME := auth-service
CMD := ./cmd/auth
BINARY := bin/$(APP_NAME)
DOCKER_COMPOSE := docker compose

# Protobuf / Buf
BUF            ?= buf
PROTO_ROOT     := api/proto
PROTO_FILE     := $(PROTO_ROOT)/auth/v1/auth.proto
GEN_DIR        := internal/delivery/grpc/gen

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

migrate-up: ## Apply database migrations (requires golang-migrate CLI)
	@test -n "$$DATABASE_URL" || (echo "Set DATABASE_URL in .env (see .env.example)" && exit 1)
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down: ## Roll back the last database migration
	@test -n "$$DATABASE_URL" || (echo "Set DATABASE_URL in .env (see .env.example)" && exit 1)
	migrate -path migrations -database "$$DATABASE_URL" down 1

# ---------------------------------------------------------------------------
# Protobuf generation (Buf + protoc-gen-go + protoc-gen-go-grpc + protoc-gen-validate)
# ---------------------------------------------------------------------------

proto-install: ## Install Buf CLI and Go protoc plugins (validate, go, grpc)
	@echo "Installing protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.35.2
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	go install github.com/envoyproxy/protoc-gen-validate@latest
	@echo "Install Buf from https://buf.build/docs/installation if not present: $(BUF)"

proto-deps: ## Pull protobuf dependencies (protoc-gen-validate validate.proto)
	$(BUF) dep update

proto-lint: proto-deps ## Lint protobuf files
	$(BUF) lint

proto-breaking: proto-deps ## Check for breaking changes against main
	$(BUF) breaking --against '.git#branch=main'

proto-clean: ## Remove generated protobuf Go code
	rm -rf $(GEN_DIR)

proto: proto-install proto-deps ## Generate Go gRPC stubs and PGV validation code
	@mkdir -p $(GEN_DIR)
	$(BUF) generate
	@echo "Generated: $(GEN_DIR)/auth/v1/"

# Fallback: raw protoc (no Buf CLI). Set PROTOC_INCLUDE to protoc-gen-validate include path.
.PHONY: proto-protoc
proto-protoc: proto-install ## Generate via protoc directly (alternative to buf generate)
	@test -n "$(PROTOC_VALIDATE_INCLUDE)" || (echo "Set PROTOC_VALIDATE_INCLUDE to protoc-gen-validate include dir (e.g. $$(go list -m -f '{{.Dir}}' github.com/envoyproxy/protoc-gen-validate)/validate)" && exit 1)
	@mkdir -p $(GEN_DIR)/auth/v1
	protoc \
		-I $(PROTO_ROOT) \
		-I $(PROTOC_VALIDATE_INCLUDE)/.. \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		--validate_out=lang=go,paths=source_relative:$(GEN_DIR) \
		$(PROTO_FILE)

docker-build: ## Build application Docker image
	docker build -f deployments/docker/Dockerfile -t $(APP_NAME):latest .
