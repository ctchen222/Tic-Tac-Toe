package service

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/api/models"
	"ctchen222/Tic-Tac-Toe/internal/api/repository"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UserService defines the interface for user-related business logic.
type UserService interface {
	Register(ctx context.Context, req *models.RegisterRequest) (*models.User, error)
	Login(ctx context.Context, req *models.LoginRequest) (*models.User, error)
	GuestLogin(ctx context.Context) (*models.User, error)
}

type userService struct {
	userRepo repository.UserRepository
}

// NewUserService creates a new UserService.
func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

// Register handles user registration and returns the created user.
func (s *userService) Register(ctx context.Context, req *models.RegisterRequest) (*models.User, error) {
	existingUser, err := s.userRepo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		// Assuming a database error, not "not found"
		return nil, err
	}
	if existingUser != nil {
		return nil, errors.New("username already taken")
	}

	user := &models.User{
		Username: req.Username,
	}

	err = s.userRepo.CreateUser(ctx, user, req.Password)
	if err != nil {
		return nil, err
	}

	// Fetch the user again to get the ID assigned by the database
	createdUser, err := s.userRepo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if createdUser == nil {
		return nil, errors.New("failed to retrieve user after creation")
	}

	return createdUser, nil
}

// Login handles user login and returns the user on success.
func (s *userService) Login(ctx context.Context, req *models.LoginRequest) (*models.User, error) {
	user, err := s.userRepo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("invalid username or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	return user, nil
}

// GuestLogin creates a new guest user in the database and returns it.
func (s *userService) GuestLogin(ctx context.Context) (*models.User, error) {
	// Generate a unique username for the guest
	guestUsername := "guest_" + uuid.New().String()[:8]

	// Use a dummy password for guest accounts
	guestPassword := uuid.New().String()

	user := &models.User{
		Username: guestUsername,
	}

	err := s.userRepo.CreateUser(ctx, user, guestPassword)
	if err != nil {
		return nil, err
	}

	// Fetch the created guest user to get its ID
	createdGuest, err := s.userRepo.GetUserByUsername(ctx, guestUsername)
	if err != nil {
		return nil, err
	}
	if createdGuest == nil {
		return nil, errors.New("failed to retrieve guest user after creation")
	}

	return createdGuest, nil
}
