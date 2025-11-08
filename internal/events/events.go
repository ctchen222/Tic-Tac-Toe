package events

import "encoding/json"

// Pub/Sub channel constants
const (
	EventsChannel = "channel:events"
)

// Event represents a global message published via Pub/Sub.
type Event struct {
	Type    string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

// MatchMadePayload is the payload for the "match_made" event.
type MatchMadePayload struct {
	RoomID    string   `json:"room_id"`
	PlayerIDs []string `json:"player_ids"`
}

// PlayerDisconnectedPayload is the payload for the "player_disconnected" event.
type PlayerDisconnectedPayload struct {
	RoomID   string `json:"room_id"`
	PlayerID string `json:"player_id"`
}

// PlayerReconnectedPayload is the payload for the "player_reconnected" event.
type PlayerReconnectedPayload struct {
	RoomID   string `json:"room_id"`
	PlayerID string `json:"player_id"`
}

// RematchRequestedPayload is the payload for the "rematch_requested" event.
type RematchRequestedPayload struct {
	RoomID   string `json:"room_id"`
	PlayerID string `json:"player_id"`
}

// RematchSuccessfulPayload is the payload for the "rematch_successful" event.
type RematchSuccessfulPayload struct {
	RoomID string `json:"room_id"`
}
