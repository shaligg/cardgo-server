# Cardgo Server

A scalable WebSocket game server built in Go for **casual card games**, designed for real-time multiplayer gameplay with 2,000 concurrent connections per node.

## Overview

Cardgo Server is a game server demo that implements a complete client-to-database pipeline: login &rarr; ticket-based auth &rarr; WebSocket connection &rarr; sharded business dispatch &rarr; service layer &rarr; repository &rarr; MySQL/Redis.

### Core Features

- **WebSocket Gateway** — connection upgrade, HMAC ticket authentication, nonce-based replay protection, heartbeat, rate limiting, graceful shutdown
- **Shard Dispatcher** — per-player serial execution via 64-way sharded locks; ensures data consistency without blocking different players
- **6 Game Modules** — Player, Asset, Card, Battle, Inventory, Workshop; each with its own service + repository + model
- **Session Management** — in-process connection binding, Redis-backed cross-node ownership, kick-on-relogin
- **Idempotency** — command cache with `req_id` + payload hash to handle network retry safely
- **State Recovery** — same-node memory reuse, database reconstruction when local state is unavailable, TTL cleanup
- **Metrics** — connection count, auth success/failure, rate-limited, queue-kick counters

## Project Structure

```
cmd/gameserver/          # Entry point
internal/
  app/gameserver/        # Bootstrap, lifecycle, config
  framework/
    gateway/ws/          # WebSocket server, client, codec, limiter
    dispatcher/          # Shard executor (per-player serial)
    transport/           # DTO, error codes
  game/                  # Business logic (player, asset, card, battle, inventory, workshop)
  handler/               # Router, dispatcher, protocol handlers
  repo/                  # Data access layer (GORM + MySQL)
  platform/
    auth/                # Ticket verifier, nonce store
    session/             # Session manager, command cache
    login/               # Ticket issuer, node allocator
    state/               # Online state, ownership reconciliation, TTL cleanup
  infra/
    db/                  # DB manager, transaction manager
    redis/               # Redis client, node registry
    log/                  # Structured logger
    metrics/             # Metrics registry
    websearch/           # Wikipedia OpenSearch client
  gamedata/              # Static game data (items, levels)
  contract/protocol/     # Op codes, request/response types
configs/                 # Runtime configuration by environment
```

## Quick Start

### Prerequisites

- Go 1.21+
- MySQL 8.0+
- Redis 6.0+

### Build & Run

```bash
# Install dependencies
go mod tidy

# Build
go build -o bin/gameserver ./cmd/gameserver

# Run (requires MySQL and Redis)
export GAME_DB_DSN='game:password@tcp(127.0.0.1:3306)/game_demo?charset=utf8mb4&parseTime=True&loc=Local'
export GAME_TICKET_SECRET='local-dev-ticket-secret'
./bin/gameserver

# Or run directly
GAME_DB_DSN="$GAME_DB_DSN" GAME_TICKET_SECRET="$GAME_TICKET_SECRET" go run ./cmd/gameserver
```

The API server listens on `:8080`; the WebSocket server listens on `:8081/ws`.

### Run Tests

```bash
go test ./...

# Run MySQL integration tests as well.
GAME_TEST_DB_DSN='game_test:password@tcp(127.0.0.1:3306)/game_test?charset=utf8mb4&parseTime=True&loc=Local' go test ./...
```

## Configuration

Edit the environment file under `configs/`. Database credentials are not written to YAML; the DSN is read from the environment variable named by `db.dsn_env_key`.

## Architecture

```
Client ──WS──▶ Gateway ──▶ Auth ──▶ Dispatcher (shard) ──▶ Router
                                                              ├──▶ Player Service
                                                              ├──▶ Asset Service
                                                              ├──▶ Card Service
                                                              ├──▶ Battle Service
                                                              ├──▶ Inventory Service
                                                              └──▶ Workshop Service
                                                                    │
                                                                    ▼
                                                               Repository
                                                                    │
                                                                    ▼
                                                               MySQL + Redis
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.21+ |
| WebSocket | gorilla/websocket |
| ORM | GORM + MySQL |
| Shared State | Redis (go-redis/v9) |
| Auth | HMAC-SHA256 ticket + nonce |
| Logging | Structured logger (leveled) |
| UUID | google/uuid |

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for details.
