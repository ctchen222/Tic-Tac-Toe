package logger

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

const instrumentationName = "ctchen222/Tic-Tac-Toe"

// MultiHandler is a slog.Handler that dispatches records to multiple handlers.
type MultiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler creates a new MultiHandler.
func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

// Enabled reports whether the handler handles records at the given level.
// The handler is enabled if any of its underlying handlers is enabled.
func (h *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle handles the Record.
// It dispatches the record to all of its underlying handlers.
func (h *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

// WithAttrs returns a new MultiHandler whose handlers have the given attributes.
func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}
	return NewMultiHandler(newHandlers...)
}

// WithGroup returns a new MultiHandler whose handlers have the given group.
func (h *MultiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}
	return NewMultiHandler(newHandlers...)
}

// Init initializes the global slog logger to be backed by both the console and OpenTelemetry.
func Init() {
	otelHandler := otelslog.NewHandler(instrumentationName)

	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true, // Include source file and line number
		Level:     slog.LevelDebug,
	})

	multiHandler := NewMultiHandler(consoleHandler, otelHandler)

	slogLogger := slog.New(multiHandler)

	slog.SetDefault(slogLogger)
}
