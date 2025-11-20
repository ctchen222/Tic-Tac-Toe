package repository

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/api/models"
	"log"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

// setupTestDB creates and initializes an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *sqlx.DB {
	db, err := sqlx.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Create users table schema
	userSchema := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL
		);
	`

	if _, err := db.Exec(userSchema); err != nil {
		db.Close()
		t.Fatalf("Failed to create users table: %v", err)
	}

	log.Println("In-memory DB initialized for test.")
	return db
}

func TestUserRepository(t *testing.T) {
	db := setupTestDB(t)
	t.Cleanup(func() {
		db.Close()
	})

	repo := NewUserRepository(db)
	ctx := context.Background()

	t.Run("Create and Get User Successfully", func(t *testing.T) {
		username := "testuser"
		password := "password123"
		userToCreate := &models.User{
			Username: username,
		}

		// 1. Create User
		err := repo.CreateUser(ctx, userToCreate, password)
		assert.NoError(t, err)

		// 2. Get User
		fetchedUser, err := repo.GetUserByUsername(ctx, username)
		assert.NoError(t, err)
		assert.NotNil(t, fetchedUser)

		// 3. Assert equality and password hash
		assert.Equal(t, username, fetchedUser.Username)
		assert.NotZero(t, fetchedUser.ID)
		assert.NotEmpty(t, fetchedUser.PasswordHash)
		err = bcrypt.CompareHashAndPassword([]byte(fetchedUser.PasswordHash), []byte(password))
		assert.NoError(t, err, "Password hash should match the provided password")
	})

	t.Run("Create Duplicate User Fails", func(t *testing.T) {
		// This user should already exist from the previous test case
		username := "testuser"
		password := "newpassword"
		duplicateUser := &models.User{
			Username: username,
		}

		err := repo.CreateUser(ctx, duplicateUser, password)
		assert.Error(t, err, "Should return an error when creating a duplicate user")
	})

	t.Run("Get Non-Existent User", func(t *testing.T) {
		user, err := repo.GetUserByUsername(ctx, "nonexistentuser")
		assert.NoError(t, err, "Getting a non-existent user should not produce a database error")
		assert.Nil(t, user, "User should be nil for a non-existent user")
	})
}
