package repository

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/api/models"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// UserRepository defines the interface for user data operations.
type UserRepository interface {
	CreateUser(ctx context.Context, user *models.User, password string) error
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
}

type sqliteUserRepository struct {
	db *sqlx.DB
}

// NewUserRepository creates a new SQLite-based UserRepository.
func NewUserRepository(db *sqlx.DB) UserRepository {
	return &sqliteUserRepository{db: db}
}

// CreateUser hashes the password and inserts a new user into the database.
func (r *sqliteUserRepository) CreateUser(ctx context.Context, user *models.User, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	user.PasswordHash = string(hashedPassword)

	query := `INSERT INTO users (username, password_hash) VALUES (?, ?)`
	_, err = r.db.ExecContext(ctx, query, user.Username, user.PasswordHash)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByUsername retrieves a user from the database by their username.
func (r *sqliteUserRepository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	query := `SELECT id, username, password_hash FROM users WHERE username = ?`
	err := r.db.GetContext(ctx, &user, query, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No user found is not an application error
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}
	return &user, nil
}
