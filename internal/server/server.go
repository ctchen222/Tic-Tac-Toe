package server

import (
	"ctchen222/Tic-Tac-Toe/internal/api/controller"
	"ctchen222/Tic-Tac-Toe/internal/hub"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
}

// NewServer creates a new Server instance.
func NewServer(h *hub.Hub, uc *controller.UserController) *Server {
	engine := gin.Default()
	s := &Server{
		hub:            h,
		engine:         engine,
		userController: uc,
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

	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to upgrade connection", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to upgrade connection")
		return
	}

	playerID := c.Query("playerId")
	if playerID == "" {
		playerID = uuid.New().String()
	}
	span.SetAttributes(attribute.String("player.id", playerID))

	p := player.NewPlayer(playerID, conn)

	mode := c.DefaultQuery("mode", "human")
	difficulty := c.DefaultQuery("difficulty", "easy")
	span.SetAttributes(attribute.String("game.mode", mode), attribute.String("game.difficulty", difficulty))

	req := &types.RegistrationRequest{
		Player:     p,
		PlayerID:   p.ID,
		Mode:       mode,
		Difficulty: difficulty,
		Ctx:        ctx,
	}
	s.hub.Register() <- req
}
