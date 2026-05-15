# Auth Microservice

Production-oriented authentication microservice scaffold in **Go**, structured with **Clean Architecture**. Business logic is not implemented yet—this repository defines layout, tooling, and infrastructure wiring only.

## Stack

| Concern        | Technology              |
|----------------|-------------------------|
| API            | gRPC                    |
| Database       | PostgreSQL              |
| Cache / sessions | Redis               |
| Messaging      | NATS                    |
| Auth tokens    | JWT                     |
| Metrics        | Prometheus              |
| Dashboards     | Grafana                 |
| Containers     | Docker Compose          |

## Architecture

Dependencies point inward: delivery and infrastructure depend on use cases; use cases depend on domain ports.

```
                    ┌─────────────────────────────────────┐
                    │           cmd/auth (main)           │
                    └──────────────────┬──────────────────┘
                                       │
         ┌─────────────────────────────┼─────────────────────────────┐
         ▼                             ▼                             ▼
  internal/delivery/grpc      internal/platform/*          internal/repository/*
  (gRPC handlers)             (config, logger, metrics)    (postgres, redis, nats)
         │                             │                             │
         └─────────────────────────────┼─────────────────────────────┘
                                       ▼
                            internal/usecase (application)
                                       │
                                       ▼
                            internal/domain (entities + ports)
```

### Layers

- **`internal/domain`** — Entities and repository/publisher interfaces (no framework imports).
- **`internal/usecase`** — Application services orchestrating domain ports.
- **`internal/delivery/grpc`** — gRPC transport, request/response mapping, interceptors.
- **`internal/repository/postgres`** — PostgreSQL persistence adapters.
- **`internal/repository/redis`** — Redis cache/session adapters.
- **`internal/repository/nats`** — Event publisher adapters.
- **`internal/auth/jwt`** — JWT signing and validation utilities.
- **`internal/platform`** — Cross-cutting config, logging, Prometheus metrics.
- **`pkg/grpc`** — Shared gRPC middleware (auth, logging, recovery).
- **`api/proto`** — Protobuf contracts and generated code target.

## Project layout

```
.
├── api/proto/auth/v1/          # gRPC API contracts
├── cmd/auth/                   # Application entrypoint
├── internal/
│   ├── domain/                 # Entities & ports
│   ├── usecase/                # Application layer
│   ├── delivery/grpc/          # gRPC server & handlers
│   ├── repository/             # postgres | redis | nats adapters
│   ├── auth/jwt/               # JWT helpers
│   └── platform/               # config, logger, metrics
├── pkg/grpc/                   # Reusable gRPC middleware
├── migrations/                 # SQL migrations
├── deployments/                # Dockerfile & docker-compose
├── monitoring/                 # Prometheus & Grafana config
└── scripts/                    # Dev/ops helper scripts
```

## Quick start

1. Copy environment template:

   ```bash
   cp .env.example .env
   ```

2. Start infrastructure:

   ```bash
   make docker-up
   ```

3. Install dependencies and build:

   ```bash
   make deps
   make build
   ```

4. Prometheus: `http://localhost:9091` · Grafana: `http://localhost:3000` (see `.env.example`).

## Makefile targets

Run `make help` for the full list (`build`, `test`, `docker-up`, `proto`, etc.).

## Next steps (implementation)

- Define protobuf services under `api/proto/auth/v1/`.
- Implement domain entities and repository interfaces.
- Wire use cases in `cmd/auth/main.go`.
- Add SQL migrations under `migrations/`.
- Generate gRPC stubs (`make proto`) and implement handlers.

## License

Private / course project — adjust as needed.
