# Cardgo Server

A scalable WebSocket game server built in Go for **casual card games**, designed for real-time multiplayer gameplay with 2,000 concurrent connections per node.

## Overview

Cardgo Server is a game server demo that implements a complete client-to-database pipeline: login &rarr; ticket-based auth &rarr; WebSocket connection &rarr; sharded business dispatch &rarr; service layer &rarr; repository &rarr; SQLite/Redis.

### Core Features

- **WebSocket Gateway** — connection upgrade, HMAC ticket authentication, nonce-based replay protection, heartbeat, rate limiting, graceful shutdown
- **Shard Dispatcher** — per-player serial execution via 64-way sharded locks; ensures data consistency without blocking different players
- **6 Game Modules** — Player, Asset, Card, Battle, Inventory, Workshop; each with its own service + repository + model
- **Session Management** — Redis-backed session store with bind/unbind, single-node ownership, kick-on-relogin
- **Idempotency** — command cache with `req_id` + payload hash to handle network retry safely
- **State Recovery** — offline state flush & restore on reconnect
- **L1 Cache** — in-process short-TTL read cache with sharded locks to reduce DB load
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
  repo/                  # Data access layer (GORM + SQLite)
  platform/
    auth/                # Ticket verifier, nonce store
    session/             # Session manager, command cache
    login/               # Ticket issuer, node allocator
    state/               # Online state, flush worker
  infra/
    cache/               # L1 cache
    db/                  # DB manager, transaction manager
    redis/               # Redis client, node registry
    log/                  # Structured logger
    metrics/             # Metrics registry
    websearch/           # Wikipedia OpenSearch client
  gamedata/              # Static game data (items, levels)
  contract/protocol/     # Op codes, request/response types
config.yaml              # Runtime configuration
```

## Quick Start

### Prerequisites

- Go 1.21+
- Redis 6.0+

### Build & Run

```bash
# Install dependencies
go mod tidy

# Build
go build -o bin/gameserver ./cmd/gameserver

# Run (requires Redis on localhost:6379)
./bin/gameserver

# Or run directly
go run ./cmd/gameserver
```

The server listens on `:8080` for WebSocket connections at `/ws` and health check at `/healthz`.

### Run Tests

```bash
go test ./...
```

## Configuration

Edit [config.yaml](config.yaml) for database, Redis, cache, and server settings.

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
                                                              SQLite + Redis
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.21+ |
| WebSocket | gorilla/websocket |
| ORM | GORM + SQLite |
| Cache | Redis (go-redis/v9) |
| Auth | HMAC-SHA256 ticket + nonce |
| Logging | Structured logger (leveled) |
| UUID | google/uuid |

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for details.
