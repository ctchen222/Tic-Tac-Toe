package server

import (
	"ctchen222/Tic-Tac-Toe/internal/api/auth"
	"ctchen222/Tic-Tac-Toe/internal/api/controller"
	"ctchen222/Tic-Tac-Toe/internal/hub"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/repository"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("server")

type Server struct {
	hub            *hub.Hub
	engine         *gin.Engine
	upgrader       websocket.Upgrader
	userController *controller.UserController
	playerRepo     repository.PlayerRepository
	gameRepo       repository.GameRepository
}

// NewServer creates a new Server instance.
func NewServer(h *hub.Hub, uc *controller.UserController, pr repository.PlayerRepository, gr repository.GameRepository) *Server { // Add pr parameter
	engine := gin.Default()
	s := &Server{
		hub:            h,
		engine:         engine,
		userController: uc,
		playerRepo:     pr,
		gameRepo:       gr,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
	s.registerHandlers()
	return s
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// registerHandlers sets up the routes.
func (s *Server) registerHandlers() {
	// Serve static files
	s.engine.Static("/css", "./web/css")
	s.engine.Static("/js", "./web/js")
	s.engine.Static("/pages", "./web/pages")

	// Serve the main page
	s.engine.GET("/", func(c *gin.Context) {
		c.File("./web/index.html")
	})

	// API routes
	api := s.engine.Group("/api")
	{
		api.GET("/ws", s.handleWebSocket)
		api.POST("/register", s.userController.Register)
		api.POST("/login", s.userController.Login)
		api.POST("/guest-login", s.userController.GuestLogin)
	}
}

// handleWebSocket upgrades the connection and passes a registration request to the hub.
func (s *Server) handleWebSocket(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "server.handleWebSocket", trace.WithAttributes(
		attribute.String("http.url", c.Request.URL.String()),
		attribute.String("http.method", c.Request.Method),
	))
	defer span.End()

	tokenString := c.Query("token")
	if tokenString == "" {
		slog.WarnContext(ctx, "WebSocket connection attempt without token")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authentication token"})
		span.SetStatus(codes.Error, "Missing authentication token")
		return
	}

	userID, err := auth.ValidateToken(tokenString)
	if err != nil {
		slog.WarnContext(ctx, "WebSocket connection attempt with invalid token", "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		span.SetStatus(codes.Error, "Invalid or expired token")
		return
	}
	span.SetAttributes(attribute.String("player.id", userID))

	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to upgrade connection", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to upgrade connection")
		return
	}

	// --- New Reconnection Logic ---
	playerStatus, roomID, err := s.playerRepo.GetGameStatus(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get player game status for reconnection check", "player.id", userID, "error", err)
		// Fallback to normal registration if status check fails
		playerStatus = ""
		roomID = ""
	}

	roomExists, err := s.gameRepo.Exists(ctx, roomID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to check if game exists for reconnection", "room.id", roomID, "error", err)
		roomExists = false
	}

	if playerStatus == repository.PlayerOffline && roomID != "" && roomExists {
		slog.InfoContext(ctx, "Player attempting to reconnect to game", "player.id", userID, "room.id", roomID)
		p := player.NewPlayer(userID, conn)
		reconnectReq := &types.ReconnectRequest{Player: p, RoomID: roomID}
		s.hub.Reconnect() <- reconnectReq
		return // Reconnection handled, exit
	}

	// Existing logic for normal registration
	p := player.NewPlayer(userID, conn)
	req := &types.RegistrationRequest{
		Player:   p,
		PlayerID: p.ID,
		Ctx:      ctx,
	}
	s.hub.Register() <- req
}
