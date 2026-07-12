# Persistence and Matchmaking Specification

## Purpose

Define the current shared-state, atomicity, matchmaking, event distribution, and account persistence guarantees.

## Requirements

### Requirement: Required Redis connectivity

The server MUST establish and verify a Redis connection during startup before accepting traffic.

#### Scenario: Redis is available

- **WHEN** the process starts with a reachable Redis endpoint
- **THEN** the client ping succeeds and the repositories and Hub are initialized

#### Scenario: Redis is unavailable

- **WHEN** the startup ping fails
- **THEN** the server exits instead of serving without shared game state

### Requirement: Shared player state

The player repository SHALL store server ownership, lobby or game status, connection status, and room identifier in a Redis player hash with a seven-day expiration when initialized.

#### Scenario: Initialize a lobby player

- **WHEN** a WebSocket player is registered in the lobby
- **THEN** Redis records the Hub server identifier, lobby status, connected status, empty room identifier, and expiry

#### Scenario: Assign a matched player

- **WHEN** matchmaking creates a room for a player
- **THEN** Redis records in-game status, connected status, and that room identifier

### Requirement: Shared room state

The game repository SHALL represent each room as a Redis hash containing board, X player, O player, next turn, winner, and status, with a ten-minute expiration at creation.

#### Scenario: Create a room

- **WHEN** a PvP or bot match is created
- **THEN** Redis stores an empty board, both participant identifiers, a randomized first turn, no winner, and in-progress status

### Requirement: Atomic move persistence

The game repository SHALL use Redis WATCH and a transaction pipeline to validate and persist a move against the latest room state.

#### Scenario: Commit against an unchanged room

- **WHEN** the watched room remains unchanged and the move is valid
- **THEN** the board, next turn, winner, and status are committed together

#### Scenario: Concurrent room modification

- **WHEN** another writer modifies the room before the transaction commits
- **THEN** the transaction fails rather than overwriting the concurrent state

### Requirement: Blocking two-player matchmaking

The matchmaking repository SHALL consume two player identifiers from the Redis matchmaking list before creating a PvP room.

#### Scenario: Match two waiting players

- **WHEN** at least two player identifiers are available
- **THEN** the matcher removes two identifiers, creates one room, updates both player states, and publishes a match event

#### Scenario: Second dequeue fails

- **WHEN** the first player was removed but retrieving the second player fails
- **THEN** the matcher attempts to return the first player to the queue

### Requirement: Distributed event channels

The system SHALL use a global Redis Pub/Sub channel for lifecycle events and a room-specific channel for game-state update notifications.

#### Scenario: Publish a match event

- **WHEN** the matcher successfully creates a room
- **THEN** it publishes `match_made` on the global event channel for Hub instances to consume

#### Scenario: Publish a room update

- **WHEN** a move is committed
- **THEN** the Room publishes on `channel:room:<room-id>` so local subscribers reload and broadcast authoritative state

### Requirement: SQLite account persistence

The service SHALL initialize SQLite schemas for users, friends, and match results, while the current account repository SHALL persist and retrieve user credentials through the users table.

#### Scenario: Initialize the database

- **WHEN** the server starts against a writable SQLite database
- **THEN** foreign keys are enabled and missing baseline tables are created

#### Scenario: Enforce unique usernames

- **WHEN** two account creations use the same username
- **THEN** SQLite uniqueness prevents duplicate user records
