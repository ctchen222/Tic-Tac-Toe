package repository

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"fmt"

	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("repository.player")

// PlayerRepository defines the interface for player data operations.
type PlayerRepository interface {
	FindForReconnection(ctx context.Context, id string) (roomID string, status player.PlayerStatus, err error)
	UpdateConnectionStatus(ctx context.Context, id string, status player.PlayerStatus) error
	SetInitialState(ctx context.Context, id, serverID string) error
	UpdateForMatch(ctx context.Context, id, roomID string) error
	SetOffline(ctx context.Context, id string) error
}

type redisPlayerRepository struct {
	rdb *redis.Client
}

// NewPlayerRepository creates a new Redis-based PlayerRepository.
func NewPlayerRepository(rdb *redis.Client) PlayerRepository {
	return &redisPlayerRepository{
		rdb: rdb,
	}
}

// FindForReconnection retrieves the necessary data for a player to reconnect.
func (r *redisPlayerRepository) FindForReconnection(ctx context.Context, id string) (string, player.PlayerStatus, error) {
	ctx, span := tracer.Start(ctx, "PlayerRepository.FindForReconnection")
	defer span.End()

	playerKey := fmt.Sprintf("player:%s", id)
	data, err := r.rdb.HGetAll(ctx, playerKey).Result()
	if err != nil {
		return "", "", err
	}
	return data["room_id"], player.PlayerStatus(data["connection_status"]), nil
}

// UpdateConnectionStatus updates only the connection status of a player.
func (r *redisPlayerRepository) UpdateConnectionStatus(ctx context.Context, id string, status player.PlayerStatus) error {
	ctx, span := tracer.Start(ctx, "PlayerRepository.UpdateConnectionStatus")
	defer span.End()

	playerKey := fmt.Sprintf("player:%s", id)
	return r.rdb.HSet(ctx, playerKey, "connection_status", string(status)).Err()
}

// SetInitialState sets the initial data for a newly registered player.
func (r *redisPlayerRepository) SetInitialState(ctx context.Context, id, serverID string) error {
	ctx, span := tracer.Start(ctx, "PlayerRepository.SetInitialState")
	defer span.End()

	playerKey := fmt.Sprintf("player:%s", id)
	pipe := r.rdb.Pipeline()
	pipe.HSet(ctx, playerKey, "server_id", serverID)
	pipe.HSet(ctx, playerKey, "status", "waiting")
	_, err := pipe.Exec(ctx)
	return err
}

// UpdateForMatch updates a player's state when they are put into a match.
func (r *redisPlayerRepository) UpdateForMatch(ctx context.Context, id, roomID string) error {
	ctx, span := tracer.Start(ctx, "PlayerRepository.UpdateForMatch")
	defer span.End()

	playerKey := fmt.Sprintf("player:%s", id)
	pipe := r.rdb.Pipeline()
	pipe.HSet(ctx, playerKey, "room_id", roomID)
	pipe.HSet(ctx, playerKey, "status", "in_game")
	pipe.HSet(ctx, playerKey, "connection_status", string(player.StatusConnected))
	_, err := pipe.Exec(ctx)
	return err
}

// SetOffline marks a player as offline, typically during unregistration.
func (r *redisPlayerRepository) SetOffline(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "PlayerRepository.SetOffline")
	defer span.End()

	playerKey := fmt.Sprintf("player:%s", id)
	return r.rdb.HSet(ctx, playerKey, "status", "offline").Err()
}
