package repository

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/game"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	RoomKeyPrefix = "room:"
	voteKeyPrefix = "vote:"
	roomExpiry    = time.Minute * 10
)

// GameRepository defines the interface for game data operations.
type GameRepository interface {
	Create(ctx context.Context, roomID, playerXID, playerOID string) error
	FindByID(ctx context.Context, id string) (*game.GameStateDTO, error)
	Exists(ctx context.Context, id string) (bool, error)
	Update(ctx context.Context, id string, mark game.PlayerMark, row, col int) (*game.GameStateDTO, error)
	Delete(ctx context.Context, id string) error
	RecordVote(ctx context.Context, roomID, playerID string) error
	GetVotes(ctx context.Context, roomID string) (map[string]string, error)
	ClearVotes(ctx context.Context, roomID, playerXID, playerOID string) error
}

type redisGameRepository struct {
	rdb *redis.Client
}

// NewGameRepository creates a new Redis-based GameRepository.
func NewGameRepository(rdb *redis.Client) GameRepository {
	return &redisGameRepository{rdb: rdb}
}

// Create initializes a new game state in Redis.
func (r *redisGameRepository) Create(ctx context.Context, roomID, playerXID, playerOID string) error {
	ctx, span := tracer.Start(ctx, "GameRepository.Create")
	defer span.End()

	board := [3][3]game.PlayerMark{}
	boardJSON, err := json.Marshal(board)
	if err != nil {
		return fmt.Errorf("failed to marshal initial board: %w", err)
	}

	playerXKey := PlayerKeyPrefix + playerXID
	playerOKey := PlayerKeyPrefix + playerOID
	playerXIsBot := strings.HasPrefix(playerXID, "bot-")
	playerOIsBot := strings.HasPrefix(playerOID, "bot-")

	pipe := r.rdb.Pipeline()
	roomKey := RoomKeyPrefix + roomID
	pipe.HSet(ctx, roomKey, game.FieldBoard, boardJSON)
	pipe.HSet(ctx, roomKey, game.FieldPlayerX, playerXID)
	pipe.HSet(ctx, roomKey, game.FieldPlayerO, playerOID)
	pipe.HSet(ctx, roomKey, game.FieldNextTurn, string(game.RandomlyChooseFirstPlayer()))
	pipe.HSet(ctx, roomKey, game.FieldWinner, "")
	pipe.HSet(ctx, roomKey, game.FieldStatus, "in_progress")
	if !playerXIsBot {
		pipe.HSet(ctx, playerXKey, "status", PlayerInGame)
	}
	if !playerOIsBot {
		pipe.HSet(ctx, playerOKey, "status", PlayerInGame)
	}
	pipe.Expire(ctx, roomKey, roomExpiry)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create game in redis: %w", err)
	}
	return nil
}

// FindByID retrieves the current game state from Redis.
func (r *redisGameRepository) FindByID(ctx context.Context, id string) (*game.GameStateDTO, error) {
	ctx, span := tracer.Start(ctx, "GameRepository.FindByID")
	defer span.End()

	roomKey := RoomKeyPrefix + id
	data, err := r.rdb.HGetAll(ctx, roomKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get game state from redis: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("game not found")
	}

	var board [3][3]game.PlayerMark
	if err := json.Unmarshal([]byte(data[game.FieldBoard]), &board); err != nil {
		return nil, fmt.Errorf("failed to unmarshal board: %w", err)
	}

	isDraw := game.IsBoardFull(board) && data[game.FieldWinner] == ""

	return &game.GameStateDTO{
		Board:       board,
		CurrentTurn: game.PlayerMark(data[game.FieldNextTurn]),
		Winner:      game.PlayerMark(data[game.FieldWinner]),
		IsDraw:      isDraw,
		PlayerXID:   data[game.FieldPlayerX],
		PlayerOID:   data[game.FieldPlayerO],
	}, nil
}

