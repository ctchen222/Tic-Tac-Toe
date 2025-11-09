package models

// User represents a user in the database.
type User struct {
	ID           int64  `db:"id"`
	Username     string `db:"username"`
	PasswordHash string `db:"password_hash"`
}

// RegisterRequest defines the structure for a user registration request.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20"`
	Password string `json:"password" binding:"required,min=6,max=50"`
}

// LoginRequest defines the structure for a user login request.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse defines the structure for a successful login response.
type LoginResponse struct {
	Token string `json:"token"`
}
