package controller

import (
	"ctchen222/Tic-Tac-Toe/internal/api/auth"
	"ctchen222/Tic-Tac-Toe/internal/api/models"
	"ctchen222/Tic-Tac-Toe/internal/api/response"
	"ctchen222/Tic-Tac-Toe/internal/api/service"
	"net/http"
	"strconv"

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

// Register handles user registration, and returns a JWT upon success.
func (uc *UserController) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := uc.userService.Register(c.Request.Context(), &req)
	if err != nil {
		response.ErrorResponse(c, http.StatusConflict, err.Error())
		return
	}

	token, err := auth.GenerateToken(strconv.FormatInt(user.ID, 10))
	if err != nil {
		response.ErrorResponse(c, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	response.SuccessResponse(c, models.LoginResponse{Token: token})
}

// Login handles user login and returns a JWT upon success.
func (uc *UserController) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := uc.userService.Login(c.Request.Context(), &req)
	if err != nil {
		response.ErrorResponse(c, http.StatusUnauthorized, err.Error())
		return
	}

	token, err := auth.GenerateToken(strconv.FormatInt(user.ID, 10))
	if err != nil {
		response.ErrorResponse(c, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	response.SuccessResponse(c, models.LoginResponse{Token: token})
}

// GuestLogin creates a persistent guest user and returns a JWT.
func (uc *UserController) GuestLogin(c *gin.Context) {
	user, err := uc.userService.GuestLogin(c.Request.Context())
	if err != nil {
		response.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	token, err := auth.GenerateToken(strconv.FormatInt(user.ID, 10))
	if err != nil {
		response.ErrorResponse(c, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	response.SuccessResponse(c, models.LoginResponse{Token: token})
}
