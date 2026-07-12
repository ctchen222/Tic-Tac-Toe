# Tic-Tac-Toe Project Status

Status date: 2026-07-12

This document is descriptive, not normative. Accepted requirements live under `openspec/specs/`; future behavior starts as an OpenSpec change.

## Current Baseline

| Capability | Maturity | Current behavior | Evidence |
|---|---|---|---|
| Authentication | Implemented | Register, login, persistent guest users, JWT issuance, authenticated WebSocket admission | `internal/api/`, `internal/server/server.go`, `internal/db/db.go` |
| Realtime gameplay | Implemented with limitations | Lobby, PvP, PvE, turns, winner/draw, timeout proxy, heartbeat, rematch, local-room reconnect | `internal/hub/`, `internal/room/`, `internal/game/`, `internal/bot/`, `web/js/game.js` |
| Persistence and matchmaking | Implemented with limitations | Redis player/room state, list matchmaking, Pub/Sub, atomic move update; SQLite accounts | `internal/repository/`, `internal/db/`, `docker-compose.yml` |
| Observability | Implemented baseline | OTLP traces, metrics, logs; Jaeger, Prometheus, Loki, Grafana composition | `internal/telemetry/`, `internal/logger/`, `otel-collector-config.yaml`, `prometheus.yml` |
| Automated verification | Partial | Game, bot, and user repository tests pass; Room and Hub orchestration lack active tests; the full suite does not compile | `internal/**/*_test.go` |

## Runtime Topology

```text
Browser UI
   ├─ REST auth ──> Gin ──> Controller ──> Service ──> SQLite
   └─ WebSocket ──> Gin ──> Hub ──> Room ──> Redis
                               ├─ Game rules
                               └─ Bot move injection

Go OTel SDK ──> OTel Collector ──┬─> Jaeger
                                  ├─> Prometheus ──> Grafana
                                  └─> Loki ────────> Grafana
```

Interactive references:

- `docs/tic-tac-toe-overview.html`
- `docs/tic-tac-toe-module-map.html`
- `docs/tic-tac-toe-game-runtime.html`

## Known Limitations

| Area | Current limitation | Consequence |
|---|---|---|
| Horizontal scaling | Hub players and Room handlers are local maps; reconnect requires the Room on the current instance | Redis shares state, but WebSocket ownership and reconnect are not fully cross-instance |
| Authentication | JWT signing key is hard-coded in source | Secret rotation and production-safe configuration are not implemented |
| WebSocket security | Origin checks currently accept every origin | Cross-origin admission is not restricted |
| Input safety | Move tracing indexes `position` before checking its length | A malformed move payload can cause an out-of-range panic |
| SQLite | Account storage is a local database file | Multiple server instances do not share account state |
| Redis durability | Compose does not configure Redis persistence volumes | Local container state can be lost when Redis is recreated |
| Match history | The table exists, but completed games are not written to it | Results are not durably queryable |
| Social model | The friends table exists, but no API or service implements friendship behavior | Friends are schema-only, not a product capability |
| Matchmaking stream | Stream methods exist but the active matcher uses a Redis list | Stream-based consumption is incomplete and non-normative |

## Verification Status

The intended repository gate is:

```sh
go test ./...
openspec validate --all --strict
```

At this baseline, `go test ./...` fails during compilation of `internal/match/match_test.go` because legacy tests reference the removed `room.Player` type. Focused tests for `internal/game`, `internal/bot`, and `internal/api/repository` pass. `internal/room` and the Hub orchestration currently report no active test files. Focused results must not be interpreted as proof that the complete repository gate passes.

## Planned Work

The following ideas are not accepted requirements and require future change proposals:

- Repair or retire the legacy `internal/match` test surface.
- Move JWT secrets and security settings into production configuration.
- Validate all WebSocket message payload shapes before indexing fields.
- Implement cross-instance Room ownership and reconnect routing.
- Persist completed match results.
- Implement friends and item systems.
- Decide whether Redis Stream matchmaking replaces the active list queue.
- Add durable Redis and shared account storage for multi-instance deployment.

## Spec-Driven Development Workflow

1. Create an OpenSpec change for new or modified behavior.
2. Identify affected capabilities and add delta requirements and scenarios.
3. Implement tasks with focused automated verification.
4. Run `openspec validate --all --strict` and the relevant code tests.
5. Verify implementation against the change artifacts.
6. Archive the change so accepted behavior updates the main specs.
