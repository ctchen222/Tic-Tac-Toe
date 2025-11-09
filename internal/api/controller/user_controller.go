package controller

import (
	"ctchen222/Tic-Tac-Toe/internal/api/models"
	"ctchen222/Tic-Tac-Toe/internal/api/response"
	"ctchen222/Tic-Tac-Toe/internal/api/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// UserController handles user-related HTTP requests.
type UserController struct {
	userService service.UserService
}

// NewUserController creates a new UserController.
func NewUserController(userService service.UserService) *UserController {
	return &UserController{
		userService: userService,
	}
}

// Register handles the user registration endpoint.
func (uc *UserController) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	err := uc.userService.Register(c.Request.Context(), &req)
	if err != nil {
		response.ErrorResponse(c, http.StatusConflict, err.Error())
		return
	}

	response.SuccessResponse(c, gin.H{"message": "User created successfully"})
}

// Login handles the user login endpoint.
func (uc *UserController) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	token, err := uc.userService.Login(c.Request.Context(), &req)
	if err != nil {
		// Assuming service returns specific errors that can be mapped to HTTP status codes
		response.ErrorResponse(c, http.StatusUnauthorized, err.Error())
		return
	}

	response.SuccessResponse(c, models.LoginResponse{Token: token})
}

// GuestLogin handles guest login, returning a generated player ID.
func (uc *UserController) GuestLogin(c *gin.Context) {
	playerID, err := uc.userService.GuestLogin(c.Request.Context())
	if err != nil {
		response.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(c, gin.H{"player_id": playerID})
	return
}