// Exists checks if a game with the given ID exists in Redis.
func (r *redisGameRepository) Exists(ctx context.Context, id string) (bool, error) {
	ctx, span := tracer.Start(ctx, "GameRepository.Exists")
	defer span.End()

	roomKey := RoomKeyPrefix + id
	exists, err := r.rdb.Exists(ctx, roomKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check game existence in redis: %w", err)
	}
	return exists == 1, nil
}

// Update applies a player's move to the game state in Redis.
func (r *redisGameRepository) Update(ctx context.Context, id string, mark game.PlayerMark, row, col int) (*game.GameStateDTO, error) {
	ctx, span := tracer.Start(ctx, "GameRepository.Update")
	defer span.End()

	roomKey := RoomKeyPrefix + id

	txf := func(tx *redis.Tx) error {
		data, err := tx.HGetAll(ctx, roomKey).Result()
		if err != nil {
			return err
		}

		if data[game.FieldWinner] != "" || data[game.FieldStatus] == "finished" {
			return fmt.Errorf("game is already over")
		}
		if game.PlayerMark(data[game.FieldNextTurn]) != mark {
			return fmt.Errorf("not player's turn")
		}
		var board [3][3]game.PlayerMark
		if err := json.Unmarshal([]byte(data[game.FieldBoard]), &board); err != nil {
			return fmt.Errorf("failed to unmarshal board for update: %w", err)
		}
		if row < game.BorderMin || row > game.BorderMax || col < game.BorderMin || col > game.BorderMax || board[row][col] != game.None {
			return fmt.Errorf("invalid move")
		}

		board[row][col] = mark
		newBoardJSON, err := json.Marshal(board)
		if err != nil {
			return fmt.Errorf("failed to marshal updated board: %w", err)
		}

		winner := game.CheckWinner(board)
		isFull := game.IsBoardFull(board)
		nextTurn := game.PlayerO
		if mark == game.PlayerO {
			nextTurn = game.PlayerX
		}
		status := "in_progress"
		if winner != game.None || isFull {
			status = "finished"
		}

		pipe := tx.TxPipeline()
		pipe.HSet(ctx, roomKey, game.FieldBoard, newBoardJSON)
		pipe.HSet(ctx, roomKey, game.FieldNextTurn, string(nextTurn))
		pipe.HSet(ctx, roomKey, game.FieldWinner, string(winner))
		pipe.HSet(ctx, roomKey, game.FieldStatus, status)
		_, err = pipe.Exec(ctx)
		return err
	}

	if err := r.rdb.Watch(ctx, txf, roomKey); err != nil {
		return nil, err
	}

	return r.FindByID(ctx, id)
}

// Delete removes the game state from Redis.
func (r *redisGameRepository) Delete(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "GameRepository.Delete")
	defer span.End()

	roomKey := RoomKeyPrefix + id
	return r.rdb.Del(ctx, roomKey).Err()
}

// RecordVote records a player's vote for a rematch.
func (r *redisGameRepository) RecordVote(ctx context.Context, roomID, playerID string) error {
	roomKey := RoomKeyPrefix + roomID
	voteKey := voteKeyPrefix + playerID
	return r.rdb.HSet(ctx, roomKey, voteKey, "true").Err()
}

// GetVotes retrieves all votes for a room.
func (r *redisGameRepository) GetVotes(ctx context.Context, roomID string) (map[string]string, error) {
	ctx, span := tracer.Start(ctx, "GameRepository.GetVotes")
	defer span.End()

	roomKey := RoomKeyPrefix + roomID
	return r.rdb.HGetAll(ctx, roomKey).Result()
}

// ClearVotes removes rematch votes from Redis.
func (r *redisGameRepository) ClearVotes(ctx context.Context, roomID, playerXID, playerOID string) error {
	ctx, span := tracer.Start(ctx, "GameRepository.ClearVotes")
	defer span.End()

	roomKey := RoomKeyPrefix + roomID
	voteKey1 := voteKeyPrefix + playerXID
	voteKey2 := voteKeyPrefix + playerOID
	return r.rdb.HDel(ctx, roomKey, voteKey1, voteKey2).Err()
}
