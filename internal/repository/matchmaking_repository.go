package repository

import (
	"context"
	"log/slog"

	"github.com/go-redis/redis/v8"
)

// var tracer = otel.Tracer("repository.matchmaking")

const (
	matchmakingQueueKey     = "queue:matchmaking"
	matchmakingGroupName    = "matchmakers"
	matchmakingConsumerName = "matcher"
	matchmakingStreamKey    = "matchmaking"
)

// MatchmakingRepository defines the interface for matchmaking queue operations.
type MatchmakingRepository interface {
	// List
	AddToQueue(ctx context.Context, playerID string) error
	GetPlayersFromQueue(ctx context.Context) (player1ID, player2ID string, err error)
	RemoveFromQueue(ctx context.Context, playerID string) error

	// Stream
	StreamInit(ctx context.Context) error
	AddToStream(ctx context.Context, playerID string) error
}

type redisMatchmakingRepository struct {
	rdb *redis.Client
}

// NewMatchmakingRepository creates a new Redis-based MatchmakingRepository.
func NewMatchmakingRepository(rdb *redis.Client) MatchmakingRepository {
	return &redisMatchmakingRepository{rdb: rdb}
}

// AddToQueue adds a player to the matchmaking queue.
func (r *redisMatchmakingRepository) AddToQueue(ctx context.Context, playerID string) error {
	ctx, span := tracer.Start(ctx, "MatchmakingRepository.AddToQueue")
	defer span.End()

	return r.rdb.RPush(ctx, matchmakingQueueKey, playerID).Err()
}

// GetPlayersFromQueue blocks until two players are available in the queue and returns them.
func (r *redisMatchmakingRepository) GetPlayersFromQueue(ctx context.Context) (string, string, error) {
	ctx, span := tracer.Start(ctx, "MatchmakingRepository.GetPlayersFromQueue")
	defer span.End()

	// Block until one player is available
	player1Res, err := r.rdb.BLPop(ctx, 0, matchmakingQueueKey).Result()
	if err != nil {
		return "", "", err
	}
	player1ID := player1Res[1]
	slog.InfoContext(ctx, "Matcher found player 1, waiting for player 2...", "player.id", player1ID)

	// Block until a second player is available
	player2Res, err := r.rdb.BLPop(ctx, 0, matchmakingQueueKey).Result()
	if err != nil {
		slog.ErrorContext(ctx, "Matcher error on player 2, re-queuing player 1", "player.id", player1ID, "error", err)
		if requeueErr := r.AddToQueue(ctx, player1ID); requeueErr != nil {
			slog.ErrorContext(ctx, "FATAL: Failed to re-queue player", "player.id", player1ID, "error", requeueErr)
		}
		return "", "", err
	}
	player2ID := player2Res[1]
	slog.InfoContext(ctx, "Matcher found player 2. Creating match...", "player.id", player2ID)

	return player1ID, player2ID, nil
}

// RemoveFromQueue removes a specific player from the queue.
func (r *redisMatchmakingRepository) RemoveFromQueue(ctx context.Context, playerID string) error {
	ctx, span := tracer.Start(ctx, "MatchmakingRepository.RemoveFromQueue")
	defer span.End()

	// LRem removes count occurrences of value from the list.
	// If count is 0, all occurrences are removed.
	return r.rdb.LRem(ctx, matchmakingQueueKey, 0, playerID).Err()
}

// StreamInit initializes the matchmaking stream.
func (r *redisMatchmakingRepository) StreamInit(ctx context.Context) error {
	err := r.rdb.XGroupCreateMkStream(ctx, matchmakingStreamKey, matchmakingGroupName, "0").Err()
	if err != nil {
		if err.Error() == "BUSYGROUP Consumer Group name already exists" {
			return nil
		}
	}

	return nil
}

// AddToStream adds a player to the matchmaking stream.
func (r *redisMatchmakingRepository) AddToStream(ctx context.Context, playerID string) error {
	ctx, span := tracer.Start(ctx, "MatchmakingRepository.AddToQueue")
	defer span.End()

	_, err := r.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: matchmakingStreamKey,
		ID:     "*",
		Values: map[string]any{
			"player_id": playerID,
		},
	}).Result()
	if err != nil {
		return err
	}

	return nil
}
