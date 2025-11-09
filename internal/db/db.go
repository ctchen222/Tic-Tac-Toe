package db

import (
	"fmt"
	"log"
	"sync"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var (
	Once sync.Once

	DBConn *sqlx.DB
)

// LocalConnect initializes the database connection pool for a local SQLite database
// and returns a pointer to the sql.DB instance.
func LocalConnect(dbPath string) (*sqlx.DB, error) {
	pool, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		// Return the error instead of fatally exiting
		return nil, fmt.Errorf("failed to open local database connection: %w", err)
	}
	fmt.Printf("Connected to local database at %s!", dbPath)
	return pool, nil
}

func DBConnect() (*sqlx.DB, error) {
	Once.Do(func() {
		pool, err := sqlx.Open("sqlite3", "./master.db")
		if err != nil {
			log.Fatalf("Failed to open database connection: %v", err)
		}
		fmt.Println("Connected to master database!")
		DBConn = pool
	})
	return DBConn, nil
}

func InitializeDB() error {
	DB, err := DBConnect()
	if err != nil {
		return fmt.Errorf("failed to connect to master database: %w", err)
	}

	if _, err := DB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create users table if it doesn't exist
	userSchema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL
	);`

	if _, err := DB.Exec(userSchema); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	log.Println("DB connection initialized and schema verified.")

	return nil
}
