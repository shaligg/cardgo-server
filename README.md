# Cardgo Server

A scalable WebSocket game server built in Go for **casual card games**, designed for real-time multiplayer gameplay with 2,000 concurrent connections per node.

## Overview

Cardgo Server is a game server demo that implements a complete client-to-database pipeline: login &rarr; ticket-based auth &rarr; WebSocket connection &rarr; sharded business dispatch &rarr; service layer &rarr; repository &rarr; MySQL/Redis.

### Core Features

- **Independent LoginServer** — HTTP login, Redis node allocation and signed entry tickets; clients connect directly to the assigned GameServer
- **WebSocket Gateway** — connection upgrade, HMAC ticket authentication, nonce-based replay protection, heartbeat, rate limiting, graceful shutdown
- **Shard Dispatcher** — per-player serial execution via 64-way sharded locks; ensures data consistency without blocking different players
- **6 Business Modules** — Player, Asset, Inventory under `domain`; Card, Battle, Workshop under `gameplay`
- **Session Management** — in-process connection binding, Redis-backed cross-node ownership, kick-on-relogin
- **Idempotency** — command cache with `req_id` + payload hash to handle network retry safely
- **State Recovery** — same-node memory reuse, database reconstruction when local state is unavailable, TTL cleanup
- **Metrics** — connection count, auth success/failure, rate-limited, queue-kick counters

## Project Structure

```
cmd/loginserver/         # Login HTTP entry point
cmd/gameserver/          # GameServer entry point
internal/
  app/loginserver/       # Login config, Redis wiring and HTTP lifecycle
  app/gameserver/        # Bootstrap, lifecycle, config
  framework/
    gateway/ws/          # WebSocket server, client, codec, limiter
    dispatcher/          # Shard executor (per-player serial)
    transport/           # DTO, error codes
  domain/                # Reusable business capabilities (player, asset, inventory)
  gameplay/              # Concrete gameplay orchestration (card, battle, workshop)
  globalcore/            # Cross-player domain contracts and local implementations
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

- Go 1.24.2+
- MySQL 8.0+
- Redis 6.0+

### Build & Run

```bash
# Install dependencies
go mod download

# Build
go build -o bin/gameserver ./cmd/gameserver
go build -o bin/loginserver ./cmd/loginserver
```

Start GameServer in terminal 1 (create the MySQL database first and start Redis):

```bash
export GAME_DB_DSN='game:password@tcp(127.0.0.1:3306)/game_demo?charset=utf8mb4&parseTime=True&loc=Local'
export GAME_TICKET_SECRET='local-dev-ticket-secret'
GAME_CONFIG=configs/gameserver.local.yaml ./bin/gameserver
```

Start LoginServer in terminal 2 with the same ticket secret:

```bash
export GAME_TICKET_SECRET='local-dev-ticket-secret'
GAME_CONFIG=configs/loginserver.local.yaml ./bin/loginserver
```

Alternatively, replace the binary commands with `go run ./cmd/gameserver` and `go run ./cmd/loginserver` in their respective terminals.

LoginServer listens on `:8080` (`POST /api/login`, `/healthz`). GameServer listens on `:8081/ws` and exposes management, health and metrics on `:8082`. GameServer does not expose `/api/login`. LoginServer needs Redis and the ticket secret, but no MySQL connection. Stop either process with Ctrl-C; stopping LoginServer leaves existing GameServer connections alive.

### Run Tests

```bash
go test ./...

# Run MySQL integration tests as well.
GAME_TEST_DB_DSN='game_test:password@tcp(127.0.0.1:3306)/game_test?charset=utf8mb4&parseTime=True&loc=Local' go test ./...

# Explicit process acceptance: use a separate test database and free local ports.
LOGIN_SPLIT_TEST_DB_DSN='game_test:password@tcp(127.0.0.1:3306)/game_test?charset=utf8mb4&parseTime=True&loc=Local' go test ./scripts/loadtest/loginserver_split -v -count=1
```

## Configuration

Edit `configs/gameserver.{local,staging,prod}.yaml` and `configs/loginserver.{local,staging,prod}.yaml`. Each process accepts `GAME_CONFIG`, defaulting to its own local file. Both must use the same ticket issuer, algorithm, secret and Redis instance/DB/key prefixes. Database credentials are read only by GameServer from the environment variable named by `db.dsn_env_key`.

For another GameServer, use a unique `server.node_id`, WS/Admin ports and `server.advertised_ws_addr`. LoginServer discovers it through Redis without restarting. See [the runbook](docs/ops/runbook.md) for startup, drain, monitoring and acceptance commands. The demo still treats `account` as UID; formal account authentication is not implemented.

## Architecture

```
Login (independent LoginServer):
  Client -> HTTP :8080 -> Redis NodeAllocator -> TicketIssuer
  Client <- ws_addr + server_id + enter_ticket

GameServer (client connects directly to the assigned node):
Client ──WS──▶ Gateway ──▶ Auth ──▶ Dispatcher (shard) ──▶ Router / Handler
                                                                  │
                 ┌────────────────────────────────────────────────┴──────────┐
                 ▼                                                           ▼
       Gameplay Service                                               Domain Service
    (Card/Battle/Workshop)                                       (Player/Asset/Inventory)
                 │                                                           │
                 └──────────────▶ Domain Service ────────────────────────────┘
                                     │
                                     ▼
                                Repository
                                     │
                                     ▼
                                   MySQL
```

See the [architecture overview](architecture_v2.md), [technical architecture](backend_technical_architecture.md) and [documentation index](docs/README.md).

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.24.2+ |
| WebSocket | gorilla/websocket |
| ORM | GORM + MySQL |
| Shared State | Redis (go-redis/v9) |
| Auth | HMAC-SHA256 ticket + nonce |
| Logging | Structured logger (leveled) |
| UUID | google/uuid |

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for details.
