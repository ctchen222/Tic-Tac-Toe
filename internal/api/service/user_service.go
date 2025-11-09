package service

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/api/models"
	"ctchen222/Tic-Tac-Toe/internal/api/repository"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Note: In a real application, this should be loaded from a secure configuration.
var jwtSecret = []byte("my_super_secret_key")

// UserService defines the interface for user-related business logic.
type UserService interface {
	Register(ctx context.Context, req *models.RegisterRequest) error
	Login(ctx context.Context, req *models.LoginRequest) (string, error)
	GuestLogin(ctx context.Context) (string, error)
}

type userService struct {
	userRepo repository.UserRepository
}

// NewUserService creates a new UserService.
func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

// Register handles user registration.
func (s *userService) Register(ctx context.Context, req *models.RegisterRequest) error {
	// Check if user already exists
	existingUser, err := s.userRepo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return err
	}
	if existingUser != nil {
		return errors.New("username already taken")
	}

	user := &models.User{
		Username: req.Username,
	}

	return s.userRepo.CreateUser(ctx, user, req.Password)
}

// Login handles user login and returns a JWT on success.
func (s *userService) Login(ctx context.Context, req *models.LoginRequest) (string, error) {
	user, err := s.userRepo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", errors.New("invalid username or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return "", errors.New("invalid username or password")
	}

	// Create JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,
		"un":  user.Username,
		"exp": time.Now().Add(time.Hour * 72).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GuestLogin generates a UUID for a guest player.
func (s *userService) GuestLogin(ctx context.Context) (string, error) {
	playerID := uuid.New().String()
	return playerID, nil
}
