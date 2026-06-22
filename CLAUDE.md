# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ICPC 无锡学院 Linux 集控系统 — a centralized remote control system for managing Linux contestant machines in ICPC (competitive programming) competitions.

- **Server**: Deployed on Linux, publishes via Avahi mDNS (`icpc-server.local`), provides a web UI
- **Client (选手机)**: Contestant machines running as root, connect to server, report system info, accept remote commands

## Commands

```bash
# Build both binaries
go build ./...

# Run server (HTTP on :8080, TCP on :8081 by default)
go run ./cmd/server/main.go
go run ./cmd/server/main.go --port 8080 --tcp-port 8081 --db icpc.db --bind 192.168.1.1

# Run client (connects to icpc-server.local:8081 by default)
go run ./cmd/client/main.go

# Run tests
go test ./...
go test -run TestName ./pkg/go-silver-core/...
```

## Architecture

### Two-process design

**Server** (`cmd/server/`) exposes:
- HTTP/WebSocket on `:8080` — web UI (embedded in binary via `//go:embed`), REST API, admin WebSocket (`/ws/admin`), browser terminal (`/ws/terminal/{id}`), broadcast display pages (`/broadcast/*`)
- TCP on `:8081` — raw connections from contestant machines using the GSP protocol

**Client** (`cmd/client/`) runs as root on each contestant machine and maintains:
- A persistent TCP connection to the server (heartbeat every 15s, reconnect every 5s on failure)
- A local HTTP server on `:8090` for contestant check-in (embedded HTML page)
- X11 watermark overlay (`watermark.go` via XGB + Shape extension) and screen capture stream (`screen.go`)

### Custom protocol: GSP (go-silver-core)

All TCP messages use the in-tree library at `pkg/go-silver-core/`, a local module (`replace go-silver-core => ./pkg/go-silver-core`).

Packet format: `[Type uint8][Length uint32][Payload bytes]`
- `TypeJSON = 0x01` — control messages (JSON-encoded structs from `internal/model/message.go`)
- `TypeFileChunk = 0x02` — binary file distribution chunks

All control message types are discriminated by a `"type"` JSON field. See `internal/model/message.go` for the full set.

### Key message flows

**Registration**: Client → `register` → Server assigns incremental ID (persisted to `/var/lib/icpc-client/id` on client) → `register_response` with `assigned_id` and `hostname_prefix` → client runs `hostnamectl set-hostname {prefix}-{ID}`.

**Command execution**: Admin browser → `POST /api/commands` → `CommandHandler` → `CommandDispatcher.DispatchSingle/DispatchBroadcast` → `execute` message over TCP → client runs shell command → streaming `command_output` messages → `command_result` → Hub fans out to admin WebSockets.

**File distribution**: Uses `go-silver-core` chunked transfer. Server acts as sender; clients pull chunks. Progress reported via `distribute_progress` messages.

**Check-in**: Contestant submits form on `:8090` → client sends `checkin` over TCP with correlation ID → server validates and persists → `checkin_response` back to client → client HTTP handler unblocks via `checkinWaiters` map.

### Dependency injection (server `main.go`)

All wiring happens in `cmd/server/main.go` — no DI framework. Construction order: DB → repos → settings → biz layer (Hub, IDAssigner, CommandDispatcher) → service handlers → HTTP server + TCP listener.

### Layer responsibilities

| Layer | Package | Responsibility |
|---|---|---|
| model | `internal/model/` | Wire protocol structs (messages, fastfetch, device, command, broadcast) |
| data | `internal/data/` | SQLite CRUD via `modernc.org/sqlite` (no CGo) |
| biz | `internal/biz/` | Hub (connection registry), IDAssigner, CommandDispatcher |
| service | `internal/service/` | HTTP handlers, TCP handler, terminal hub, settings, auth, distribution, broadcast WS |
| server | `internal/server/` | Route registration, middleware (auth, logging, recovery), Avahi launch, embed |

### Hub

`biz.Hub` is the central connection registry. It maintains:
- `clients map[int]*ClientConn` — active TCP connections keyed by assigned ID
- `admins map[*AdminConn]bool` — active admin WebSocket connections

All client/admin connect and disconnect events flow through channel-based `Run()` loop, and are fanned out to admin browsers as `AdminEvent` WebSocket messages.

### Authentication

JWT-based. `AuthHandler` issues tokens on `POST /api/auth/login`. The `AuthMiddleware` in `server.go` wraps the entire mux. Credentials and JWT secret are stored in SQLite via `SettingsRepo`. Rate limiting (5 attempts → block) is enforced in `LoginRateLimiter`.

### Frontend

All static assets live under `internal/server/web/` and are embedded at compile time. Stack: jQuery, CodeMirror (command editor), xterm.js (browser terminal), custom CSS. No build step — files are served directly.

### Broadcast display

Separate full-screen pages (`/broadcast/before`, `/broadcast/contesting`, `/broadcast/after`) consumed by contestant-machine browsers. State is pushed via `/ws/broadcast` WebSocket (`service.BroadcastWS` global). Client machines can query current state via `query_broadcast_state` TCP message.

## Key files

- `internal/model/message.go` — all TCP protocol message types (client↔server and server→browser)
- `internal/biz/hub.go` — connection registry and admin event fan-out
- `internal/service/tcp_handler.go` — server-side per-connection TCP read loop and message dispatch
- `cmd/client/main.go` — client TCP loop, check-in HTTP bridge, command execution
- `cmd/server/main.go` — server wiring / DI root
- `pkg/go-silver-core/internal/gsp/` — GSP packet codec
