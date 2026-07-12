# Realtime Gameplay Specification

## Purpose

Define the player-visible WebSocket lifecycle for lobby, PvP, bot games, turns, outcomes, recovery, rematches, and room exit.

## Requirements

### Requirement: Lobby entry

The server SHALL place a newly registered WebSocket player in the lobby and notify the client with `lobby_joined`.

#### Scenario: Enter the lobby

- **WHEN** an authenticated player opens a new WebSocket connection without a reconnectable game
- **THEN** the Hub records the player locally, initializes the shared player state, and sends `lobby_joined`

### Requirement: PvP matchmaking request

The server SHALL accept `join_queue` only from a lobby player and SHALL notify the player with `queue_joined` after successful queue insertion.

#### Scenario: Join the PvP queue

- **WHEN** a lobby player sends `join_queue`
- **THEN** the player enters the matchmaking queue and receives `queue_joined`

#### Scenario: Ignore an invalid queue transition

- **WHEN** a player who is not in the lobby sends `join_queue`
- **THEN** the server does not enqueue that player again

### Requirement: Bot game creation

The server SHALL create a room with a simulated bot player when a lobby player sends `start_bot_game`, using easy, medium, or hard move selection behavior.

#### Scenario: Start a selected bot difficulty

- **WHEN** a lobby player sends `start_bot_game` with a supported difficulty
- **THEN** the server creates a Redis game, a local Room, and a bot that submits moves through the Room move channel

#### Scenario: Start without a difficulty

- **WHEN** a lobby player sends `start_bot_game` without a difficulty
- **THEN** the server creates the game using medium difficulty

### Requirement: Initial room assignment

The server SHALL assign each participant X or O and SHALL send the initial board and current turn after a room is created.

#### Scenario: Receive game assignment

- **WHEN** a PvP match or bot room is ready
- **THEN** each connected human player receives an `assignment` message and the current `update`

### Requirement: Validated turn processing

The server SHALL accept a move only from a participant whose mark owns the current turn, whose requested position is within the 3x3 board, and whose target cell is empty.

#### Scenario: Apply a valid move

- **WHEN** the current player selects an empty in-bounds cell
- **THEN** the mark is stored, the next turn is selected, and the updated game state is published

#### Scenario: Reject an invalid move

- **WHEN** a player moves out of turn, targets an occupied cell, selects an out-of-bounds position, or acts after game completion
- **THEN** the game state remains unchanged

### Requirement: Game outcome calculation

The server SHALL evaluate rows, columns, diagonals, and board fullness after a move to determine a winner or draw.

#### Scenario: Complete a winning line

- **WHEN** the updated board contains three identical non-empty marks in a row, column, or diagonal
- **THEN** the game is marked finished with that mark as winner

#### Scenario: Fill the board without a winner

- **WHEN** the board becomes full and no winning line exists
- **THEN** the game is marked finished as a draw

### Requirement: State broadcast

The server SHALL broadcast authoritative board, next-turn, and winner state to the room's connected players after a published room update.

#### Scenario: Receive an updated board

- **WHEN** the Room update subscriber observes a message for its room channel
- **THEN** it reloads authoritative state and sends an `update` message to local room players

### Requirement: Turn timeout proxy

The Room SHALL submit a legal proxy move when the local current player does not move before the configured turn timeout.

#### Scenario: Player exceeds the turn timer

- **WHEN** a connected local player does not move before the timer expires and the game is active
- **THEN** the Room calculates an easy legal move and processes it through the normal move handler

### Requirement: Connection health and forfeit

The Room SHALL use WebSocket ping messages to detect disconnected human players and SHALL award a forfeit after the reconnect grace period when an opponent remains connected.

#### Scenario: Detect a failed heartbeat

- **WHEN** a ping write fails for a connected human player
- **THEN** the Room marks that player disconnected and records the disconnect time

#### Scenario: Reconnect grace period expires

- **WHEN** a disconnected player remains absent beyond the grace period while the opponent is connected
- **THEN** the Room records the connected player's mark as winner and broadcasts the final result

### Requirement: Local room reconnection

The server SHALL reconnect an offline player when Redis references an existing game and the current Hub instance owns the corresponding local Room.

#### Scenario: Reconnect to a local room

- **WHEN** an authenticated player marked offline reconnects and the Room exists in the current Hub
- **THEN** the stale connection is replaced and the player receives the latest assignment and game state

#### Scenario: Room is not local

- **WHEN** Redis references an existing game but the current Hub does not own the local Room
- **THEN** the player is returned to the lobby instead of being attached to that game

### Requirement: Rematch voting

The server SHALL permit rematch votes only after game completion and SHALL reset the room after both human players accept; a bot opponent SHALL accept automatically.

#### Scenario: Both players accept a rematch

- **WHEN** both human participants vote to rematch after a completed game
- **THEN** the board and votes are reset, player sides are swapped, and new assignments and state are sent

#### Scenario: Request rematch before completion

- **WHEN** a player requests a rematch while the game is active
- **THEN** the server rejects the transition and retains the current game

### Requirement: Room exit

The server SHALL return human participants to the lobby and delete the Redis room when a player sends `leave_room`.

#### Scenario: Leave an active room

- **WHEN** a human player sends `leave_room`
- **THEN** the opponent is notified, human players return to the lobby, and the room state is removed
